package logstream

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/itg-team/itg-ray/internal/helper/protocol"
)

type LogReader interface {
	Call(ctx context.Context, op protocol.Op, args json.RawMessage) (json.RawMessage, error)
}

// primeTailBytes is how far back from the end of an engine log the poller
// starts on its first read of that file. A session log can hold hundreds of
// megabytes of debug output; draining it from offset 0 used to flood the
// buffer (and, through it, the renderer) for minutes. 512 KiB comfortably
// covers the buffer's per-source cap of recent lines and matches the helper's
// maxLogChunk, so the prime is a single ReadLogs round trip.
const primeTailBytes = 512 * 1024

// maxReadsPerPoll bounds how many back-to-back ReadLogs calls one poll makes
// for a single file. A file that grows faster than one chunk per tick is
// drained in the same poll instead of falling further behind each second;
// the bound keeps a runaway writer (or an old helper that ignores Tail and
// serves the file from the start) from pinning the poller forever.
const maxReadsPerPoll = 64

type logFile struct {
	name   string
	source string
	offset int64
	rest   string
	warned bool
	// primed flips after the first successful read. Until then reads ask the
	// helper for the tail of the file rather than its beginning.
	primed bool
	// dropPartial discards bytes up to the first newline: a tail read almost
	// always lands mid-line, and that fragment would parse as a garbage entry.
	dropPartial bool
}

type Poller struct {
	buf    *Buffer
	reader LogReader
	files  []*logFile
	cancel context.CancelFunc
	mu     sync.Mutex
}

func NewPoller(buf *Buffer, r LogReader) *Poller {
	return &Poller{
		buf:    buf,
		reader: r,
		files: []*logFile{
			{name: "sing-box.log", source: "sing-box"},
			{name: "xray.log", source: "xray"},
			{name: "helper.log", source: "helper"},
		},
	}
}

func (p *Poller) Start(ctx context.Context) {
	p.mu.Lock()
	if p.cancel != nil {
		p.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(ctx)
	p.cancel = cancel
	p.mu.Unlock()

	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		p.pollOnce(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				p.pollOnce(ctx)
			}
		}
	}()
}

func (p *Poller) Stop() {
	p.mu.Lock()
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
	p.mu.Unlock()
}

func (p *Poller) pollOnce(ctx context.Context) {
	if p.reader == nil {
		return
	}
	for _, lf := range p.files {
		for range maxReadsPerPoll {
			if ctx.Err() != nil {
				return
			}
			if !p.readFile(ctx, lf) {
				break
			}
		}
	}
}

// readFile performs one ReadLogs call for lf and feeds complete lines into the
// buffer. It reports whether the helper returned data, i.e. whether another
// read right away might find more.
func (p *Poller) readFile(ctx context.Context, lf *logFile) bool {
	args := protocol.ReadLogsArgs{Name: lf.name, Offset: lf.offset}
	if !lf.primed {
		args.Tail = primeTailBytes
	}
	raw, err := p.reader.Call(ctx, protocol.OpReadLogs, mustJSON(args))
	if err != nil {
		if !lf.warned {
			lf.warned = true
			slog.Warn("logstream: engine log read failed", "file", lf.name, "err", err.Error())
		}
		return false
	}
	lf.warned = false
	var res protocol.ReadLogsResult
	if json.Unmarshal(raw, &res) != nil {
		return false
	}
	if res.Truncated {
		lf.rest = ""
		lf.dropPartial = false
	}
	if !lf.primed {
		lf.primed = true
		lf.dropPartial = res.Offset-int64(len(res.Data)) > 0
	}
	lf.offset = res.Offset
	lf.rest += string(res.Data)
	if lf.dropPartial {
		nl := strings.IndexByte(lf.rest, '\n')
		if nl < 0 {
			lf.rest = ""
			return len(res.Data) > 0
		}
		lf.rest = lf.rest[nl+1:]
		lf.dropPartial = false
	}
	for {
		nl := strings.IndexByte(lf.rest, '\n')
		if nl < 0 {
			break
		}
		line := stripANSI(strings.TrimRight(lf.rest[:nl], "\r"))
		lf.rest = lf.rest[nl+1:]
		if line == "" {
			continue
		}
		p.buf.Add(lf.source, ParseLevel(lf.source, line), line, time.Now())
	}
	return len(res.Data) > 0
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
