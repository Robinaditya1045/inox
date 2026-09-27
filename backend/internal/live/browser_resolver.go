package live

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/inox/inox/backend/internal/domain"
)

// ErrDRMProtected means the page's stream needs a DRM licence to decrypt. The
// resolver reports this rather than trying to get around it.
var ErrDRMProtected = errors.New("stream is DRM-protected")

// Bounds on how long one page is watched. A channel can ask for more or less time
// with resolver_config.timeout_seconds, within these.
const (
	minBrowserBudget = 5 * time.Second
	maxBrowserBudget = 60 * time.Second
	// verifyBudget covers fetching the manifests the page loaded, after the page
	// itself is closed.
	verifyBudget = 20 * time.Second
	// maxManifestChecks bounds those fetches. Pages seldom load more than a master
	// and a variant or two of anything worth restreaming.
	maxManifestChecks = 6
)

// replayedHeaders are carried over from the browser's own manifest request to every
// fetch the proxy makes for the stream. Referer and Origin are what hotlink
// protection checks; User-Agent is part of some CDNs' token signatures.
//
// Cookies travel separately, in a jar seeded from the page's session (see seedJar),
// because the proxy sends one header set to every host a stream touches while a
// cookie belongs to one host or domain. Authorization and headers a player's script
// computes itself are not replayed at all.
var replayedHeaders = []string{"Referer", "Origin", "User-Agent"}

type browserResolver struct {
	browser *Browser
	fetcher *Fetcher
	budget  time.Duration
}

// NewBrowserResolver handles pages that only reveal their stream once they run: the
// page is loaded in headless Chromium, its player is given the chance (or a nudge)
// to start, and the manifest it requests -- from the page, from an iframe at any
// depth, or from a worker -- is picked out of its network traffic.
//
// The expensive tier, for sites where the static resolver finds nothing. Resolves
// share one Chromium through browser, and the manifest found is checked with the
// same Fetcher the proxy will use before it is accepted, so a stream that only
// works inside the browser is reported now rather than failing in a room later.
//
// A page whose stream needs DRM is reported as DRM_PROTECTED, with what gave it
// away: protected manifests, the page's use of Encrypted Media Extensions, licence
// requests. Nothing about the protection is worked around, and no unprotected stream
// that happens to be on the same page is passed off as the channel.
func NewBrowserResolver(browser *Browser, fetcher *Fetcher, budget time.Duration) Resolver {
	return &browserResolver{browser: browser, fetcher: fetcher, budget: budget}
}

func (r *browserResolver) ID() string { return domain.ResolverBrowser }

func (r *browserResolver) Resolve(ctx context.Context, ch *domain.LiveChannel) (*Resolution, error) {
	start := time.Now()
	page, err := url.Parse(ch.SourceURL)
	if err != nil || (page.Scheme != "http" && page.Scheme != "https") || page.Host == "" {
		return nil, resolveFailure(domain.OutcomeFailed, "invalid_config", "browser resolver requires an http(s) page url", nil)
	}
	// The page loads through the egress guard rather than the Fetcher, so the
	// allowlist is applied here -- to fallback sources too, which nothing else
	// validates before they are used.
	if err := r.fetcher.AllowsHost(ch.SourceURL); err != nil {
		return nil, resolveFailure(domain.OutcomeFailed, "host_not_allowed", err.Error(), err)
	}

	var match *regexp.Regexp
	if pattern := configString(ch, "manifest_pattern"); pattern != "" {
		if match, err = regexp.Compile(pattern); err != nil {
			return nil, resolveFailure(domain.OutcomeFailed, "invalid_config",
				"manifest_pattern is not a valid regular expression: "+err.Error(), err)
		}
	}
	budget := r.budget
	if secs := configInt(ch, "timeout_seconds"); secs > 0 {
		budget = time.Duration(secs) * time.Second
	}
	budget = min(max(budget, minBrowserBudget), maxBrowserBudget)

	// A Referer set for every request would be wrong for all but the first, so it
	// goes on the navigation and the page's own requests carry their natural one.
	pageHeaders := configHeaders(ch, "page_headers")
	referrer := ""
	for name, value := range pageHeaders {
		if strings.EqualFold(name, "Referer") {
			referrer = value
			delete(pageHeaders, name)
		}
	}

	diag := &domain.ResolveDiagnostics{
		Resolver: r.ID(),
		Page:     &domain.PageDiagnostics{Host: page.Host, BudgetSeconds: int(budget / time.Second)},
	}
	obs, err := r.browser.watch(ctx, watchOptions{
		pageURL:     ch.SourceURL,
		referrer:    referrer,
		pageHeaders: pageHeaders,
		budget:      budget,
		match:       match,
	})
	if err != nil {
		return nil, fail(err, diag, nil, start)
	}
	diag.Page.Status = obs.pageStatus
	diag.Page.Loaded = obs.loaded
	diag.Page.Frames = obs.frames
	diag.Page.Nudges = obs.nudges
	diag.Page.PopupsClosed = obs.popups
	evidence := obs.drm

	// The server continues the page's session: a stream whose CDN issued the player
	// a cookie gets the same cookie back from the proxy.
	jar := newCookieJar()
	var withheld []string
	diag.Page.Cookies, withheld = seedJar(jar, obs.cookies, time.Now())
	diag.Page.CookiesWithheld = len(withheld)
	diag.Page.CookiesWithheldNames = withheld
	diag.Page.CookieDomains = cookieDomains(obs.cookies)
	diag.Page.Redirects = obs.redirects
	diag.Page.FrameHosts = obs.frameHosts

	requests := obs.manifestsMatching(match)
	for _, m := range obs.manifests {
		if match != nil && !match.MatchString(m.url) {
			diag.Manifests = append(diag.Manifests, domain.ManifestDiagnostic{Host: hostOf(m.url), Verdict: "unmatched"})
		}
	}
	if !anyLoaded(requests) {
		for _, m := range requests {
			diag.Manifests = append(diag.Manifests, notLoaded(m))
		}
		return nil, fail(obs.explainNothingLoaded(match, budget), diag, &evidence, start)
	}

	// An operator Cookie for a source they control is scoped into the session for
	// every candidate host before verification, so the server's check fetch carries
	// it too. Without this the fetch would be refused and a working source reported
	// as browser-bound. Cookie is a per-host credential, so it is scoped, not
	// broadcast; this is a no-op unless the operator set one.
	operatorHeaders := configHeaders(ch, "headers")
	for _, req := range requests {
		if _, dom := scopeCookieHeader(operatorHeaders, req.url, jar); dom != "" {
			diag.Page.CookieDomains = appendUnique(diag.Page.CookieDomains, dom)
		}
	}

	vctx, cancel := context.WithTimeout(ctx, verifyBudget)
	defer cancel()
	choice, checks, err := chooseStream(vctx, r.fetcher.WithCookies(jar), requests, operatorHeaders, diag.Page.Cookies, &evidence)
	diag.Manifests = append(diag.Manifests, checks...)
	if err != nil {
		return nil, fail(err, diag, &evidence, start)
	}

	// An unprotected stream next to confirmed DRM is usually an advert or preview
	// in front of the protected channel. A live one is restreamed, flagged in the
	// diagnostics; a finished recording is certainly not the channel, and passing
	// it off as one would report a DRM source as playable.
	if evidence.confirmed() && choice.ended && match == nil {
		for i := range diag.Manifests {
			if diag.Manifests[i].Verdict == "chosen" {
				diag.Manifests[i].Verdict = "usable"
			}
		}
		return nil, fail(resolveFailure(domain.OutcomeDRMProtected, "only_clear_stream_ended",
			fmt.Sprintf("%s: the page plays its channel through DRM%s; the only unprotected stream it loaded (from %s) is a finished recording, most likely an advert",
				ErrDRMProtected, systemsNote(&evidence), hostOf(choice.url)), ErrDRMProtected), diag, &evidence, start)
	}

	// finishWith carries the session jar (already holding the operator cookie,
	// scoped above) onto the resolution; its headers are then replaced with the
	// browser's replayed ones.
	res := finishWith(ch, choice.url, jar)
	res.Protocol = choice.protocol
	res.Headers = choice.headers
	if expiry, ok := signedURLExpiry(choice.url, time.Now()); ok && (res.ExpiresAt.IsZero() || expiry.Before(res.ExpiresAt)) {
		res.ExpiresAt = expiry
	}

	diag.Outcome = domain.OutcomeStreamFound
	if evidence.confirmed() {
		diag.Reason = "clear_stream_alongside_drm"
	}
	diag.DRM = evidence.diagnostics()
	diag.ElapsedMS = time.Since(start).Milliseconds()
	res.Diagnostics = diag
	return res, nil
}

// fail completes a failed resolve's diagnostics and attaches them to its error, so
// they reach the service, the admin portal and the log together.
func fail(err error, diag *domain.ResolveDiagnostics, evidence *drmEvidence, start time.Time) error {
	diag.Outcome, diag.Reason = failureOutcome(err)
	if evidence != nil {
		diag.DRM = evidence.diagnostics()
	}
	diag.ElapsedMS = time.Since(start).Milliseconds()

	var re *ResolveError
	if errors.As(err, &re) {
		re.Diagnostics = diag
		return err
	}
	return &ResolveError{Outcome: diag.Outcome, Reason: diag.Reason, Message: err.Error(), Diagnostics: diag, cause: err}
}

// streamChoice is the manifest settled on and how to request it.
type streamChoice struct {
	url      string
	protocol string
	headers  map[string]string
	master   bool
	ended    bool
}

type candidate struct {
	req     *manifestRequest
	shape   streamShape
	headers map[string]string
	diag    int // index into the diagnostics
}

// chooseStream fetches the manifests the page loaded, exactly as the proxy will, and
// picks the one to restream. It returns what became of each for diagnostics, and
// folds what their manifests say about DRM into evidence.
//
// Fetching again rather than trusting what the browser saw is the point: it is the
// only way to know the upstream accepts a request from the server, with the replayed
// headers, before a viewer is waiting on it.
func chooseStream(ctx context.Context, fetcher *Fetcher, requests []*manifestRequest, operatorHeaders map[string]string, carried int, evidence *drmEvidence) (*streamChoice, []domain.ManifestDiagnostic, error) {
	var (
		diags   []domain.ManifestDiagnostic
		usable  []*candidate
		refused []string
		// sent and clearances describe the cookies the player's own requests
		// carried, for the refused ones.
		sent       int
		clearances []string
		disallowed string
		timedOut   bool
		protected  bool
		checked    int
	)
	record := func(req *manifestRequest, verdict, detail string, shape *manifestShape) int {
		d := domain.ManifestDiagnostic{Host: hostOf(req.url), Verdict: verdict, Detail: detail}
		d.BrowserCookies, _ = sentCookies(req.headers["Cookie"])
		if shape != nil {
			d.Protocol, d.Kind, d.Live, d.DRM = shape.protocol, manifestKind(*shape), !shape.ended, shape.drm
		}
		diags = append(diags, d)
		return len(diags) - 1
	}
	failed := func(i int, req *manifestRequest, err error) {
		diags[i].Verdict, diags[i].Detail = "refused", err.Error()
		switch {
		case errors.Is(err, ErrHostNotAllowed):
			diags[i].Verdict = "not_allowed"
			disallowed = diags[i].Host
		case errors.Is(err, context.DeadlineExceeded):
			diags[i].Detail = "timed out"
			timedOut = true
		default:
			// The browser loaded this manifest (it is here because req.loaded() was
			// true) but the server's equivalent request was refused. Name that state
			// so it reads apart from a manifest that failed in the browser too.
			diags[i].Verdict = "browser_only"
			refused = appendUnique(refused, fmt.Sprintf("%s: %v", diags[i].Host, err))
			n, bots := sentCookies(req.headers["Cookie"])
			sent = max(sent, n)
			clearances = appendUnique(clearances, bots...)
		}
	}

	for _, req := range requests {
		switch {
		case !req.loaded():
			diags = append(diags, notLoaded(req))
			continue
		case checked == maxManifestChecks:
			record(req, "unchecked", "", nil)
			continue
		}
		checked++
		headers := requestContext(req.headers, operatorHeaders)
		shape, err := inspectStream(ctx, fetcher, req.url, headers, false)
		if err != nil {
			failed(record(req, "refused", "", nil), req, err)
			continue
		}
		if shape.protocol == "" {
			record(req, "not_a_manifest", "", nil)
			continue
		}
		evidence.noteManifest(shape.manifestShape)
		if len(shape.drm) > 0 {
			protected = true
			record(req, "drm_protected", "", &shape.manifestShape)
			continue
		}
		usable = append(usable, &candidate{req: req, shape: shape, headers: headers,
			diag: record(req, "usable", "", &shape.manifestShape)})
	}

	// Ranking reads one rendition per stream; the one chosen is read in full before
	// it is accepted, because a single protected rendition breaks playback as soon
	// as adaptive bitrate switches to it.
	sort.SliceStable(usable, func(i, j int) bool { return rankAbove(usable[i], usable[j]) })
	for _, c := range usable {
		full, err := inspectStream(ctx, fetcher, c.req.url, c.headers, true)
		if err != nil {
			failed(c.diag, c.req, err)
			continue
		}
		evidence.noteManifest(full.manifestShape)
		if len(full.drm) > 0 {
			protected = true
			diags[c.diag].Verdict, diags[c.diag].DRM = "drm_protected", full.drm
			diags[c.diag].Detail = fmt.Sprintf("%d of %d renditions protected", full.protected, full.renditions)
			continue
		}
		diags[c.diag].Verdict = "chosen"
		return &streamChoice{url: c.req.url, protocol: full.protocol, headers: c.headers, master: full.master, ended: full.ended}, diags, nil
	}

	switch {
	case protected:
		reason := "manifest_protected"
		if evidence.protected < evidence.renditions {
			reason = "renditions_protected"
		}
		return nil, diags, resolveFailure(domain.OutcomeDRMProtected, reason,
			fmt.Sprintf("%s: the page's stream requires%s%s; only unencrypted or AES-128 HLS can be restreamed",
				ErrDRMProtected, systemsNote(evidence), renditionsNote(evidence)), ErrDRMProtected)
	case disallowed != "":
		return nil, diags, resolveFailure(domain.OutcomeFailed, "host_not_allowed",
			fmt.Sprintf("the page's player streams from %s, which is not in LIVE_SOURCE_ALLOWED_HOSTS; add it there to restream this channel", disallowed), ErrHostNotAllowed)
	case len(refused) > 0:
		return nil, diags, refusal(refused, sent, carried, clearances)
	case timedOut:
		return nil, diags, resolveFailure(domain.OutcomeTimeout, "verification_timeout",
			fmt.Sprintf("checking the page's manifests took longer than %s", verifyBudget), context.DeadlineExceeded)
	}
	return nil, diags, resolveFailure(domain.OutcomeNoStreamFound, "not_a_manifest",
		"the page requested manifest-like URLs, but none of them returned an HLS or DASH manifest", nil)
}

// refusal explains why the server was refused a manifest the player loaded. The
// stream was discovered and is browser-playable; what failed is server-side replay,
// the only mode Inox can restream through. The manifest URL is a credential and the
// upstream sends no CORS headers (see Proxy), so the frontend cannot fetch it
// directly either -- server-side proxying is not an implementation choice we can
// route around here, it is the only architecture available. So this is reported as
// SERVER_REPLAY_FAILED, not worked around, going by the one difference left between
// the two requests that the resolver can see.
func refusal(refused []string, sent, carried int, clearances []string) error {
	lead := fmt.Sprintf("the page's player loaded this stream, but the same request from the server was refused (%s)", strings.Join(refused, "; "))
	tail := "so this source cannot be server-side restreamed, which is the only way Inox can play it"
	switch {
	case len(clearances) > 0:
		return resolveFailure(domain.OutcomeServerReplayFailed, "bot_protected", fmt.Sprintf(
			"%s. The player's request carried a bot-protection clearance (%s), which the resolver does not carry over: "+
				"it certifies a check the server never took, %s", lead, strings.Join(clearances, ", "), tail), nil)
	case sent > 0 && carried == 0:
		// The only genuinely transient case: the cookies were there but unreadable.
		return resolveFailure(domain.OutcomeFailed, "upstream_refused", fmt.Sprintf(
			"%s. The player's request carried cookies, but none could be read from the browser to carry over; try again", lead), nil)
	case sent > 0:
		return resolveFailure(domain.OutcomeServerReplayFailed, "browser_bound", fmt.Sprintf(
			"%s, even with the page session's cookies carried over. The host ties the stream to the browser itself -- "+
				"a value its player computes, or the browser's network fingerprint -- which the resolver does not imitate, %s", lead, tail), nil)
	}
	return resolveFailure(domain.OutcomeServerReplayFailed, "browser_bound", fmt.Sprintf(
		"%s. The player's request carried no cookies, so the host is checking something else -- a value the player computes, "+
			"or the browser's network fingerprint -- which the resolver does not imitate, %s", lead, tail), nil)
}

// rankAbove orders candidate streams: HLS before DASH, because the proxy rewrites
// HLS; a live window before a finished recording, which on a live page is usually an
// ad or a preview; a master playlist before a variant, so viewers keep adaptive
// bitrate; and otherwise whichever the page asked for first.
func rankAbove(a, b *candidate) bool {
	if (a.shape.protocol == "hls") != (b.shape.protocol == "hls") {
		return a.shape.protocol == "hls"
	}
	if a.shape.ended != b.shape.ended {
		return !a.shape.ended
	}
	if a.shape.master != b.shape.master {
		return a.shape.master
	}
	return a.req.seq < b.req.seq
}

// requestContext is the header set the proxy sends upstream for a stream: what the
// browser sent that hotlink checks look at, overlaid by anything the operator set
// explicitly in resolver_config.headers.
func requestContext(wire, operator map[string]string) map[string]string {
	out := make(map[string]string, len(replayedHeaders)+len(operator))
	for _, name := range replayedHeaders {
		if v := wire[name]; v != "" {
			out[name] = v
		}
	}
	for name, value := range operator {
		if http.CanonicalHeaderKey(name) == "Cookie" {
			continue // cookies travel in the jar, scoped per host; see scopeCookieHeader
		}
		out[http.CanonicalHeaderKey(name)] = value
	}
	return out
}

func (o *observation) manifestsMatching(match *regexp.Regexp) []*manifestRequest {
	if match == nil {
		return o.manifests
	}
	var out []*manifestRequest
	for _, m := range o.manifests {
		if match.MatchString(m.url) {
			out = append(out, m)
		}
	}
	return out
}

func anyLoaded(requests []*manifestRequest) bool {
	for _, m := range requests {
		if m.loaded() {
			return true
		}
	}
	return false
}

// explainNothingLoaded says, as precisely as it can, why no stream turned up. Every
// case is something only the operator can act on.
func (o *observation) explainNothingLoaded(match *regexp.Regexp, budget time.Duration) error {
	switch {
	case o.pageRefused != "":
		return resolveFailure(domain.OutcomeFailed, "address_blocked",
			fmt.Sprintf("the page is on an address the headless browser may not reach (%s)", o.pageRefused), ErrBlockedAddress)
	case o.pageStatus >= 400:
		return o.statusFailure()
	case o.drm.confirmed():
		// Players set up DRM before or instead of requesting anything recognisable
		// -- a stream behind EME needs no manifest the resolver could use anyway.
		return resolveFailure(domain.OutcomeDRMProtected, "eme_session",
			fmt.Sprintf("%s: the page's player set up DRM playback%s and requested no unprotected stream; only unencrypted or AES-128 HLS can be restreamed",
				ErrDRMProtected, systemsNote(&o.drm)), ErrDRMProtected)
	case match != nil && len(o.manifests) > 0 && len(o.manifestsMatching(match)) == 0:
		return resolveFailure(domain.OutcomeNoStreamFound, "pattern_unmatched",
			fmt.Sprintf("the player requested %d manifest-like URL(s), but none matched manifest_pattern %q", len(o.manifests), match.String()), nil)
	}
	for _, m := range o.manifestsMatching(match) {
		switch {
		case m.failure != "":
			return resolveFailure(domain.OutcomeFailed, "manifest_load_failed",
				fmt.Sprintf("the player requested a manifest from %s, but it failed to load: %s", hostOf(m.url), describeNetError(m.failure)), nil)
		case m.status != 0:
			return resolveFailure(domain.OutcomeFailed, "manifest_load_failed",
				fmt.Sprintf("the player requested a manifest from %s, but it returned HTTP %d", hostOf(m.url), m.status), nil)
		}
	}
	switch {
	case !o.loaded:
		return resolveFailure(domain.OutcomeTimeout, "page_still_loading",
			fmt.Sprintf("the page was still loading after %s and its player had not requested a stream; raise timeout_seconds if the site is slow", budget), nil)
	case len(o.drm.requestedSystems()) > 0:
		// Asking which DRM the browser supports proves nothing on its own, but with
		// no stream at all it is the likeliest explanation, so say so.
		return resolveFailure(domain.OutcomeNoStreamFound, "drm_suspected",
			fmt.Sprintf("no HLS or DASH manifest was requested within %s; the player asked for %s, so the stream is probably DRM-protected",
				budget, strings.Join(o.drm.requestedSystems(), ", ")), nil)
	}
	return resolveFailure(domain.OutcomeNoStreamFound, "no_manifest_requested",
		fmt.Sprintf("no HLS or DASH manifest was requested within %s (watched %d frame(s), tried starting playback %d time(s)); "+
			"the player may need a sign-in or consent click", budget, o.frames+1, o.nudges), nil)
}

// statusFailure diagnoses a page whose own document came back with an HTTP error.
// The status codes mean different things and call for different fixes, so they get
// different reasons rather than one "http error": a stale URL, an access-controlled
// page, and a rate limit are not the same problem. The redirect chain and the URL
// that actually answered are included, since a 404 after a redirect is the site's
// doing while a 404 on the source URL is usually a stale channel URL.
func (o *observation) statusFailure() error {
	landed := hostOf(o.pageURL)
	where := fmt.Sprintf("HTTP %d", o.pageStatus)
	if len(o.redirects) > 0 {
		where += fmt.Sprintf(" after redirecting through %s to %s", strings.Join(o.redirects, " → "), landed)
	}
	switch o.pageStatus {
	case 404, 410:
		return resolveFailure(domain.OutcomeFailed, "page_not_found",
			fmt.Sprintf("the page URL returned %s; the channel's page URL is most likely stale or wrong -- open it in a browser and update it", where), nil)
	case 401, 403:
		return resolveFailure(domain.OutcomeFailed, "page_forbidden",
			fmt.Sprintf("the page URL returned %s; it is access-controlled and the headless browser was not allowed in. This resolver does not defeat access controls", where), nil)
	case 429:
		return resolveFailure(domain.OutcomeFailed, "page_rate_limited",
			fmt.Sprintf("the page URL returned %s; the site is rate-limiting our requests. Resolving less often may help", where), nil)
	}
	if o.pageStatus >= 500 {
		return resolveFailure(domain.OutcomeFailed, "page_server_error",
			fmt.Sprintf("the page URL returned %s; the site's own server is failing, which usually passes on its own", where), nil)
	}
	return resolveFailure(domain.OutcomeFailed, "page_http_error",
		fmt.Sprintf("the page URL returned %s", where), nil)
}

// notLoaded describes a manifest the page's player failed to load itself.
func notLoaded(m *manifestRequest) domain.ManifestDiagnostic {
	d := domain.ManifestDiagnostic{Host: hostOf(m.url), Verdict: "not_loaded"}
	switch {
	case m.failure != "":
		d.Detail = "failed in the browser: " + m.failure
	case m.status != 0:
		d.Detail = fmt.Sprintf("HTTP %d in the browser", m.status)
	default:
		d.Detail = "no response in the browser"
	}
	return d
}

func manifestKind(shape manifestShape) string {
	switch {
	case shape.protocol == "dash":
		return "mpd"
	case shape.master:
		return "master"
	case shape.protocol == "hls":
		return "media"
	}
	return ""
}

func systemsNote(e *drmEvidence) string {
	if len(e.systems) == 0 {
		return ""
	}
	return " (" + strings.Join(e.systems, ", ") + ")"
}

func renditionsNote(e *drmEvidence) string {
	if e.renditions == 0 || e.protected == e.renditions {
		return ""
	}
	return fmt.Sprintf(" for %d of its %d renditions", e.protected, e.renditions)
}
