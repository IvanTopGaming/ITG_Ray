//go:build !windows

package chainctl

import (
	"context"
	"sync"
	"testing"
)

// TestModeRoutingHelperClient_ConcurrentStartChain_NoRace pins backend-review
// Finding 2 (HIGH): modeRoutingHelperClient.active is read (ServiceStatus,
// StopChain, delegate) and written (StartChain) with no synchronization at
// all. Two overlapping bringUp/tearDown goroutines from Finding 1 call
// exactly this object concurrently in production, so this drives concurrent
// StartChain/StopChain against one modeRoutingHelperClient and expects
// `go test -race` to be clean. Before the fix this reliably reports a DATA
// RACE on the `active` field (concurrent write in StartChain racing the read
// in StopChain/delegate).
func TestModeRoutingHelperClient_ConcurrentStartChain_NoRace(t *testing.T) {
	core := newFake()
	daemon := newFake()
	m := newModeRoutingHelperClient(core, daemon)

	const n = 50
	var wg sync.WaitGroup
	wg.Add(n * 3)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			mode := ModeSysProxy
			if i%2 == 0 {
				mode = ModeTUN
			}
			_ = m.StartChain(context.Background(), []byte(`{}`), []byte(`{}`), mode)
		}(i)
		go func() {
			defer wg.Done()
			_ = m.StopChain(context.Background())
		}()
		go func() {
			defer wg.Done()
			_, _ = m.ServiceStatus(context.Background())
		}()
	}
	wg.Wait()
}
