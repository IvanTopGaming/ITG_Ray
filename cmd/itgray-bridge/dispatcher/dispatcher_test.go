package dispatcher

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
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
