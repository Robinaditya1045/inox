package live

import (
	"runtime"
	"testing"
)

// browserTestSlots bounds how many tests drive a real Chromium at once, across both
// this package's tests and the external live_test package (they compile into one
// test binary). A headless Chromium wants roughly a core; without this the ~25
// t.Parallel browser tests launch that many at once and starve a CI runner, so
// launches crawl and pages miss the budgets the tests assert against. Sized to the
// machine -- two on a small CI runner, more on a workstation.
var browserTestSlots = make(chan struct{}, max(2, runtime.GOMAXPROCS(0)/2))

// AcquireBrowserSlot blocks until a Chromium slot is free and gives it back when the
// test ends. Exported so the external live_test package shares the one limit. Call it
// before timing a resolve: the wait is not part of what the resolve costs, and the
// browser it gates is closed (its slot cleanup runs after the browser's) before the
// slot is released.
func AcquireBrowserSlot(t *testing.T) {
	t.Helper()
	browserTestSlots <- struct{}{}
	t.Cleanup(func() { <-browserTestSlots })
}
