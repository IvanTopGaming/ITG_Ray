package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/itg-team/itg-ray/internal/helper/protocol"
)

type fakeOpCaller struct {
	op  protocol.Op
	raw string
	err error
}

func (f *fakeOpCaller) Call(_ context.Context, op protocol.Op, _ json.RawMessage) (json.RawMessage, error) {
	f.op = op
	if f.err != nil {
		return nil, f.err
	}
	return json.RawMessage(f.raw), nil
}

func TestHelperVersionProbeReadsServiceStatusVersion(t *testing.T) {
	c := &fakeOpCaller{raw: `{"version":"v0.1.2","uptime_secs":5,"chain_active":true}`}
	v, err := helperVersionProbe(c)(context.Background())
	if err != nil || v != "v0.1.2" {
		t.Fatalf("got %q, %v", v, err)
	}
	if c.op != protocol.OpServiceStatus {
		t.Fatalf("op = %q", c.op)
	}
}

func TestHelperVersionProbePropagatesCallError(t *testing.T) {
	c := &fakeOpCaller{err: errors.New("dial")}
	if _, err := helperVersionProbe(c)(context.Background()); err == nil {
		t.Fatal("want error")
	}
}

func TestHelperVersionProbeRejectsBadPayload(t *testing.T) {
	c := &fakeOpCaller{raw: `not json`}
	if _, err := helperVersionProbe(c)(context.Background()); err == nil {
		t.Fatal("want error")
	}
}
