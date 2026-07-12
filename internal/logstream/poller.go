package logstream

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/itg-team/itg-ray/internal/helper/protocol"
)

type LogReader interface {
	Call(ctx context.Context, op protocol.Op, args json.RawMessage) (json.RawMessage, error)
}

type logFile struct {
	name   string
	source string
	offset int64
	rest   string
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
		args, _ := json.Marshal(protocol.ReadLogsArgs{Name: lf.name, Offset: lf.offset})
		raw, err := p.reader.Call(ctx, protocol.OpReadLogs, args)
		if err != nil {
			continue
		}
		var res protocol.ReadLogsResult
		if json.Unmarshal(raw, &res) != nil {
			continue
		}
		if res.Truncated {
			lf.rest = ""
		}
		lf.offset = res.Offset
		lf.rest += string(res.Data)
		for {
			nl := strings.IndexByte(lf.rest, '\n')
			if nl < 0 {
				break
			}
			line := strings.TrimRight(lf.rest[:nl], "\r")
			lf.rest = lf.rest[nl+1:]
			if line == "" {
				continue
			}
			p.buf.Add(lf.source, ParseLevel(lf.source, line), line, time.Now())
		}
	}
}
