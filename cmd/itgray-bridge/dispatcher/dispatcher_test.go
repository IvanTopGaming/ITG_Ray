package dispatcher

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDispatcherRoutesRegisteredMethod(t *testing.T) {
	d := New()
	d.Register("echo", func(_ context.Context, params json.RawMessage) (any, error) {
		var s string
		if err := json.Unmarshal(params, &s); err != nil {
			return nil, err
		}
		return s, nil
	})

	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"echo","params":"hi"}` + "\n")
	var out bytes.Buffer
	if err := d.Serve(context.Background(), in, &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	got := strings.TrimSpace(out.String())
	want := `{"jsonrpc":"2.0","id":1,"result":"hi"}`
	if got != want {
		t.Fatalf("response mismatch:\n got=%s\nwant=%s", got, want)
	}
}

func TestDispatcherUnknownMethod(t *testing.T) {
	d := New()
	in := strings.NewReader(`{"jsonrpc":"2.0","id":7,"method":"nope"}` + "\n")
	var out bytes.Buffer
	if err := d.Serve(context.Background(), in, &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	if !strings.Contains(out.String(), `"code":-32601`) {
		t.Fatalf("expected method-not-found, got: %s", out.String())
	}
}

func TestDispatcherMalformedJSON(t *testing.T) {
	d := New()
	in := strings.NewReader("not json\n")
	var out bytes.Buffer
	if err := d.Serve(context.Background(), in, &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	if !strings.Contains(out.String(), `"code":-32700`) {
		t.Fatalf("expected parse-error, got: %s", out.String())
	}
}

// syncBuffer is a concurrency-safe io.Writer standing in for the bridge's
// lockedWriter (cmd/itgray-bridge/main.go), which already serializes writes
// so concurrent handlers can't interleave their response lines.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// A slow handler must not block unrelated requests behind it. subs.syncOne
// holds the handler for a full 30s HTTP fetch; while Serve ran one request at
// a time, a second sync (or any rules.list / servers.list) simply waited for
// it to finish. Both handlers must be in flight simultaneously.
func TestServeRunsHandlersConcurrently(t *testing.T) {
	const n = 2
	d := New()
	entered := make(chan struct{}, n)
	release := make(chan struct{})
	d.Register("slow", func(_ context.Context, _ json.RawMessage) (any, error) {
		entered <- struct{}{}
		<-release
		return "done", nil
	})

	in := strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"slow"}` + "\n" +
			`{"jsonrpc":"2.0","id":2,"method":"slow"}` + "\n")
	out := &syncBuffer{}
	served := make(chan error, 1)
	go func() { served <- d.Serve(context.Background(), in, out) }()

	// Every handler must enter before any of them is allowed to return.
	// Serialized dispatch can only ever get one in, so this times out.
	for i := range n {
		select {
		case <-entered:
		case <-time.After(3 * time.Second):
			t.Fatalf("only %d of %d handlers were in flight: Serve is serializing requests", i, n)
		}
	}
	close(release)

	select {
	case err := <-served:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("Serve did not return after handlers finished")
	}

	// Serve must not return before every in-flight response has been written,
	// otherwise the bridge exits with answers still pending.
	got := out.String()
	require.Contains(t, got, `"id":1`)
	require.Contains(t, got, `"id":2`)
}

func TestDispatcher_ObserverInvoked(t *testing.T) {
	d := New()
	d.Register("ping", func(_ context.Context, _ json.RawMessage) (any, error) {
		return "pong", nil
	})
	d.Register("boom", func(_ context.Context, _ json.RawMessage) (any, error) {
		return nil, errors.New("kaboom")
	})
	// Serve dispatches concurrently, so the Observer is called from handler
	// goroutines in completion order — guard the recorded state and assert per
	// method rather than on whichever call happened to land last.
	var mu sync.Mutex
	errByMethod := map[string]error{}
	d.Observer = func(method string, _ json.RawMessage, err error, _ time.Duration) {
		mu.Lock()
		defer mu.Unlock()
		errByMethod[method] = err
	}
	in := strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n" +
			`{"jsonrpc":"2.0","id":2,"method":"boom"}` + "\n")
	out := &syncBuffer{}
	require.NoError(t, d.Serve(context.Background(), in, out))

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, errByMethod, 2)
	require.NoError(t, errByMethod["ping"])
	require.Error(t, errByMethod["boom"])
}
