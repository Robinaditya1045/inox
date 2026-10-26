package live

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"
)

// TestMain starts Chromium once, on its own, before any test runs. It covers the
// external live_test package too, since both compile into one test binary.
//
// On a fresh CI runner the first Chromium launch is slow: the binary and its
// libraries are not in the page cache yet, and `go test -race ./...` is busy
// running every other package alongside this one. The first two browser tests to
// take a slot (see AcquireBrowserSlot) paid that cost inside launchChromium's 30s
// startup handshake, which they sometimes overran ("headless browser failed to
// start: context deadline exceeded") while every later launch took a few seconds.
// Paying it here, with no handshake and a generous deadline, keeps it out of the
// tests' budgets without loosening the production timeout.
func TestMain(m *testing.M) {
	warmUpChromium()
	os.Exit(m.Run())
}

func warmUpChromium() {
	path := os.Getenv("LIVE_BROWSER_PATH")
	if path == "" {
		found, ok := FindChromium()
		if !ok {
			return // the browser tests skip themselves
		}
		path = found
	}
	profile, err := os.MkdirTemp("", "inox-chromium-warmup-")
	if err != nil {
		return
	}
	defer os.RemoveAll(profile)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	start := time.Now()
	cmd := exec.CommandContext(ctx, path,
		"--headless", "--no-sandbox", "--disable-gpu", "--no-first-run",
		"--user-data-dir="+profile, "--dump-dom", "about:blank")
	cmd.Env = chromiumEnv(profile)
	// A failure here is only lost warm-up: the tests launch their own browsers and
	// report anything actually wrong with them.
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "chromium warm-up failed after %s: %v\n%s\n", time.Since(start).Round(time.Millisecond), err, tail(out, 2000))
		return
	}
	fmt.Fprintf(os.Stderr, "chromium warm-up took %s\n", time.Since(start).Round(time.Millisecond))
}

func tail(b []byte, n int) []byte {
	if len(b) > n {
		return b[len(b)-n:]
	}
	return b
}
