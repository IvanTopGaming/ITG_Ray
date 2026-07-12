package bindings

import (
	"os"
	"strings"
	"time"

	"github.com/itg-team/itg-ray/internal/bridge/protocol"
	"github.com/itg-team/itg-ray/internal/logstream"
	"github.com/itg-team/itg-ray/internal/sysopen"
)

type LogDeps struct {
	Buffer      *logstream.Buffer
	StartPoller func()
	StopPoller  func()
	LogDir      string
}

type LogService struct{ d LogDeps }

func NewLogService(d LogDeps) *LogService { return &LogService{d: d} }

func (s *LogService) Start() (protocol.LogsStartResult, error) {
	if n := s.d.Buffer.Subscribe(); n == 1 && s.d.StartPoller != nil {
		s.d.StartPoller()
	}
	var out []protocol.LogsEntry
	for _, e := range s.d.Buffer.Snapshot() {
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
	if n := s.d.Buffer.Unsubscribe(); n == 0 && s.d.StopPoller != nil {
		s.d.StopPoller()
	}
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
