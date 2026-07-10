//go:build !windows

package chainctl

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/itg-team/itg-ray/internal/configgen"
	"github.com/itg-team/itg-ray/internal/core"
	"github.com/itg-team/itg-ray/internal/helper/xrayapi"
)

var errTunUnsupported = errors.New("TUN mode is not yet available on Linux (coming in Phase B); use SysProxy mode")

type coreRunner interface {
	Start(ctx context.Context, singboxJSON, xrayJSON []byte) error
	Stop() error
}

type counterSource interface {
	Counters(ctx context.Context) (up, down uint64, err error)
	Close() error
}

type coreHelperClient struct {
	newRunner func() coreRunner
	newStats  func() counterSource

	mu      sync.Mutex
	runner  coreRunner
	stats   counterSource
	running bool
}

func NewCoreHelperClient() HelperClient {
	return &coreHelperClient{
		newRunner: func() coreRunner { return core.NewManager() },
		newStats: func() counterSource {
			return xrayapi.New(fmt.Sprintf("127.0.0.1:%d", configgen.XrayAPIPort))
		},
	}
}

func (h *coreHelperClient) StartChain(ctx context.Context, singboxJSON, xrayJSON []byte, mode Mode) error {
	if mode == ModeTUN {
		return errTunUnsupported
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.running {
		return fmt.Errorf("chainctl: chain already running")
	}
	runner := h.newRunner()
	if err := runner.Start(ctx, singboxJSON, xrayJSON); err != nil {
		return fmt.Errorf("core start: %w", err)
	}
	h.runner = runner
	h.stats = h.newStats()
	h.running = true
	return nil
}

func (h *coreHelperClient) StopChain(_ context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.running {
		return nil
	}
	if h.stats != nil {
		_ = h.stats.Close()
		h.stats = nil
	}
	var err error
	if h.runner != nil {
		err = h.runner.Stop()
		h.runner = nil
	}
	h.running = false
	return err
}

func (h *coreHelperClient) ServiceStatus(ctx context.Context) (ChainState, error) {
	h.mu.Lock()
	running := h.running
	stats := h.stats
	h.mu.Unlock()

	st := ChainState{Running: running}
	if running && stats != nil {
		if up, down, err := stats.Counters(ctx); err == nil {
			st.UpBytes = up
			st.DownBytes = down
		}
	}
	return st, nil
}

func (h *coreHelperClient) TunCreate(context.Context, string, string) error { return errTunUnsupported }
func (h *coreHelperClient) TunDestroy(context.Context) error                { return nil }
func (h *coreHelperClient) RouteSnapshot(context.Context) error             { return errTunUnsupported }
func (h *coreHelperClient) RouteAdd(context.Context, string) error          { return errTunUnsupported }
func (h *coreHelperClient) RouteRestore(context.Context) error              { return nil }
func (h *coreHelperClient) DnsSet(context.Context, []string) error          { return errTunUnsupported }
func (h *coreHelperClient) DnsRestore(context.Context) error                { return nil }
