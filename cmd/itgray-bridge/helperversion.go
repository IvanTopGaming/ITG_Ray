package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/itg-team/itg-ray/internal/helper/protocol"
	helperserver "github.com/itg-team/itg-ray/internal/helper/server"
	"github.com/itg-team/itg-ray/internal/logstream"
)

func helperVersionProbe(c logstream.LogReader) func(context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		raw, err := c.Call(ctx, protocol.OpServiceStatus, nil)
		if err != nil {
			return "", err
		}
		var res helperserver.ServiceStatusResult
		if err := json.Unmarshal(raw, &res); err != nil {
			return "", fmt.Errorf("decode ServiceStatus: %w", err)
		}
		return res.Version, nil
	}
}
