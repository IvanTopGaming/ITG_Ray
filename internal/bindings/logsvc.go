package bindings

import (
	"os"
	"strings"
	"time"

	"github.com/itg-team/itg-ray/internal/bridge/protocol"
	"github.com/itg-team/itg-ray/internal/logstream"
	"github.com/itg-team/itg-ray/internal/sysopen"
)

// startTailLimit is how much history Start hands back when the Logs tab
// opens, and matches the number of lines the renderer retains: the tab always
// shows the newest 2000 lines across sources, then follows live log:batch
// events. The full history is still reachable through logs.export.
const startTailLimit = 2000

// LogDeps wires the Logs service. The engine-log poller is owned by the
// bridge and runs for its whole lifetime, so Start/Stop only gate whether new
// lines are pushed to the renderer.
type LogDeps struct {
	Buffer *logstream.Buffer
	LogDir string
}

type LogService struct{ d LogDeps }

func NewLogService(d LogDeps) *LogService { return &LogService{d: d} }

func (s *LogService) Start() (protocol.LogsStartResult, error) {
	s.d.Buffer.Subscribe()
	var out []protocol.LogsEntry
	for _, e := range s.d.Buffer.Tail(startTailLimit) {
		out = append(out, protocol.LogsEntry{
			Seq:     e.Seq,
			Time:    e.Time.Format(time.RFC3339Nano),
			Level:   e.Level,
			Source:  e.Source,
			Message: e.Message,
		})
	}
	return protocol.LogsStartResult{Entries: out}, nil
}

func (s *LogService) Stop() error {
	s.d.Buffer.Unsubscribe()
	return nil
}

func (s *LogService) OpenFolder() error {
	return sysopen.Dir(s.d.LogDir)
}

func (s *LogService) DirInfo() (protocol.LogsDirInfoResult, error) {
	var total int64
	entries, err := os.ReadDir(s.d.LogDir)
	if err == nil {
		for _, e := range entries {
			if !strings.Contains(e.Name(), ".log") {
				continue
			}
			if info, ierr := e.Info(); ierr == nil {
				total += info.Size()
			}
		}
	}
	return protocol.LogsDirInfoResult{Path: s.d.LogDir, SizeBytes: total}, nil
}
