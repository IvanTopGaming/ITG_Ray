package bus

import (
	"bytes"
	"strings"
	"sync"
	"testing"
)

func TestBusEmitWritesNotification(t *testing.T) {
	var buf bytes.Buffer
	b := New(&buf)
	b.Emit("vpn.status", map[string]string{"status": "connecting"})

	got := strings.TrimSpace(buf.String())
	want := `{"jsonrpc":"2.0","method":"event:vpn.status","params":{"status":"connecting"}}`
	if got != want {
		t.Fatalf("output mismatch:\n got=%s\nwant=%s", got, want)
	}
}

func TestBusEmitConcurrent(t *testing.T) {
	// Bus must serialize writes — interleaved newlines would break the
	// line-delimited parser on the consumer side.
	var buf bytes.Buffer
	b := New(&buf)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			b.Emit("ping", map[string]int{"i": i})
		}(i)
	}
	wg.Wait()

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 100 {
		t.Fatalf("expected 100 lines, got %d", len(lines))
	}
}
