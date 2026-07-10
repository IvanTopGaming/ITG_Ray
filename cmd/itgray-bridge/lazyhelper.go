package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/itg-team/itg-ray/internal/chainctl"
)

// helperDialer opens a fresh helper connection wrapped as a chainctl
// HelperClient. It returns an error when the helper isn't reachable (e.g.
// the service isn't installed/started yet).
type helperDialer func(ctx context.Context) (chainctl.HelperClient, error)

// lazyHelperClient wraps a helperDialer so the helper connection is
// established ON DEMAND and re-established after it breaks — instead of
// being dialed once at process start.
//
// This fixes the first-run flow: the app launches before the helper
// service is installed, so an eager startup dial fails and (previously)
// stuck a permanent "helper unavailable" stub for the whole session,
// forcing an app restart after the user installed the helper. Lazy
// dialing means the very next Connect after install succeeds. It also
// covers the Restart/Reinstall helper actions: when the pipe dies the
// cached connection is dropped and the next call redials.
//
// Method semantics mirror the previous missing-helper stub: connection-
// dependent operations surface a clear "helper unavailable" error when the
// helper can't be reached, while teardown operations (StopChain,
// TunDestroy, RouteRestore, DnsRestore) degrade to no-ops so a rollback
// path never hard-fails just because the helper is already gone.
type lazyHelperClient struct {
	dial helperDialer
	mu   sync.Mutex
	cur  chainctl.HelperClient // cached live delegate; nil when disconnected
}

func newLazyHelperClient(dial helperDialer) *lazyHelperClient {
	return &lazyHelperClient{dial: dial}
}

// get returns a live delegate, dialing if necessary. The dial happens under
// the mutex so concurrent first-callers don't open duplicate connections.
func (l *lazyHelperClient) get(ctx context.Context) (chainctl.HelperClient, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.cur != nil {
		return l.cur, nil
	}
	c, err := l.dial(ctx)
	if err != nil {
		return nil, fmt.Errorf("helper unavailable: %w", err)
	}
	l.cur = c
	return c, nil
}

// invalidate drops the cached delegate so the next call redials. Called when
// a delegate op errors, on the assumption the pipe may have died (helper
// restart/reinstall). The stale connection is intentionally not closed here
// to avoid racing a concurrent in-flight call on it; the OS reclaims it.
func (l *lazyHelperClient) invalidate() {
	l.mu.Lock()
	l.cur = nil
	l.mu.Unlock()
}

// required runs op against a live delegate; a dial failure surfaces the
// error (the operation genuinely can't proceed without the helper).
func (l *lazyHelperClient) required(ctx context.Context, op func(c chainctl.HelperClient) error) error {
	c, err := l.get(ctx)
	if err != nil {
		return err
	}
	if err := op(c); err != nil {
		l.invalidate()
		return err
	}
	return nil
}

// teardown runs op against a live delegate but degrades to a no-op when the
// helper is unreachable, mirroring the old stub so rollback can't hard-fail.
func (l *lazyHelperClient) teardown(ctx context.Context, op func(c chainctl.HelperClient) error) error {
	c, err := l.get(ctx)
	if err != nil {
		return nil
	}
	if err := op(c); err != nil {
		l.invalidate()
		return err
	}
	return nil
}

func (l *lazyHelperClient) StartChain(ctx context.Context, singboxJSON, xrayJSON []byte, mode chainctl.Mode) error {
	return l.required(ctx, func(c chainctl.HelperClient) error {
		return c.StartChain(ctx, singboxJSON, xrayJSON, mode)
	})
}

func (l *lazyHelperClient) StopChain(ctx context.Context) error {
	return l.teardown(ctx, func(c chainctl.HelperClient) error { return c.StopChain(ctx) })
}

func (l *lazyHelperClient) TunCreate(ctx context.Context, name, cidr string) error {
	return l.required(ctx, func(c chainctl.HelperClient) error { return c.TunCreate(ctx, name, cidr) })
}

func (l *lazyHelperClient) TunDestroy(ctx context.Context) error {
	return l.teardown(ctx, func(c chainctl.HelperClient) error { return c.TunDestroy(ctx) })
}

func (l *lazyHelperClient) RouteSnapshot(ctx context.Context) error {
	return l.required(ctx, func(c chainctl.HelperClient) error { return c.RouteSnapshot(ctx) })
}

func (l *lazyHelperClient) RouteAdd(ctx context.Context, serverHost string) error {
	return l.required(ctx, func(c chainctl.HelperClient) error { return c.RouteAdd(ctx, serverHost) })
}

func (l *lazyHelperClient) RouteRestore(ctx context.Context) error {
	return l.teardown(ctx, func(c chainctl.HelperClient) error { return c.RouteRestore(ctx) })
}

func (l *lazyHelperClient) DnsSet(ctx context.Context, servers []string) error {
	return l.required(ctx, func(c chainctl.HelperClient) error { return c.DnsSet(ctx, servers) })
}

func (l *lazyHelperClient) DnsRestore(ctx context.Context) error {
	return l.teardown(ctx, func(c chainctl.HelperClient) error { return c.DnsRestore(ctx) })
}

func (l *lazyHelperClient) ServiceStatus(ctx context.Context) (chainctl.ChainState, error) {
	c, err := l.get(ctx)
	if err != nil {
		return chainctl.ChainState{}, err
	}
	st, err := c.ServiceStatus(ctx)
	if err != nil {
		l.invalidate()
		return chainctl.ChainState{}, err
	}
	return st, nil
}
