//go:build windows

package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/itg-team/itg-ray/internal/helper/client"
	"github.com/itg-team/itg-ray/internal/helper/protocol"
	helperserver "github.com/itg-team/itg-ray/internal/helper/server"
	"github.com/itg-team/itg-ray/internal/server"
)

const helperPipe = `\\.\pipe\ITGRay.Helper.v1`

// helperSession holds the live helper connection plus the session id
// that OpStartChain returned, so the matching OpStopChain can address
// the same session.
type helperSession struct {
	c         *client.Client
	sessionID string
}

func (s *helperSession) cleanup(ctx context.Context) {
	if s == nil || s.c == nil {
		return
	}
	args, _ := json.Marshal(helperserver.StopChainArgs{SessionID: s.sessionID})
	_, _ = s.c.Call(ctx, protocol.OpStopChain, args)
	_ = s.c.Close()
}

// startHelperSession dials the helper, sends OpStartChain with the
// pre-built sing-box and xray configs, and returns a session whose
// cleanup() closes everything down on shutdown.
func startHelperSession(
	ctx context.Context,
	srv *server.Server,
	sbCfg, xrCfg []byte,
	tunName string,
) (*helperSession, error) {
	c, err := client.Dial(ctx, helperPipe)
	if err != nil {
		return nil, fmt.Errorf("dial helper pipe: %w", err)
	}
	args, err := json.Marshal(helperserver.StartChainArgs{
		SingboxConfig: sbCfg,
		XrayConfig:    xrCfg,
		ServerHost:    srv.Vless.Address,
		ServerPort:    int(srv.Vless.Port),
		TunName:       tunName,
		Mode:          "tun",
	})
	if err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("marshal StartChain args: %w", err)
	}
	raw, err := c.Call(ctx, protocol.OpStartChain, args)
	if err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("OpStartChain: %w", err)
	}
	var res helperserver.StartChainResult
	if err := json.Unmarshal(raw, &res); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("decode StartChain result: %w", err)
	}
	return &helperSession{c: c, sessionID: res.SessionID}, nil
}
