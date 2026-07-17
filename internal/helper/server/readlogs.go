package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/itg-team/itg-ray/internal/helper/protocol"
	"github.com/itg-team/itg-ray/internal/logging"
)

var readLogsAllow = map[string]bool{"sing-box.log": true, "xray.log": true, "helper.log": true}

// maxLogChunk caps the bytes returned by a single ReadLogs call so the
// JSON-encoded (base64) response stays well under protocol.MaxFrame (1 MiB).
// The poller advances Offset and drains the remainder over subsequent polls.
var maxLogChunk = 512 * 1024

// RuntimeDir returns the directory where the privileged helper persists its
// session log files (sing-box.log/xray.log/helper.log). It resolves to the
// same per-platform location the ReadLogs handler serves from.
func RuntimeDir() string { return engineLogDir() }

func NewReadLogsHandler() Handler {
	return func(_ context.Context, raw json.RawMessage) (json.RawMessage, error) {
		var args protocol.ReadLogsArgs
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, err
		}
		if !readLogsAllow[args.Name] {
			slog.Warn("read logs rejected: name not allowed",
				slog.String("scope", "helper"), slog.String("name", args.Name))
			return nil, fmt.Errorf("read logs: name %q not allowed", args.Name)
		}
		f, err := os.Open(filepath.Join(engineLogDir(), args.Name))
		if err != nil {
			if os.IsNotExist(err) {
				return json.Marshal(protocol.ReadLogsResult{Offset: 0})
			}
			slog.Error("read logs failed", slog.String("scope", "helper"),
				slog.String("stage", "open"), slog.String("name", args.Name), slog.String("err", logging.RedactError(err)))
			return nil, err
		}
		defer f.Close()

		info, err := f.Stat()
		if err != nil {
			slog.Error("read logs failed", slog.String("scope", "helper"),
				slog.String("stage", "stat"), slog.String("name", args.Name), slog.String("err", logging.RedactError(err)))
			return nil, err
		}
		offset, truncated := args.Offset, false
		if info.Size() < offset {
			offset, truncated = 0, true
		}
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			slog.Error("read logs failed", slog.String("scope", "helper"),
				slog.String("stage", "seek"), slog.String("name", args.Name), slog.String("err", logging.RedactError(err)))
			return nil, err
		}
		data, err := io.ReadAll(io.LimitReader(f, int64(maxLogChunk)))
		if err != nil {
			slog.Error("read logs failed", slog.String("scope", "helper"),
				slog.String("stage", "read"), slog.String("name", args.Name), slog.String("err", logging.RedactError(err)))
			return nil, err
		}
		slog.Debug("read logs ok", slog.String("scope", "helper"),
			slog.String("name", args.Name), slog.Int("bytes", len(data)), slog.Bool("truncated", truncated))
		return json.Marshal(protocol.ReadLogsResult{
			Data:      data,
			Offset:    offset + int64(len(data)),
			Truncated: truncated,
		})
	}
}
