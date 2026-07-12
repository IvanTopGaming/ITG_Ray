package handlers

import (
	"context"
	"encoding/json"

	"github.com/itg-team/itg-ray/internal/bridge/protocol"
)

// Logs is the surface LogsHandlers needs.
type Logs interface {
	Start() (protocol.LogsStartResult, error)
	Stop() error
	OpenFolder() error
	DirInfo() (protocol.LogsDirInfoResult, error)
}

// LogsHandlers groups methods under the "logs." namespace.
type LogsHandlers struct {
	Svc Logs
}

// Start subscribes the renderer to the live log stream and returns the backlog.
func (h LogsHandlers) Start(_ context.Context, _ json.RawMessage) (any, error) {
	if h.Svc == nil {
		return protocol.LogsStartResult{}, nil
	}
	res, err := h.Svc.Start()
	if err != nil {
		return nil, err
	}
	return res, nil
}

// Stop unsubscribes the renderer; the poller stops on the last subscriber.
func (h LogsHandlers) Stop(_ context.Context, _ json.RawMessage) (any, error) {
	if h.Svc == nil {
		return struct{}{}, nil
	}
	if err := h.Svc.Stop(); err != nil {
		return nil, err
	}
	return struct{}{}, nil
}

// OpenFolder reveals the log directory in the OS file manager.
func (h LogsHandlers) OpenFolder(_ context.Context, _ json.RawMessage) (any, error) {
	if h.Svc == nil {
		return struct{}{}, nil
	}
	if err := h.Svc.OpenFolder(); err != nil {
		return nil, err
	}
	return struct{}{}, nil
}

// DirInfo returns the log directory path and total size of its *.log files.
func (h LogsHandlers) DirInfo(_ context.Context, _ json.RawMessage) (any, error) {
	if h.Svc == nil {
		return protocol.LogsDirInfoResult{}, nil
	}
	res, err := h.Svc.DirInfo()
	if err != nil {
		return nil, err
	}
	return res, nil
}
