package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/itg-team/itg-ray/internal/helper/protocol"
)

var readLogsAllow = map[string]bool{"sing-box.log": true, "xray.log": true}

func NewReadLogsHandler() Handler {
	return func(_ context.Context, raw json.RawMessage) (json.RawMessage, error) {
		var args protocol.ReadLogsArgs
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, err
		}
		if !readLogsAllow[args.Name] {
			return nil, fmt.Errorf("read logs: name %q not allowed", args.Name)
		}
		f, err := os.Open(filepath.Join(engineLogDir(), args.Name))
		if err != nil {
			if os.IsNotExist(err) {
				return json.Marshal(protocol.ReadLogsResult{Offset: 0})
			}
			return nil, err
		}
		defer f.Close()

		info, err := f.Stat()
		if err != nil {
			return nil, err
		}
		offset, truncated := args.Offset, false
		if info.Size() < offset {
			offset, truncated = 0, true
		}
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			return nil, err
		}
		data, err := io.ReadAll(f)
		if err != nil {
			return nil, err
		}
		return json.Marshal(protocol.ReadLogsResult{
			Data:      data,
			Offset:    offset + int64(len(data)),
			Truncated: truncated,
		})
	}
}
