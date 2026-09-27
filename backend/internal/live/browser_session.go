package live

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/inox/inox/backend/internal/domain"
)

// Pacing for coaxing a player into starting. Autoplaying players need none of it,
// so the first nudge waits until the page has had a moment to start on its own.
const (
	nudgeAfterLoad    = 2 * time.Second
	nudgeWithoutLoad  = 8 * time.Second
	nudgeInterval     = 4 * time.Second
	maxNudges         = 3
	manifestSettle    = 1500 * time.Millisecond
	maxTrackedPending = 4000
	// licenceWindow is how soon after a page generates a licence request its
	// next POST is taken to be that request going out.
	licenceWindow = 15 * time.Second
)

// watchOptions describes one page to load and observe.
type watchOptions struct {
	pageURL  string
	referrer string
	// pageHeaders are added to the page's own requests, like the static
	// resolver's page_headers.
	pageHeaders map[string]string
	// budget is how long to wait for the player to request a manifest.
	budget time.Duration
	// match, when set, is the only kind of manifest URL that ends the wait.
	match *regexp.Regexp
}

// observation is what a page revealed about its streams.
type observation struct {
	manifests  []*manifestRequest // in the order they were first requested
	pageStatus int
	// pageURL is the URL the page's document actually came back from, after any
	// redirects; it can differ from the source URL.
	pageURL string
	// pageRefused is why the egress guard refused the page itself, if it did.
	pageRefused string
	loaded      bool // the page's load event fired
	frames      int  // out-of-process frames attached, at any depth
	nudges      int
	popups      int
	drm         drmEvidence
	elapsed     time.Duration
	// cookies is every cookie the page's browser context held when watching
	// stopped: the session the player's requests were made in.
	cookies []browserCookie
	// redirects are the hosts the top-level page bounced through before landing,
	// and frameHosts the hosts of the frames it loaded. Hostnames only.
	redirects  []string
	frameHosts []string
}

// manifestRequest is one URL the page requested that looks like a stream manifest,
// either by its name or by the content type it came back with.
type manifestRequest struct {
	url      string
	headers  map[string]string // as sent on the wire, canonicalised
	mimeType string
	status   int    // 0 until a response arrives
	failure  string // network error, if it never got one
	hits     int
	seq      int
}

func (m *manifestRequest) loaded() bool { return m.status >= 200 && m.status < 300 }

// trackedRequest holds a request's headers until its response says whether it was
// a manifest. Most requests on a page are not, and are dropped at that point.
type trackedRequest struct {
	url          string
	resourceType string
	headers      map[string]string
}

// pageWatch is one resolve's view of its tab: every session attached under it and
// the manifest requests any of them made.
type pageWatch struct {
	ctx       context.Context
	conn      *cdpConn
	opts      watchOptions
	contextID string

	mu        sync.Mutex
	targetID  string // also the ID of the tab's main frame
	sessionID string
	sessions  map[string]string // attached session -> target type
	pending   map[string]*trackedRequest
	manifests map[string]*manifestRequest
	seq       int
	// pageStatus, pageURL and pageRefused describe the page's own document
	// response: its final status and landed URL, after any redirects.
	pageStatus  int
	pageURL     string
	pageRefused string
	strayPages  []string
	frames      int
	redirects   []string // hosts the top document bounced through, in order
	frameHosts  []string // hosts of the frames the page loaded
	popups      int
	nudges      int
	drm         drmEvidence
	// keyRequests holds, per session, when the page last generated a licence
	// request, so the POST that carries it can be recognised by timing.
	keyRequests map[string]time.Time

	found      chan struct{}
	foundOnce  sync.Once
	drmFound   chan struct{}
	drmOnce    sync.Once
	loaded     chan struct{}
	loadedOnce sync.Once
}

// watch opens the page in a fresh browser context and records every manifest
// request made by it, its frames at any depth, and their workers.
func (c *chromium) watch(ctx context.Context, opts watchOptions) (*observation, error) {
	w := &pageWatch{
		ctx:         ctx,
		conn:        c.conn,
		opts:        opts,
		sessions:    make(map[string]string),
		pending:     make(map[string]*trackedRequest),
		manifests:   make(map[string]*manifestRequest),
		keyRequests: make(map[string]time.Time),
		found:       make(chan struct{}),
		drmFound:    make(chan struct{}),
		loaded:      make(chan struct{}),
	}

	var created struct {
		BrowserContextID string `json:"browserContextId"`
	}
	if err := c.conn.call(ctx, "", "Target.createBrowserContext", map[string]any{"disposeOnDetach": true}, &created); err != nil {
		return nil, fmt.Errorf("headless browser could not open a context: %w", err)
	}
	w.contextID = created.BrowserContextID
	c.routeContext(w.contextID, w.onTargetCreated)
	defer func() {
		c.unrouteContext(w.contextID)
		w.forgetSessions()
		// Disposing the context closes the tab, its frames and any popup, and
		// drops every cookie the page set; the resolve keeps its own copy.
		dctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = c.conn.call(dctx, "", "Target.disposeBrowserContext", map[string]any{"browserContextId": w.contextID}, nil)
		cancel()
	}()

	_ = c.conn.call(ctx, "", "Browser.setDownloadBehavior", map[string]any{
		"behavior": "deny", "browserContextId": w.contextID,
	}, nil)

	var target struct {
		TargetID string `json:"targetId"`
	}
	if err := c.conn.call(ctx, "", "Target.createTarget", map[string]any{
		"url": "about:blank", "browserContextId": w.contextID,
	}, &target); err != nil {
		return nil, fmt.Errorf("headless browser could not open a tab: %w", err)
	}
	w.adoptTab(target.TargetID)

	var attached struct {
		SessionID string `json:"sessionId"`
	}
	if err := c.conn.call(ctx, "", "Target.attachToTarget", map[string]any{
		"targetId": target.TargetID, "flatten": true,
	}, &attached); err != nil {
		return nil, fmt.Errorf("headless browser could not attach to its tab: %w", err)
	}
	w.mu.Lock()
	w.sessionID = attached.SessionID
	w.mu.Unlock()
	w.listen(attached.SessionID, "page")

	if err := w.prepare(ctx, attached.SessionID, "page"); err != nil {
		return nil, fmt.Errorf("headless browser could not instrument its tab: %w", err)
	}
	if len(opts.pageHeaders) > 0 {
		if err := c.conn.call(ctx, attached.SessionID, "Network.setExtraHTTPHeaders", map[string]any{
			"headers": opts.pageHeaders,
		}, nil); err != nil {
			return nil, fmt.Errorf("could not apply page_headers: %w", err)
		}
	}

	// The budget covers the navigation too: a page that never answers must not
	// hold the resolve for however long the caller is prepared to wait.
	start := time.Now()
	budgeted, cancel := context.WithDeadline(ctx, start.Add(opts.budget))
	defer cancel()
	navigate := map[string]any{"url": opts.pageURL}
	if opts.referrer != "" {
		navigate["referrer"] = opts.referrer
	}
	var nav struct {
		ErrorText string `json:"errorText"`
	}
	if err := c.conn.call(budgeted, attached.SessionID, "Page.navigate", navigate, &nav); err != nil {
		if budgeted.Err() != nil && ctx.Err() == nil {
			return nil, resolveFailure(domain.OutcomeTimeout, "page_unresponsive",
				fmt.Sprintf("the page did not respond within %s", opts.budget), err)
		}
		return nil, fmt.Errorf("could not navigate to the page: %w", err)
	}
	if nav.ErrorText != "" {
		return nil, resolveFailure(domain.OutcomeFailed, "page_unreachable",
			"the page did not load: "+describeNetError(nav.ErrorText), nil)
	}

	w.mu.Lock()
	refused := w.pageRefused != ""
	w.mu.Unlock()
	if refused { // a page the guard refused has nothing to watch
		return w.result(start), nil
	}
	w.observe(budgeted, start)
	obs := w.result(start)
	obs.cookies = c.sessionCookies(ctx, w.contextID)
	return obs, nil
}

// sessionCookies reads the cookies a browser context holds, from every site its
// page and frames loaded. It must run before the context is disposed, which drops
// them. A failure here is not a failed resolve: most streams need no cookies, and
// one that does is reported when the server's check fetch is refused.
func (c *chromium) sessionCookies(ctx context.Context, contextID string) []browserCookie {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var out struct {
		Cookies []browserCookie `json:"cookies"`
	}
	if err := c.conn.call(ctx, "", "Storage.getCookies", map[string]any{"browserContextId": contextID}, &out); err != nil {
		return nil
	}
	return out.Cookies
}

// prepare instruments a freshly attached target before it runs any script. Frames
// and pages auto-attach their own children paused, which is what makes a player
// three cross-origin iframes deep as visible as one in the page itself: its first
// request cannot happen before Network is enabled on its session.
//
// Frames also get mediaProbeScript, installed the same way, so the page's use of EME
// is seen from its first call. The binding it reports through needs the Runtime
// domain enabled; without it Chromium installs bindings into existing documents only.
func (w *pageWatch) prepare(ctx context.Context, sessionID, kind string) error {
	if err := w.conn.call(ctx, sessionID, "Network.enable", nil, nil); err != nil && kind == "page" {
		return err
	}
	if kind != "page" && kind != "iframe" {
		return nil
	}
	for _, step := range []struct {
		method string
		params any
	}{
		{"Page.enable", nil},
		{"Runtime.enable", nil},
		{"Runtime.addBinding", map[string]any{"name": probeBinding}},
		{"Page.addScriptToEvaluateOnNewDocument", map[string]any{"source": mediaProbeScript}},
	} {
		if err := w.conn.call(ctx, sessionID, step.method, step.params, nil); err != nil && kind == "page" {
			return err
		}
	}
	return w.conn.call(ctx, sessionID, "Target.setAutoAttach", map[string]any{
		"autoAttach": true, "waitForDebuggerOnStart": true, "flatten": true,
	}, nil)
}

func (w *pageWatch) listen(sessionID, kind string) {
	w.mu.Lock()
	w.sessions[sessionID] = kind
	w.mu.Unlock()
	w.conn.listen(sessionID, func(method string, params json.RawMessage) {
		w.onEvent(sessionID, method, params)
	})
}

func (w *pageWatch) forgetSessions() {
	w.mu.Lock()
	ids := make([]string, 0, len(w.sessions))
	for id := range w.sessions {
		ids = append(ids, id)
	}
	w.mu.Unlock()
	for _, id := range ids {
		w.conn.unlisten(id)
	}
}

// onEvent runs on the connection's read loop, so it only records and hands
// anything that needs a round trip to a goroutine.
func (w *pageWatch) onEvent(sessionID, method string, params json.RawMessage) {
	switch method {
	case "Network.requestWillBeSent":
		w.onRequest(sessionID, params)
	case "Network.requestWillBeSentExtraInfo":
		w.onRequestExtraInfo(sessionID, params)
	case "Network.responseReceived":
		w.onResponse(sessionID, params)
	case "Network.loadingFinished":
		w.onDone(sessionID, params, "")
	case "Network.loadingFailed":
		var ev struct {
			ErrorText string `json:"errorText"`
		}
		_ = json.Unmarshal(params, &ev)
		w.onDone(sessionID, params, ev.ErrorText)
	case "Runtime.bindingCalled":
		w.onProbe(sessionID, params)
	case "Target.attachedToTarget":
		w.onAttached(params)
	case "Target.detachedFromTarget":
		var ev struct {
			SessionID string `json:"sessionId"`
		}
		if json.Unmarshal(params, &ev) == nil {
			w.mu.Lock()
			delete(w.sessions, ev.SessionID)
			w.mu.Unlock()
			w.conn.unlisten(ev.SessionID)
		}
	case "Page.javascriptDialogOpening":
		// An unanswered alert() freezes its frame, player included.
		go func() {
			_ = w.conn.call(w.ctx, sessionID, "Page.handleJavaScriptDialog", map[string]any{"accept": true}, nil)
		}()
	case "Page.loadEventFired":
		w.mu.Lock()
		isTab := sessionID == w.sessionID
		w.mu.Unlock()
		if isTab {
			w.loadedOnce.Do(func() { close(w.loaded) })
		}
	}
}

func (w *pageWatch) onAttached(params json.RawMessage) {
	var ev struct {
		SessionID          string     `json:"sessionId"`
		TargetInfo         targetInfo `json:"targetInfo"`
		WaitingForDebugger bool       `json:"waitingForDebugger"`
	}
	if json.Unmarshal(params, &ev) != nil || ev.SessionID == "" {
		return
	}
	// Listening starts before the target is resumed, so none of its events are lost.
	w.listen(ev.SessionID, ev.TargetInfo.Type)
	if ev.TargetInfo.Type == "iframe" {
		w.mu.Lock()
		w.frames++
		w.mu.Unlock()
	}
	go func() {
		ctx, cancel := context.WithTimeout(w.ctx, 5*time.Second)
		_ = w.prepare(ctx, ev.SessionID, ev.TargetInfo.Type)
		cancel()
		if ev.WaitingForDebugger {
			// Always resume, even if instrumenting failed or ran out of time: a
			// frame left paused is a player that never starts.
			ctx, cancel := context.WithTimeout(w.ctx, 5*time.Second)
			_ = w.conn.call(ctx, ev.SessionID, "Runtime.runIfWaitingForDebugger", nil, nil)
			cancel()
		}
	}()
}

// onProbe takes one report from mediaProbeScript.
func (w *pageWatch) onProbe(sessionID string, params json.RawMessage) {
	var call struct {
		Name    string `json:"name"`
		Payload string `json:"payload"`
	}
	if json.Unmarshal(params, &call) != nil || call.Name != probeBinding || len(call.Payload) > maxProbePayload {
		return
	}
	var ev probeEvent
	if json.Unmarshal([]byte(call.Payload), &ev) != nil {
		return
	}

	w.mu.Lock()
	if w.drm.apply(ev) {
		w.keyRequests[sessionID] = time.Now()
	}
	confirmed := w.drm.confirmed()
	w.mu.Unlock()
	if confirmed {
		w.drmOnce.Do(func() { close(w.drmFound) })
	}
}

// ── network events ──────────────────────────────────────────────────────────

func requestKey(sessionID, requestID string) string { return sessionID + "|" + requestID }

func (w *pageWatch) onRequest(sessionID string, params json.RawMessage) {
	var ev struct {
		RequestID string `json:"requestId"`
		Type      string `json:"type"`
		FrameID   string `json:"frameId"`
		Request   struct {
			URL     string            `json:"url"`
			Method  string            `json:"method"`
			Headers map[string]string `json:"headers"`
		} `json:"request"`
		RedirectResponse *struct {
			URL string `json:"url"`
		} `json:"redirectResponse"`
	}
	if json.Unmarshal(params, &ev) != nil || !isNetworkURL(ev.Request.URL) {
		return
	}
	// A redirect of the top-level document: record the host it came from, so the
	// operator can see where a page bounced through before it landed.
	if ev.RedirectResponse != nil && ev.Type == "Document" {
		w.noteRedirect(ev.FrameID, ev.RedirectResponse.URL)
	}
	if ev.Request.Method == http.MethodPost {
		w.checkLicence(sessionID, ev.Type, ev.Request.URL)
	}
	if !mayCarryManifest(ev.Type) {
		return
	}
	key := requestKey(sessionID, ev.RequestID)

	w.mu.Lock()
	defer w.mu.Unlock()
	tr, ok := w.pending[key]
	if !ok {
		if len(w.pending) >= maxTrackedPending && !mentionsManifest(ev.Request.URL) {
			return
		}
		tr = &trackedRequest{headers: map[string]string{}}
		w.pending[key] = tr
	}
	// A redirect reuses the request ID; the newest hop is the one that counts.
	tr.url = ev.Request.URL
	tr.resourceType = ev.Type
	mergeWireHeaders(tr.headers, ev.Request.Headers)

	if mentionsManifest(ev.Request.URL) {
		w.noteManifestLocked(ev.Request.URL, tr.headers)
	}
}

// checkLicence recognises a licence request: the first XHR or fetch POST a frame
// makes after generating one, or a POST to a URL shaped like a licence server from a
// page that has been using EME. A licence-like URL alone is not enough -- plenty of
// sites POST to /license for reasons that have nothing to do with DRM. Only the host
// is kept, and the request body -- the licence challenge -- is never read.
func (w *pageWatch) checkLicence(sessionID, resourceType, rawURL string) {
	w.mu.Lock()
	generated, ok := w.keyRequests[sessionID]
	justGenerated := ok && time.Since(generated) < licenceWindow &&
		(resourceType == "XHR" || resourceType == "Fetch")
	// Not len(signals) > 0: a page that only probed EME support has a signal too,
	// and its analytics POST must not be mistaken for a licence request.
	usedEME := w.drm.usingEME()
	isLicence := justGenerated || (usedEME && licenseURL(rawURL))
	if isLicence {
		w.drm.noteLicence(rawURL)
		delete(w.keyRequests, sessionID)
	}
	w.mu.Unlock()
	if isLicence {
		w.drmOnce.Do(func() { close(w.drmFound) })
	}
}

// onRequestExtraInfo merges the headers as they actually went out -- Origin and
// the final Referer are only known to the network stack. It can arrive before or
// after the request event it belongs to.
func (w *pageWatch) onRequestExtraInfo(sessionID string, params json.RawMessage) {
	var ev struct {
		RequestID string            `json:"requestId"`
		Headers   map[string]string `json:"headers"`
	}
	if json.Unmarshal(params, &ev) != nil {
		return
	}
	key := requestKey(sessionID, ev.RequestID)

	w.mu.Lock()
	defer w.mu.Unlock()
	tr, ok := w.pending[key]
	if !ok {
		if len(w.pending) >= maxTrackedPending {
			return
		}
		tr = &trackedRequest{headers: map[string]string{}}
		w.pending[key] = tr
	}
	mergeWireHeaders(tr.headers, ev.Headers)
	if tr.url != "" && mentionsManifest(tr.url) {
		w.noteManifestLocked(tr.url, tr.headers)
	}
}

func (w *pageWatch) onResponse(sessionID string, params json.RawMessage) {
	var ev struct {
		RequestID string `json:"requestId"`
		Type      string `json:"type"`
		FrameID   string `json:"frameId"`
		Response  struct {
			URL      string            `json:"url"`
			Status   int               `json:"status"`
			MimeType string            `json:"mimeType"`
			Headers  map[string]string `json:"headers"`
		} `json:"response"`
	}
	if json.Unmarshal(params, &ev) != nil {
		return
	}

	w.mu.Lock()
	// A page target's ID is its main frame's ID, and it is known before the
	// navigation starts -- unlike Page.navigate's reply, which lands after this.
	if ev.Type == "Document" && ev.FrameID == w.targetID && w.pageStatus == 0 {
		w.pageStatus = ev.Response.Status
		w.pageURL = ev.Response.URL // the URL that actually answered, after redirects
		for name, value := range ev.Response.Headers {
			if strings.EqualFold(name, egressRefusedHeader) {
				w.pageRefused = value
			}
		}
	} else if ev.Type == "Document" && ev.FrameID != w.targetID {
		// A sub-frame's own document: record its host, from the URL it actually
		// loaded rather than the about:blank it was attached at.
		if host := hostOf(ev.Response.URL); host != "" && host != "<unknown host>" {
			w.frameHosts = appendUnique(w.frameHosts, host)
		}
	}
	tr := w.pending[requestKey(sessionID, ev.RequestID)]
	if tr == nil || !(mentionsManifest(ev.Response.URL) || manifestMIME(ev.Response.MimeType)) {
		w.mu.Unlock()
		return
	}
	m := w.noteManifestLocked(ev.Response.URL, tr.headers)
	m.status = ev.Response.Status
	m.mimeType = ev.Response.MimeType
	m.failure = ""
	m.hits++
	convincing := m.loaded() && servedAsManifest(m.url, m.mimeType) &&
		(w.opts.match == nil || w.opts.match.MatchString(m.url))
	w.mu.Unlock()

	if convincing {
		w.foundOnce.Do(func() { close(w.found) })
	}
}

func (w *pageWatch) onDone(sessionID string, params json.RawMessage, failure string) {
	var ev struct {
		RequestID string `json:"requestId"`
	}
	if json.Unmarshal(params, &ev) != nil {
		return
	}
	key := requestKey(sessionID, ev.RequestID)

	w.mu.Lock()
	defer w.mu.Unlock()
	tr := w.pending[key]
	delete(w.pending, key)
	if tr == nil || failure == "" {
		return
	}
	if m, ok := w.manifests[tr.url]; ok && m.status == 0 {
		m.failure = failure
	}
}

// noteRedirect records the host of a top-level document redirect hop. frameID is
// checked against the main frame under the lock, since onRequest reads targetID
// without it.
func (w *pageWatch) noteRedirect(frameID, fromURL string) {
	host := hostOf(fromURL)
	if host == "" || host == "<unknown host>" {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if frameID == w.targetID {
		w.redirects = appendUnique(w.redirects, host)
	}
}

// noteManifestLocked records a manifest URL the first time it is seen and refreshes
// the headers it was requested with.
func (w *pageWatch) noteManifestLocked(rawURL string, headers map[string]string) *manifestRequest {
	m, ok := w.manifests[rawURL]
	if !ok {
		w.seq++
		m = &manifestRequest{url: rawURL, seq: w.seq, headers: map[string]string{}}
		w.manifests[rawURL] = m
	}
	for k, v := range headers {
		m.headers[k] = v
	}
	return m
}

// ── targets ─────────────────────────────────────────────────────────────────

// adoptTab records which page in this context is ours; any other is a popup.
func (w *pageWatch) adoptTab(targetID string) {
	w.mu.Lock()
	w.targetID = targetID
	stray := w.strayPages
	w.strayPages = nil
	w.mu.Unlock()
	for _, id := range stray {
		if id != targetID {
			w.closePopup(id)
		}
	}
}

// onTargetCreated closes every page the tab opens. Popups are almost always ads,
// and each is another renderer on a one-core host.
func (w *pageWatch) onTargetCreated(info targetInfo) {
	if info.Type != "page" {
		return
	}
	w.mu.Lock()
	own := w.targetID
	if own == "" {
		// Our own tab can be announced before createTarget returns its ID.
		w.strayPages = append(w.strayPages, info.TargetID)
		w.mu.Unlock()
		return
	}
	w.mu.Unlock()
	if info.TargetID != own {
		w.closePopup(info.TargetID)
	}
}

func (w *pageWatch) closePopup(targetID string) {
	w.mu.Lock()
	w.popups++
	w.mu.Unlock()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = w.conn.call(ctx, "", "Target.closeTarget", map[string]any{"targetId": targetID}, nil)
	}()
}

// ── waiting for the player ──────────────────────────────────────────────────

// observe waits until the page's player has requested a manifest, nudging it to
// start if it does not do so by itself, or until the budget runs out.
func (w *pageWatch) observe(ctx context.Context, start time.Time) {
	deadline := time.NewTimer(w.opts.budget - time.Since(start))
	defer deadline.Stop()
	nudge := time.NewTimer(nudgeWithoutLoad)
	defer nudge.Stop()
	loaded := w.loaded

	for {
		select {
		case <-ctx.Done():
			return
		case <-deadline.C:
			return
		case <-w.found:
			// A player asks for the master playlist and then a variant within
			// moments; waiting for both lets the choice see the whole ladder.
			w.settle(ctx, deadline)
			return
		case <-w.drmFound:
			// The page is setting up DRM playback. Watching it negotiate a licence
			// changes nothing about the verdict, so stop -- after the same short
			// wait, in case the page is also fetching an unprotected stream.
			w.settle(ctx, deadline)
			return
		case <-loaded:
			loaded = nil
			resetTimer(nudge, nudgeAfterLoad)
		case <-nudge.C:
			w.mu.Lock()
			w.nudges++
			n := w.nudges
			w.mu.Unlock()
			w.nudge(ctx)
			if n < maxNudges {
				nudge.Reset(nudgeInterval)
			}
		}
	}
}

func (w *pageWatch) settle(ctx context.Context, deadline *time.Timer) {
	settle := time.NewTimer(manifestSettle)
	defer settle.Stop()
	select {
	case <-settle.C:
	case <-ctx.Done():
	case <-deadline.C:
	}
}

// playScript starts every media element that exists but is paused, in this frame
// and any same-origin frame below it. Cross-origin frames are reached through their
// own sessions.
const playScript = `(() => {
  const visit = (doc, depth) => {
    for (const el of doc.querySelectorAll('video, audio')) {
      try { el.muted = true; const p = el.play(); if (p) p.catch(() => {}); } catch (e) {}
    }
    if (depth >= 4) return;
    for (const f of doc.querySelectorAll('iframe')) {
      try { if (f.contentDocument) visit(f.contentDocument, depth + 1); } catch (e) {}
    }
  };
  visit(document, 0);
})()`

// locateScript finds where a viewer would click to start the stream: the centre of
// the largest visible video or embedded frame, or failing that of the largest
// element named like a player. Returns null rather than guess, because a click on
// an arbitrary spot of a streaming page is usually a click on an ad.
const locateScript = `(() => {
  const visible = (el, minW, minH) => {
    const r = el.getBoundingClientRect();
    if (r.width < minW || r.height < minH) return 0;
    const s = getComputedStyle(el);
    if (s.visibility === 'hidden' || s.display === 'none' || Number(s.opacity) === 0) return 0;
    return r.width * r.height;
  };
  const largest = (selector, minW, minH) => {
    let best = null, bestArea = 0;
    for (const el of document.querySelectorAll(selector)) {
      const area = visible(el, minW, minH);
      if (area > bestArea) { best = el; bestArea = area; }
    }
    return best;
  };
  const el = largest('video, iframe, embed, object', 120, 90) ||
    largest('[id*="player" i], [class*="player" i], [id*="video" i], [class*="video" i]', 240, 135);
  if (!el) return null;
  el.scrollIntoView({ block: 'center', inline: 'center' });
  const r = el.getBoundingClientRect();
  return {
    x: Math.min(Math.max(r.left + r.width / 2, 1), innerWidth - 1),
    y: Math.min(Math.max(r.top + r.height / 2, 1), innerHeight - 1),
  };
})()`

// nudge does what a viewer would to get a player going: start any paused media
// element, then click the middle of the player, where play buttons live.
func (w *pageWatch) nudge(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	w.mu.Lock()
	tab := w.sessionID
	frames := []string{tab}
	for id, kind := range w.sessions {
		if kind == "iframe" {
			frames = append(frames, id)
		}
	}
	w.mu.Unlock()

	for _, id := range frames {
		_ = w.conn.call(ctx, id, "Runtime.evaluate", map[string]any{"expression": playScript}, nil)
	}

	var located struct {
		Result struct {
			Value *struct {
				X float64 `json:"x"`
				Y float64 `json:"y"`
			} `json:"value"`
		} `json:"result"`
	}
	if err := w.conn.call(ctx, tab, "Runtime.evaluate", map[string]any{
		"expression": locateScript, "returnByValue": true,
	}, &located); err != nil || located.Result.Value == nil {
		return
	}
	x, y := located.Result.Value.X, located.Result.Value.Y
	for _, kind := range []string{"mouseMoved", "mousePressed", "mouseReleased"} {
		ev := map[string]any{"type": kind, "x": x, "y": y}
		if kind != "mouseMoved" {
			ev["button"] = "left"
			ev["clickCount"] = 1
		}
		if err := w.conn.call(ctx, tab, "Input.dispatchMouseEvent", ev, nil); err != nil {
			return
		}
	}
}

func (w *pageWatch) result(start time.Time) *observation {
	w.mu.Lock()
	defer w.mu.Unlock()
	obs := &observation{
		pageStatus:  w.pageStatus,
		pageURL:     w.pageURL,
		pageRefused: w.pageRefused,
		frames:      w.frames,
		nudges:      w.nudges,
		popups:      w.popups,
		drm:         w.drm.clone(),
		elapsed:     time.Since(start),
		redirects:   append([]string(nil), w.redirects...),
		frameHosts:  append([]string(nil), w.frameHosts...),
	}
	select {
	case <-w.loaded:
		obs.loaded = true
	default:
	}
	for _, m := range w.manifests {
		copied := *m
		copied.headers = make(map[string]string, len(m.headers))
		for k, v := range m.headers {
			copied.headers[k] = v
		}
		obs.manifests = append(obs.manifests, &copied)
	}
	sort.Slice(obs.manifests, func(i, j int) bool { return obs.manifests[i].seq < obs.manifests[j].seq })
	return obs
}

// ── helpers ─────────────────────────────────────────────────────────────────

// mayCarryManifest reports whether a request of this resource type could be a
// player fetching a playlist. Images, scripts, fonts and the like never are.
func mayCarryManifest(resourceType string) bool {
	switch resourceType {
	case "XHR", "Fetch", "Media", "Other", "":
		return true
	}
	return false
}

func mentionsManifest(rawURL string) bool {
	return manifestURL(rawURL) || manifestQuery(rawURL)
}

func isNetworkURL(rawURL string) bool {
	return strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://")
}

// mergeWireHeaders folds CDP header maps into one canonical map. HTTP/2 requests
// report lower-case names and pseudo-headers, which have no use here.
func mergeWireHeaders(dst, src map[string]string) {
	for k, v := range src {
		if strings.HasPrefix(k, ":") {
			continue
		}
		dst[http.CanonicalHeaderKey(k)] = v
	}
}

func resetTimer(t *time.Timer, d time.Duration) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
	t.Reset(d)
}

// describeNetError turns Chromium's error names into something an operator can act on.
func describeNetError(text string) string {
	switch {
	case strings.Contains(text, "ERR_TUNNEL_CONNECTION_FAILED"), strings.Contains(text, "ERR_PROXY_CONNECTION_FAILED"):
		return text + " (the connection was refused; private and internal addresses are blocked)"
	case strings.Contains(text, "ERR_NAME_NOT_RESOLVED"):
		return text + " (the host name does not resolve)"
	}
	return text
}
