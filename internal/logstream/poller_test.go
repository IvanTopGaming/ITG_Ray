package logstream

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/itg-team/itg-ray/internal/helper/protocol"
	"github.com/itg-team/itg-ray/internal/hub"
)

type fakeReader struct {
	payloads map[string][]byte
	chunk    int
}

func (f fakeReader) Call(_ context.Context, _ protocol.Op, raw json.RawMessage) (json.RawMessage, error) {
	var a protocol.ReadLogsArgs
	_ = json.Unmarshal(raw, &a)
	data := f.payloads[a.Name]
	off := a.Offset
	if a.Tail > 0 && off == 0 && int64(len(data)) > a.Tail {
		off = int64(len(data)) - a.Tail
	}
	if off >= int64(len(data)) {
		data = nil
	} else {
		data = data[off:]
	}
	if f.chunk > 0 && len(data) > f.chunk {
		data = data[:f.chunk]
	}
	return json.Marshal(protocol.ReadLogsResult{Data: data, Offset: off + int64(len(data))})
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

func TestPollerPrimesFromTailAndDropsPartialLine(t *testing.T) {
	var big []byte
	for len(big) < primeTailBytes*3 {
		big = append(big, []byte("2026 INFO old history line that should be skipped\n")...)
	}
	big = append(big, []byte("2026 WARN newest line\n")...)
	buf := New(hub.New(), 100000)
	p := NewPoller(buf, fakeReader{payloads: map[string][]byte{"sing-box.log": big}})
	p.pollOnce(context.Background())

	snap := buf.Snapshot()
	if len(snap) == 0 || snap[len(snap)-1].Message != "2026 WARN newest line" {
		t.Fatalf("newest line missing after prime: %d entries", len(snap))
	}
	total := 0
	for _, e := range snap {
		total += len(e.Message) + 1
		if e.Message != "2026 WARN newest line" && e.Message != "2026 INFO old history line that should be skipped" {
			t.Fatalf("partial first line leaked into the buffer: %q", e.Message)
		}
	}
	if total > primeTailBytes {
		t.Fatalf("prime read %d bytes of history, want at most %d", total, primeTailBytes)
	}
}

func TestPollerDrainsBacklogWithinOnePoll(t *testing.T) {
	data := []byte("2026 INFO a\n2026 INFO b\n2026 INFO c\n2026 INFO d\n")
	buf := New(hub.New(), 100)
	p := NewPoller(buf, fakeReader{payloads: map[string][]byte{"xray.log": data}, chunk: 7})
	p.pollOnce(context.Background())
	if got := len(buf.Snapshot()); got != 4 {
		t.Fatalf("want all 4 lines drained across chunked reads in one poll, got %d", got)
	}
}
