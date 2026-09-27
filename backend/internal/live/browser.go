package live

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/inox/inox/backend/internal/domain"
)

var (
	errBrowserClosed  = errors.New("headless browser is shut down")
	errBrowserExited  = errors.New("headless browser exited")
	errBrowserStopped = errors.New("headless browser stopped")
)

// BrowserConfig configures the headless Chromium that browser resolves share.
type BrowserConfig struct {
	// ExecPath is the Chromium (or Chrome) binary.
	ExecPath string
	// MaxConcurrency bounds how many pages are resolved at once. Each is one tab, and
	// the production VM has a single core, so resolves beyond this queue.
	MaxConcurrency int
	// IdleTimeout stops Chromium once no resolve has used it for this long. A
	// browser-resolved channel only needs it again when its manifest expires, and
	// an idle Chromium still holds a few hundred megabytes.
	IdleTimeout time.Duration
	// NoSandbox turns off Chromium's own sandbox, for hosts where it cannot create
	// the user namespaces it is built on. Pages are untrusted third-party code, so
	// this is a real loss of isolation: prefer allowing namespaces for the container.
	NoSandbox bool
}

// FindChromium returns the first Chromium-family browser on PATH.
func FindChromium() (string, bool) {
	for _, name := range []string{"chromium", "chromium-browser", "google-chrome-stable", "google-chrome", "chrome"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, true
		}
	}
	return "", false
}

// Browser shares one headless Chromium between all browser resolves.
//
// Starting Chromium costs a second or two of CPU and around a hundred megabytes
// before it has opened a single page, so it is launched on first use and kept while
// resolves keep arriving. Isolation between resolves comes from giving each one its
// own browser context -- an incognito profile with its own cookies, storage and
// cache -- which is disposed as soon as that resolve ends. If Chromium crashes, the
// next resolve launches a fresh one.
type Browser struct {
	cfg   BrowserConfig
	dial  func(ctx context.Context, network, address string) (net.Conn, error)
	slots chan struct{}

	mu       sync.Mutex
	proc     *chromium
	active   int
	idle     *time.Timer
	closed   bool
	launches int

	// launchMu serializes Chromium launches so only one happens at a time, while
	// letting the blocking launch run without b.mu held -- a cold start must not
	// stall every other resolve's acquire, release or idle check.
	launchMu sync.Mutex
}

// NewBrowser prepares a shared browser. Nothing is launched until the first resolve.
// Every connection a page makes is dialled through the fetcher's address guard.
func NewBrowser(cfg BrowserConfig, fetcher *Fetcher) *Browser {
	if cfg.MaxConcurrency < 1 {
		cfg.MaxConcurrency = 1
	}
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = 5 * time.Minute
	}
	return &Browser{
		cfg:   cfg,
		dial:  fetcher.DialContext,
		slots: make(chan struct{}, cfg.MaxConcurrency),
	}
}

// watch loads one page and reports what its player requested.
func (b *Browser) watch(ctx context.Context, opts watchOptions) (*observation, error) {
	proc, release, err := b.acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	return proc.watch(ctx, opts)
}

// acquire takes a concurrency slot and returns a running Chromium, launching one if
// none is running or the last one died.
func (b *Browser) acquire(ctx context.Context) (*chromium, func(), error) {
	select {
	case b.slots <- struct{}{}:
	case <-ctx.Done():
		return nil, nil, resolveFailure(domain.OutcomeTimeout, "browser_busy",
			"no headless browser slot became free before the resolve ran out of time; "+
				"other channels were resolving -- raise LIVE_BROWSER_MAX_CONCURRENCY if this keeps happening", ctx.Err())
	}

	// Reserve the resolve before doing anything slow. With active incremented,
	// stopIfIdle sees the browser as in use and will not stop the process out from
	// under a launch that runs with b.mu released.
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		<-b.slots
		return nil, nil, resolveFailure(domain.OutcomeFailed, "browser_unavailable", errBrowserClosed.Error(), errBrowserClosed)
	}
	if b.idle != nil {
		b.idle.Stop()
		b.idle = nil
	}
	b.active++
	proc := b.proc
	fresh := proc != nil && !proc.dead()
	b.mu.Unlock()

	if !fresh {
		var err error
		if proc, err = b.launchProc(ctx); err != nil {
			b.decActive()
			<-b.slots
			return nil, nil, err
		}
	}

	var once sync.Once
	release := func() {
		once.Do(func() {
			b.decActive()
			<-b.slots
		})
	}
	return proc, release, nil
}

// decActive gives back one reservation, arming the idle timer when the last resolve
// finishes.
func (b *Browser) decActive() {
	b.mu.Lock()
	b.active--
	if b.active == 0 && !b.closed && b.proc != nil {
		b.idle = time.AfterFunc(b.cfg.IdleTimeout, b.stopIfIdle)
	}
	b.mu.Unlock()
}

// launchProc returns a live Chromium, launching one if the current process is
// missing or dead. The launch -- a blocking handshake -- runs without b.mu held and
// is serialized by launchMu, so concurrent acquires do not each start a browser and
// do not block one another (or release, or the idle check) while one starts.
func (b *Browser) launchProc(ctx context.Context) (*chromium, error) {
	b.launchMu.Lock()
	defer b.launchMu.Unlock()

	b.mu.Lock()
	closed := b.closed
	current := b.proc
	b.mu.Unlock()
	if closed {
		return nil, resolveFailure(domain.OutcomeFailed, "browser_unavailable", errBrowserClosed.Error(), errBrowserClosed)
	}
	if current != nil && !current.dead() {
		return current, nil // another acquire launched while we waited for launchMu
	}
	if current != nil {
		slog.Warn("headless browser exited unexpectedly; relaunching", "stderr", crashDiagnosis(current.stderr.String()))
		go current.stop()
	}

	proc, err := launchChromium(ctx, b.cfg, b.dial)
	if err != nil {
		b.mu.Lock()
		if b.proc == current {
			b.proc = nil
		}
		b.mu.Unlock()
		return nil, resolveFailure(domain.OutcomeFailed, "browser_unavailable", err.Error(), err)
	}
	b.mu.Lock()
	if b.closed {
		// Close ran while we were launching; do not store a live orphan it will
		// never stop.
		b.mu.Unlock()
		proc.stop()
		return nil, resolveFailure(domain.OutcomeFailed, "browser_unavailable", errBrowserClosed.Error(), errBrowserClosed)
	}
	b.proc = proc
	b.launches++
	b.mu.Unlock()
	return proc, nil
}

func (b *Browser) stopIfIdle() {
	b.mu.Lock()
	if b.active > 0 || b.proc == nil {
		b.mu.Unlock()
		return
	}
	proc := b.proc
	b.proc = nil
	b.mu.Unlock()

	proc.stop()
	slog.Info("stopped idle headless browser")
}

// Close stops Chromium and refuses further resolves. Resolves in flight fail.
func (b *Browser) Close() {
	b.mu.Lock()
	b.closed = true
	if b.idle != nil {
		b.idle.Stop()
		b.idle = nil
	}
	proc := b.proc
	b.proc = nil
	b.mu.Unlock()

	if proc != nil {
		proc.stop()
	}
}

// ── one running Chromium ────────────────────────────────────────────────────

type chromium struct {
	cmd     *exec.Cmd
	conn    *cdpConn
	egress  *egressProxy
	profile string
	stderr  *tailBuffer
	exited  chan struct{}

	mu sync.Mutex
	// contexts routes browser-level target events to the resolve that owns the
	// browser context they happened in.
	contexts map[string]func(targetInfo)
}

type targetInfo struct {
	TargetID         string `json:"targetId"`
	Type             string `json:"type"`
	URL              string `json:"url"`
	BrowserContextID string `json:"browserContextId"`
}

func launchChromium(ctx context.Context, cfg BrowserConfig, dial func(ctx context.Context, network, address string) (net.Conn, error)) (*chromium, error) {
	egress, err := newEgressProxy(dial)
	if err != nil {
		return nil, fmt.Errorf("could not start the headless browser's egress proxy: %w", err)
	}
	profile, err := os.MkdirTemp("", "inox-chromium-")
	if err != nil {
		egress.close()
		return nil, fmt.Errorf("could not create a headless browser profile: %w", err)
	}
	// Chromium reads commands on fd 3 and writes replies and events to fd 4.
	cmdRead, cmdWrite, err := os.Pipe()
	if err != nil {
		egress.close()
		_ = os.RemoveAll(profile)
		return nil, err
	}
	eventRead, eventWrite, err := os.Pipe()
	if err != nil {
		egress.close()
		_ = os.RemoveAll(profile)
		cmdRead.Close()
		cmdWrite.Close()
		return nil, err
	}

	c := &chromium{
		egress:   egress,
		profile:  profile,
		stderr:   &tailBuffer{limit: 16 << 10},
		exited:   make(chan struct{}),
		contexts: make(map[string]func(targetInfo)),
	}
	c.cmd = exec.Command(cfg.ExecPath, chromiumArgs(cfg, profile, egress.URL())...)
	c.cmd.ExtraFiles = []*os.File{cmdRead, eventWrite}
	c.cmd.Env = chromiumEnv(profile)
	c.cmd.Stderr = c.stderr
	configureChromiumProcess(c.cmd)

	err = c.cmd.Start()
	// The child holds its own copies of these ends now.
	cmdRead.Close()
	eventWrite.Close()
	if err != nil {
		cmdWrite.Close()
		eventRead.Close()
		egress.close()
		_ = os.RemoveAll(profile)
		return nil, fmt.Errorf("could not start the headless browser at %s: %w", cfg.ExecPath, err)
	}

	c.conn = newCDPConn(cmdWrite, eventRead)
	go func() {
		_ = c.cmd.Wait()
		c.conn.close(errBrowserExited)
		eventRead.Close()
		close(c.exited)
	}()

	hctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var version struct {
		Product string `json:"product"`
	}
	if err := c.conn.call(hctx, "", "Browser.getVersion", nil, &version); err != nil {
		// Give a crashing browser a moment to finish writing why.
		select {
		case <-c.exited:
		case <-time.After(time.Second):
		}
		c.stop()
		return nil, launchError(err, c.stderr.String(), cfg.NoSandbox)
	}

	c.conn.listen("", c.onBrowserEvent)
	// Discovery is what reports popups, so they can be closed rather than left
	// running whatever an ad opened.
	if err := c.conn.call(hctx, "", "Target.setDiscoverTargets", map[string]any{"discover": true}, nil); err != nil {
		c.stop()
		return nil, fmt.Errorf("headless browser did not accept target discovery: %w", err)
	}
	slog.Info("started headless browser for live channel resolves", "product", version.Product, "sandbox", !cfg.NoSandbox)
	return c, nil
}

// chromiumArgs is the whole command line. The proxy and QUIC/WebRTC flags are what
// force every byte a page sends through the egress guard.
func chromiumArgs(cfg BrowserConfig, profile, proxyURL string) []string {
	args := []string{
		"--headless",
		"--remote-debugging-pipe",
		"--user-data-dir=" + profile,
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-background-networking",
		"--disable-background-timer-throttling",
		"--disable-backgrounding-occluded-windows",
		"--disable-renderer-backgrounding",
		"--disable-breakpad",
		"--disable-component-update",
		"--disable-default-apps",
		"--disable-dev-shm-usage", // Docker's /dev/shm is 64 MB, which Chromium outgrows
		"--disable-extensions",
		// No GPU and no separate GPU process: compositing is done in software on a
		// thread of the browser process. Nothing here needs a GPU, and the GPU
		// process is where builds break -- Alpine's SwiftShader lacks the Vulkan
		// extensions it needs, and the GPU process's seccomp policy kills it for a
		// syscall musl makes (pwritev2), which takes the whole browser down with
		// it. Renderers, which run the pages, stay sandboxed.
		"--disable-gpu",
		"--disable-software-rasterizer",
		"--in-process-gpu",
		"--disable-sync",
		"--disable-features=Translate,MediaRouter,OptimizationHints,DialMediaRouteProvider",
		"--metrics-recording-only",
		"--mute-audio",
		"--password-store=basic",
		// Players that autoplay should not need a click to start requesting.
		"--autoplay-policy=no-user-gesture-required",
		"--proxy-server=" + proxyURL,
		// Without this, localhost and 127.0.0.1 skip the proxy -- and the guard.
		"--proxy-bypass-list=<-loopback>",
		"--disable-quic",
		"--force-webrtc-ip-handling-policy=disable_non_proxied_udp",
		"--window-size=1280,720",
		"--hide-scrollbars",
	}
	if cfg.NoSandbox {
		args = append(args, "--no-sandbox")
	}
	return append(args, "about:blank")
}

// chromiumEnv is the entire environment Chromium gets. The server's own environment
// holds DATABASE_URL, SESSION_SECRET and storage credentials, and a renderer running
// a hostile page must not be able to read any of them out of its own memory.
func chromiumEnv(profile string) []string {
	env := []string{
		"HOME=" + profile,
		"XDG_CONFIG_HOME=" + profile,
		"XDG_CACHE_HOME=" + profile,
	}
	for _, key := range []string{"PATH", "TMPDIR", "TZ", "LANG", "LC_ALL"} {
		if v, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+v)
		}
	}
	return env
}

func launchError(err error, stderr string, noSandbox bool) error {
	msg := fmt.Sprintf("headless browser failed to start: %v", err)
	if why := crashDiagnosis(stderr); why != "" {
		msg += " -- " + why
	}
	lower := strings.ToLower(stderr)
	if !noSandbox && (strings.Contains(lower, "namespace") || strings.Contains(lower, "sandbox")) {
		msg += ". Chromium could not create its sandbox: allow user namespaces for this container " +
			"(Docker's default seccomp profile blocks them; see deploy/oracle/docker-compose.prod.yml), " +
			"or set LIVE_BROWSER_NO_SANDBOX=true to run pages without it"
	}
	return errors.New(msg)
}

func (c *chromium) onBrowserEvent(method string, params json.RawMessage) {
	if method != "Target.targetCreated" {
		return
	}
	var ev struct {
		TargetInfo targetInfo `json:"targetInfo"`
	}
	if json.Unmarshal(params, &ev) != nil {
		return
	}
	c.mu.Lock()
	route := c.contexts[ev.TargetInfo.BrowserContextID]
	c.mu.Unlock()
	if route != nil {
		route(ev.TargetInfo)
	}
}

func (c *chromium) routeContext(contextID string, fn func(targetInfo)) {
	c.mu.Lock()
	c.contexts[contextID] = fn
	c.mu.Unlock()
}

func (c *chromium) unrouteContext(contextID string) {
	c.mu.Lock()
	delete(c.contexts, contextID)
	c.mu.Unlock()
}

func (c *chromium) dead() bool {
	select {
	case <-c.exited:
		return true
	default:
		return false
	}
}

// stop shuts Chromium down and removes everything it left on disk.
func (c *chromium) stop() {
	if !c.dead() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = c.conn.call(ctx, "", "Browser.close", nil, nil)
		cancel()
		select {
		case <-c.exited:
		case <-time.After(3 * time.Second):
		}
	}
	// Helpers (zygote, renderers, the network service) share its process group.
	killChromium(c.cmd)
	select {
	case <-c.exited:
	case <-time.After(5 * time.Second):
		slog.Warn("headless browser did not exit after being killed", "pid", c.cmd.Process.Pid)
	}
	c.conn.close(errBrowserStopped)
	c.egress.close()
	_ = os.RemoveAll(c.profile)
}

// tailBuffer keeps the last limit bytes written to it: enough of Chromium's stderr to
// say why a launch failed, without holding everything it logs for hours.
type tailBuffer struct {
	mu    sync.Mutex
	limit int
	buf   []byte
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if over := len(t.buf) - t.limit; over > 0 {
		t.buf = append(t.buf[:0], t.buf[over:]...)
	}
	return len(p), nil
}

func (t *tailBuffer) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.buf)
}

// crashDiagnosis picks the lines of Chromium's stderr that say why it died. It logs
// plenty of harmless noise -- D-Bus, the crash reporter -- so the last lines are
// rarely the ones that matter.
func crashDiagnosis(stderr string) string {
	var telling []string
	for _, line := range strings.Split(stderr, "\n") {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "dbus") || strings.Contains(lower, "crashpad") {
			continue
		}
		if strings.Contains(lower, "fatal") || strings.Contains(lower, "sandbox") ||
			strings.Contains(lower, "namespace") || strings.Contains(lower, "seccomp") {
			telling = append(telling, strings.TrimSpace(line))
		}
	}
	if len(telling) == 0 {
		return lastLines(stderr, 3)
	}
	if len(telling) > 3 {
		telling = telling[len(telling)-3:]
	}
	return strings.Join(telling, " | ")
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.TrimSpace(strings.Join(lines, " | "))
}
