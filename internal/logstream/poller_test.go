package logstream

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/itg-team/itg-ray/internal/helper/protocol"
	"github.com/itg-team/itg-ray/internal/hub"
)

type fakeReader struct{ payloads map[string][]byte }

func (f fakeReader) Call(_ context.Context, _ protocol.Op, raw json.RawMessage) (json.RawMessage, error) {
	var a protocol.ReadLogsArgs
	_ = json.Unmarshal(raw, &a)
	data := f.payloads[a.Name]
	if a.Offset >= int64(len(data)) {
		data = nil
	} else {
		data = data[a.Offset:]
	}
	return json.Marshal(protocol.ReadLogsResult{Data: data, Offset: a.Offset + int64(len(data))})
}

func TestPollerParsesAndTagsSources(t *testing.T) {
	buf := New(hub.New(), 100)
	r := fakeReader{payloads: map[string][]byte{
		"sing-box.log": []byte("2026 WARN dns timeout\n2026 INFO router up\n"),
		"xray.log":     []byte("2026 [Error] reset\n"),
	}}
	p := NewPoller(buf, r)
	p.pollOnce(context.Background())

	snap := buf.Snapshot()
	if len(snap) != 3 {
		t.Fatalf("want 3 parsed lines, got %d", len(snap))
	}
	var sawWarnSingbox, sawErrXray bool
	for _, e := range snap {
		if e.Source == "sing-box" && e.Level == "WARN" {
			sawWarnSingbox = true
		}
		if e.Source == "xray" && e.Level == "ERROR" {
			sawErrXray = true
		}
	}
	if !sawWarnSingbox || !sawErrXray {
		t.Fatalf("source/level tagging wrong: %+v", snap)
	}

	// Second poll with no new bytes should add nothing (offset advanced).
	p.pollOnce(context.Background())
	if len(buf.Snapshot()) != 3 {
		t.Fatalf("offset not advanced; got %d entries", len(buf.Snapshot()))
	}
}

func TestPollerTailsHelperLog(t *testing.T) {
	buf := New(hub.New(), 100)
	r := fakeReader{payloads: map[string][]byte{
		"helper.log": []byte("2026 INFO chain spawned\n2026 ERROR firewall seal failed\n"),
	}}
	p := NewPoller(buf, r)
	p.pollOnce(context.Background())

	var sawHelper bool
	for _, e := range buf.Snapshot() {
		if e.Source == "helper" {
			sawHelper = true
		}
	}
	if !sawHelper {
		t.Fatalf("helper.log lines not tagged source %q: %+v", "helper", buf.Snapshot())
	}
}
