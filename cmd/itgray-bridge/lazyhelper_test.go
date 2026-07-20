package main

import (
	"context"
	"errors"
	"testing"

	"github.com/itg-team/itg-ray/internal/chainctl"
	"github.com/stretchr/testify/require"
)

// fakeHelper is a minimal chainctl.HelperClient for exercising the lazy
// wrapper. failOps, when set, makes every op return an error (simulating a
// dead pipe / helper-side failure).
type fakeHelper struct {
	calls   []string
	failOps bool
}

func (f *fakeHelper) rec(op string) error {
	f.calls = append(f.calls, op)
	if f.failOps {
		return errors.New("pipe broken")
	}
	return nil
}

func (f *fakeHelper) StartChain(_ context.Context, _, _ []byte, _ chainctl.Mode) error {
	return f.rec("StartChain")
}
func (f *fakeHelper) StopChain(_ context.Context) error              { return f.rec("StopChain") }
func (f *fakeHelper) TunCreate(_ context.Context, _, _ string) error { return f.rec("TunCreate") }
func (f *fakeHelper) TunDestroy(_ context.Context) error             { return f.rec("TunDestroy") }
func (f *fakeHelper) RouteSnapshot(_ context.Context) error          { return f.rec("RouteSnapshot") }
func (f *fakeHelper) RouteAdd(_ context.Context, _ string) error     { return f.rec("RouteAdd") }
func (f *fakeHelper) RouteRestore(_ context.Context) error           { return f.rec("RouteRestore") }
func (f *fakeHelper) DnsSet(_ context.Context, _ []string) error     { return f.rec("DnsSet") }
func (f *fakeHelper) DnsRestore(_ context.Context) error             { return f.rec("DnsRestore") }
func (f *fakeHelper) ServiceStatus(_ context.Context) (chainctl.ChainState, error) {
	if err := f.rec("ServiceStatus"); err != nil {
		return chainctl.ChainState{}, err
	}
	return chainctl.ChainState{Running: true}, nil
}

func TestLazyHelper_DialsOnDemandAfterInstall(t *testing.T) {
	installed := false
	dials := 0
	fake := &fakeHelper{}
	l := newLazyHelperClient(func(_ context.Context) (chainctl.HelperClient, error) {
		dials++
		if !installed {
			return nil, errors.New("The system cannot find the file specified.")
		}
		return fake, nil
	})
	ctx := context.Background()

	// Pre-install: a required op surfaces "helper unavailable" (no restart-
	// stuck stub) and does NOT cache the failure.
	err := l.RouteSnapshot(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "helper unavailable")

	// User installs the helper; the very next call must redial and succeed.
	installed = true
	require.NoError(t, l.RouteSnapshot(ctx))
	require.Equal(t, []string{"RouteSnapshot"}, fake.calls)
	require.Equal(t, 2, dials, "dialed once (failed) pre-install, once (ok) post-install")
}

func TestLazyHelper_TeardownIsNoOpWhenUnavailable(t *testing.T) {
	l := newLazyHelperClient(func(_ context.Context) (chainctl.HelperClient, error) {
		return nil, errors.New("unavailable")
	})
	ctx := context.Background()
	// Rollback/teardown ops must not hard-fail when the helper is unreachable.
	require.NoError(t, l.StopChain(ctx))
	require.NoError(t, l.TunDestroy(ctx))
	require.NoError(t, l.RouteRestore(ctx))
	require.NoError(t, l.DnsRestore(ctx))
	// A required op still errors.
	require.Error(t, l.StartChain(ctx, nil, nil, chainctl.ModeTUN))
}

func TestLazyHelper_ReusesConnectionThenRedialsOnError(t *testing.T) {
	dials := 0
	fake := &fakeHelper{}
	l := newLazyHelperClient(func(_ context.Context) (chainctl.HelperClient, error) {
		dials++
		return fake, nil
	})
	ctx := context.Background()

	require.NoError(t, l.RouteSnapshot(ctx))
	require.NoError(t, l.RouteAdd(ctx, "h"))
	require.Equal(t, 1, dials, "successful calls reuse the one connection")

	// Pipe dies (helper restart/reinstall): the op errors and the cached
	// connection is dropped so the next call redials.
	fake.failOps = true
	require.Error(t, l.RouteSnapshot(ctx))
	fake.failOps = false
	require.NoError(t, l.RouteSnapshot(ctx))
	require.Equal(t, 2, dials, "redialed after the connection error")
}
