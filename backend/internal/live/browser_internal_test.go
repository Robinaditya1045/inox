package live

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// chromiumForTest finds a browser or skips. As in the resolver tests, the sandbox
// is off unless LIVE_BROWSER_TEST_SANDBOX=true.
func chromiumForTest(t *testing.T, idle time.Duration, concurrency int) *Browser {
	t.Helper()
	AcquireBrowserSlot(t)
	path := os.Getenv("LIVE_BROWSER_PATH")
	if path == "" {
		found, ok := FindChromium()
		if !ok {
			t.Skip("no Chromium-family browser installed; set LIVE_BROWSER_PATH to run browser tests")
		}
		path = found
	}
	noSandbox := os.Getenv("LIVE_BROWSER_TEST_SANDBOX") != "true"
	b := NewBrowser(BrowserConfig{ExecPath: path, MaxConcurrency: concurrency, IdleTimeout: idle, NoSandbox: noSandbox},
		NewFetcher("", false, AllowLoopback()))
	t.Cleanup(b.Close)
	return b
}

// popupPage is a click-to-play player whose click also opens a popup, as ad-laden
// players do.
func popupPage(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/live.m3u8":
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			fmt.Fprint(w, "#EXTM3U\n#EXT-X-TARGETDURATION:6\n#EXTINF:6,\na.ts\n")
		default:
			fmt.Fprint(w, `<!doctype html><html><body style="margin:0">
<div id="p" style="width:100vw;height:100vh"><video style="width:100%;height:100%"></video></div>
<script>
document.getElementById('p').addEventListener('click', () => {
  window.open('/ad', '_blank');
  fetch('/live.m3u8');
}, { once: true });
</script></body></html>`)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func watchPage(t *testing.T, b *Browser, pageURL string) *observation {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	obs, err := b.watch(ctx, watchOptions{pageURL: pageURL, budget: 20 * time.Second})
	if err != nil {
		t.Fatalf("watch: %v", err)
	}
	return obs
}

func (b *Browser) launchCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.launches
}

func (b *Browser) running() *chromium {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.proc
}

func TestBrowserReusesOneChromiumAndClosesPopups(t *testing.T) {
	t.Parallel() // each test drives its own Chromium
	b := chromiumForTest(t, time.Minute, 2)
	page := popupPage(t)

	for i := 0; i < 2; i++ {
		obs := watchPage(t, b, page.URL+"/watch")
		if len(obs.manifests) != 1 || !obs.manifests[0].loaded() {
			t.Fatalf("resolve %d: manifests = %+v, want the one the click requested", i, obs.manifests)
		}
		if obs.popups < 1 {
			t.Errorf("resolve %d: popups closed = %d, want the ad window closed", i, obs.popups)
		}
	}
	if n := b.launchCount(); n != 1 {
		t.Errorf("Chromium launched %d times for two resolves, want 1", n)
	}
}

func TestBrowserStopsWhenIdleAndStartsAgainOnDemand(t *testing.T) {
	t.Parallel() // each test drives its own Chromium
	b := chromiumForTest(t, 300*time.Millisecond, 1)
	page := popupPage(t)

	watchPage(t, b, page.URL+"/watch")
	first := b.running()
	if first == nil {
		t.Fatal("no browser running right after a resolve")
	}

	deadline := time.Now().Add(10 * time.Second)
	for b.running() != nil && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if b.running() != nil {
		t.Fatal("browser still running long after the idle timeout")
	}
	// The handle is dropped before the process is stopped, so a resolve arriving
	// meanwhile never waits on a browser that is shutting down.
	select {
	case <-first.exited:
	case <-time.After(10 * time.Second):
		t.Fatal("idle browser was forgotten but its process never exited")
	}
	for time.Now().Before(deadline) {
		if _, err := os.Stat(first.profile); os.IsNotExist(err) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if _, err := os.Stat(first.profile); !os.IsNotExist(err) {
		t.Errorf("idle browser left its profile behind: %v", err)
	}

	watchPage(t, b, page.URL+"/watch")
	if n := b.launchCount(); n != 2 {
		t.Errorf("launches = %d, want a fresh launch after the idle stop", n)
	}
}

func TestBrowserRelaunchesAfterACrash(t *testing.T) {
	t.Parallel() // each test drives its own Chromium
	b := chromiumForTest(t, time.Minute, 1)
	page := popupPage(t)

	watchPage(t, b, page.URL+"/watch")
	crashed := b.running()
	killChromium(crashed.cmd)
	select {
	case <-crashed.exited:
	case <-time.After(10 * time.Second):
		t.Fatal("killed browser never exited")
	}

	obs := watchPage(t, b, page.URL+"/watch")
	if len(obs.manifests) != 1 {
		t.Errorf("resolve after crash found %d manifests, want 1", len(obs.manifests))
	}
	if n := b.launchCount(); n != 2 {
		t.Errorf("launches = %d, want one relaunch after the crash", n)
	}
}

func TestBrowserQueueGivesUpWithTheCaller(t *testing.T) {
	t.Parallel() // each test drives its own Chromium
	b := chromiumForTest(t, time.Minute, 1)

	_, release, err := b.acquire(context.Background())
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer release()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if _, _, err := b.acquire(ctx); err == nil || !strings.Contains(err.Error(), "slot") {
		t.Errorf("second acquire = %v, want it to give up waiting for the only slot", err)
	}
}

func TestLaunchErrorExplainsAMissingSandbox(t *testing.T) {
	// What Alpine's Chromium writes when Docker's default seccomp profile stops it
	// creating the namespaces its sandbox needs.
	stderr := strings.Join([]string{
		`[1:1:0926/202927.500001:ERROR:dbus/bus.cc:405] Failed to connect to the bus: Could not parse server address`,
		`Failed to move to new namespace: PID namespaces supported, Network namespace supported, but failed: errno = Operation not permitted`,
		`[1:1:0926/202927.513074:FATAL:content/browser/zygote_host/zygote_host_impl_linux.cc:213] Zygote process exited prematurely with exit code 1`,
		`[0926/202927.518175:WARNING:third_party/crashpad/crashpad/snapshot/linux/process_reader_linux.cc:95] sched_getscheduler: Function not implemented (38)`,
		`[0926/202927.518294:WARNING:third_party/crashpad/crashpad/snapshot/linux/process_reader_linux.cc:95] sched_getscheduler: Function not implemented (38)`,
	}, "\n")

	msg := launchError(errBrowserExited, stderr, false).Error()
	for _, want := range []string{"Failed to move to new namespace", "Zygote process exited", "LIVE_BROWSER_NO_SANDBOX", "seccomp"} {
		if !strings.Contains(msg, want) {
			t.Errorf("launch error lacks %q:\n%s", want, msg)
		}
	}
	if strings.Contains(msg, "sched_getscheduler") || strings.Contains(msg, "dbus") {
		t.Errorf("launch error quotes noise instead of the cause:\n%s", msg)
	}

	// Already unsandboxed: the sandbox advice would be wrong.
	if msg := launchError(errBrowserExited, stderr, true).Error(); strings.Contains(msg, "LIVE_BROWSER_NO_SANDBOX") {
		t.Errorf("launch error suggests disabling a sandbox that is already off:\n%s", msg)
	}
}

func TestBrowserRefusesWorkAfterClose(t *testing.T) {
	t.Parallel() // each test drives its own Chromium
	b := chromiumForTest(t, time.Minute, 1)
	watchPage(t, b, popupPage(t).URL+"/watch")
	proc := b.running()

	b.Close()
	if !proc.dead() {
		t.Error("Close left Chromium running")
	}
	if _, _, err := b.acquire(context.Background()); !errors.Is(err, errBrowserClosed) {
		t.Errorf("acquire after Close = %v, want errBrowserClosed", err)
	}
}
