package core

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"

	sbinclude "github.com/sagernet/sing-box/include"
	sbopt "github.com/sagernet/sing-box/option"
	xserial "github.com/xtls/xray-core/infra/conf/serial"

	"github.com/itg-team/itg-ray/internal/logging"
)

// Manager coordinates the lifecycle of the embedded sing-box and xray-core instances.
type Manager struct {
	sbx *SingboxAdapter
	xry *XrayAdapter
}

// NewManager returns an unstarted Manager with fresh adapters.
func NewManager() *Manager {
	return &Manager{sbx: NewSingboxAdapter(), xry: NewXrayAdapter()}
}

// DryValidate parses both JSON config blobs through the libraries' loaders
// without actually starting any networking. Used by tests and pre-flight checks.
func (m *Manager) DryValidate(ctx context.Context, sbJSON, xrJSON []byte) error {
	// UnmarshalJSONContext requires sing-box type registries in the context.
	sbCtx := sbinclude.Context(ctx)
	var sbOpts sbopt.Options
	if err := sbOpts.UnmarshalJSONContext(sbCtx, sbJSON); err != nil {
		slog.Error("config validation failed", slog.String("scope", "core"),
			slog.String("engine", "singbox"), slog.String("err", logging.RedactError(err)))
		return fmt.Errorf("singbox: %w", err)
	}
	if _, err := xserial.LoadJSONConfig(bytes.NewReader(xrJSON)); err != nil {
		slog.Error("config validation failed", slog.String("scope", "core"),
			slog.String("engine", "xray"), slog.String("err", logging.RedactError(err)))
		return fmt.Errorf("xray: %w", err)
	}
	return nil
}

// Start launches xray first (so its SOCKS5 listener is up), then sing-box.
// On any error, both are torn down.
func (m *Manager) Start(ctx context.Context, sbJSON, xrJSON []byte) error {
	if err := m.xry.Start(ctx, xrJSON); err != nil {
		return err
	}
	if err := m.sbx.Start(ctx, sbJSON); err != nil {
		_ = m.xry.Close()
		return err
	}
	slog.Info("core started", slog.String("scope", "core"))
	return nil
}

// Stop closes both instances; the first error encountered is returned but the
// other instance is still attempted.
func (m *Manager) Stop() error {
	errSB := m.sbx.Close()
	errX := m.xry.Close()
	if errSB != nil {
		return errSB
	}
	if errX != nil {
		return errX
	}
	slog.Info("core stopped", slog.String("scope", "core"))
	return nil
}
