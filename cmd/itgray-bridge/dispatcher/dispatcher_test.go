package dispatcher

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
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

func TestDispatcher_ObserverInvoked(t *testing.T) {
	d := New()
	d.Register("ping", func(_ context.Context, _ json.RawMessage) (any, error) {
		return "pong", nil
	})
	d.Register("boom", func(_ context.Context, _ json.RawMessage) (any, error) {
		return nil, errors.New("kaboom")
	})
	var gotMethod string
	var gotErr error
	var calls int
	d.Observer = func(method string, _ json.RawMessage, err error, _ time.Duration) {
		calls++
		gotMethod = method
		gotErr = err
	}
	in := strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n" +
			`{"jsonrpc":"2.0","id":2,"method":"boom"}` + "\n")
	var out strings.Builder
	require.NoError(t, d.Serve(context.Background(), in, &out))
	require.Equal(t, 2, calls)
	require.Equal(t, "boom", gotMethod)
	require.Error(t, gotErr)
}
