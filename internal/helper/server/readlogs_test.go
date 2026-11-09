package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/itg-team/itg-ray/internal/helper/protocol"
)

func TestReadLogsOffsetAndTruncation(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ITGRAY_RUNTIME_BASE", dir)
	path := filepath.Join(dir, "sing-box.log")
	if err := os.WriteFile(path, []byte("line-a\nline-b\n"), 0o640); err != nil {
		t.Fatal(err)
	}

	h := NewReadLogsHandler()
	call := func(off int64) protocol.ReadLogsResult {
		args, _ := json.Marshal(protocol.ReadLogsArgs{Name: "sing-box.log", Offset: off})
		raw, err := h(context.Background(), args)
		if err != nil {
			t.Fatalf("handler err: %v", err)
		}
		var r protocol.ReadLogsResult
		if err := json.Unmarshal(raw, &r); err != nil {
			t.Fatal(err)
		}
		return r
	}

	first := call(0)
	if string(first.Data) != "line-a\nline-b\n" || first.Offset != 14 {
		t.Fatalf("first read bad: %q off=%d", first.Data, first.Offset)
	}
	empty := call(first.Offset)
	if len(empty.Data) != 0 {
		t.Fatalf("expected no new data, got %q", empty.Data)
	}
	// Simulate rotation: file shrinks below the offset.
	if err := os.WriteFile(path, []byte("new\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	trunc := call(first.Offset)
	if !trunc.Truncated || string(trunc.Data) != "new\n" {
		t.Fatalf("truncation not handled: %+v", trunc)
	}
}

func TestReadLogsRejectsUnknownName(t *testing.T) {
	h := NewReadLogsHandler()
	args, _ := json.Marshal(protocol.ReadLogsArgs{Name: "../secret", Offset: 0})
	if _, err := h(context.Background(), args); err == nil {
		t.Fatal("expected rejection of non-allowlisted name")
	}
}

func TestReadLogsChunksLargeFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ITGRAY_RUNTIME_BASE", dir)
	prev := maxLogChunk
	maxLogChunk = 8
	t.Cleanup(func() { maxLogChunk = prev })

	path := filepath.Join(dir, "sing-box.log")
	if err := os.WriteFile(path, []byte("0123456789ABCDE\n"), 0o640); err != nil {
		t.Fatal(err)
	}

	h := NewReadLogsHandler()
	call := func(off int64) protocol.ReadLogsResult {
		args, _ := json.Marshal(protocol.ReadLogsArgs{Name: "sing-box.log", Offset: off})
		raw, err := h(context.Background(), args)
		if err != nil {
			t.Fatalf("handler err: %v", err)
		}
		var r protocol.ReadLogsResult
		if err := json.Unmarshal(raw, &r); err != nil {
			t.Fatal(err)
		}
		return r
	}

	first := call(0)
	if len(first.Data) != 8 || first.Offset != 8 {
		t.Fatalf("expected 8-byte chunk at off 8, got %d bytes off=%d", len(first.Data), first.Offset)
	}
	second := call(first.Offset)
	if len(second.Data) != 8 || second.Offset != 16 {
		t.Fatalf("expected next 8-byte chunk at off 16, got %d bytes off=%d", len(second.Data), second.Offset)
	}
}

func TestReadLogsTailStartsNearEnd(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ITGRAY_RUNTIME_BASE", dir)
	path := filepath.Join(dir, "xray.log")
	if err := os.WriteFile(path, []byte("old-1\nold-2\nnew-3\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	h := NewReadLogsHandler()
	read := func(a protocol.ReadLogsArgs) protocol.ReadLogsResult {
		args, _ := json.Marshal(a)
		raw, err := h(context.Background(), args)
		if err != nil {
			t.Fatalf("handler err: %v", err)
		}
		var r protocol.ReadLogsResult
		if err := json.Unmarshal(raw, &r); err != nil {
			t.Fatal(err)
		}
		return r
	}

	tail := read(protocol.ReadLogsArgs{Name: "xray.log", Tail: 8})
	if string(tail.Data) != "2\nnew-3\n" || tail.Offset != 18 {
		t.Fatalf("tail read bad: %q off=%d", tail.Data, tail.Offset)
	}
	whole := read(protocol.ReadLogsArgs{Name: "xray.log", Tail: 100})
	if string(whole.Data) != "old-1\nold-2\nnew-3\n" {
		t.Fatalf("tail larger than file must read from start: %q", whole.Data)
	}
	resumed := read(protocol.ReadLogsArgs{Name: "xray.log", Offset: 6, Tail: 8})
	if string(resumed.Data) != "old-2\nnew-3\n" {
		t.Fatalf("tail must not apply once an offset is set: %q", resumed.Data)
	}
}
