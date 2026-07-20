package chainctl

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/itg-team/itg-ray/internal/hub"
)

// TestController_StartStop_Race_NoOrphanChain pins backend-review Finding 1
// (CRITICAL): racing StartChain against StopChain must never leave a live,
// untracked chain running while the Controller reports idle.
//
// Before the fix, Stop()'s captured cancel only ever canceled the poller's
// pollCtx — a context bringUp never looked at — so a Stop() that lands while
// bringUp is still in flight has zero effect on it. bringUp keeps running,
// eventually calls fh.StartChain successfully, publishes "connected", and
// starts a poller on an already-canceled pollCtx that exits on its first
// tick. Net effect: fh.running == true forever, but Controller.Status()
// reports idle because Stop() already cleared c.cancel/c.current — exactly
// the orphan-chain scenario the reviewer reproduced 5/5 with a throwaway
// test (StartChain delayed 150ms, Stop() fired 30ms later).
//
// fakeHelper.startChainDelay widens that window deterministically instead
// of relying on scheduler luck, and deliberately does not honor ctx
// cancellation — the fix must hold even when the helper client itself
// doesn't abort in-flight RPCs on cancel, exactly as the real StartChain
// RPC (a synchronous helper call) may not.
func TestController_StartStop_Race_NoOrphanChain(t *testing.T) {
	const (
		iterations      = 10
		startChainDelay = 40 * time.Millisecond
		stopHeadStart   = 8 * time.Millisecond
		watchWindow     = 250 * time.Millisecond
		watchStep       = 5 * time.Millisecond
	)
	for i := 0; i < iterations; i++ {
		c, fh, h, _ := setup(t)
		fh.mu.Lock()
		fh.startChainDelay = startChainDelay
		fh.mu.Unlock()
		rcv := h.Subscribe(64)

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = c.Start(context.Background(), "a", ModeTUN)
		}()
		go func() {
			defer wg.Done()
			// Give Start a head start so it has locked in c.cancel and is
			// blocked inside the delayed StartChain call before Stop
			// lands — otherwise Stop would just see "already idle" and
			// this iteration wouldn't exercise the race at all.
			time.Sleep(stopHeadStart)
			_ = c.Stop(context.Background())
		}()
		wg.Wait()

		// Both calls returning does not by itself mean the race has
		// settled: pre-fix, Start/Stop both return near-instantly without
		// waiting for the in-flight bringUp goroutine, so the orphan only
		// becomes observable once the delayed StartChain finally
		// completes, some time later. Watch across that window rather
		// than sampling once.
		deadline := time.Now().Add(watchWindow)
		for time.Now().Before(deadline) {
			st, _, _ := c.Status()
			fh.mu.Lock()
			running := fh.running
			calls := append([]string(nil), fh.calls...)
			fh.mu.Unlock()

			if st == hub.StatusIdle && running {
				h.Unsubscribe(rcv)
				t.Fatalf("iteration %d: orphan chain — Controller reports idle but "+
					"fakeHelper is still running (calls=%v)", i, calls)
			}
			time.Sleep(watchStep)
		}

		h.Unsubscribe(rcv)
	}
}
