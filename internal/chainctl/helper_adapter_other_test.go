//go:build !windows

package chainctl

import (
	"context"
	"errors"
	"testing"
)

type fakeRunner struct {
	started  bool
	stopped  bool
	startErr error
}

func (f *fakeRunner) Start(_ context.Context, _, _ []byte) error {
	if f.startErr != nil {
		return f.startErr
	}
	f.started = true
	return nil
}
func (f *fakeRunner) Stop() error { f.stopped = true; return nil }

type fakeStats struct {
	up, down uint64
	err      error
	closed   bool
}

func (f *fakeStats) Counters(_ context.Context) (uint64, uint64, error) {
	return f.up, f.down, f.err
}
func (f *fakeStats) Close() error { f.closed = true; return nil }

func newTestClient(r *fakeRunner, s *fakeStats) *coreHelperClient {
	return &coreHelperClient{
		newRunner: func() coreRunner { return r },
		newStats:  func() counterSource { return s },
	}
}

func TestCoreHelper_StartChainRejectsTUN(t *testing.T) {
	r := &fakeRunner{}
	c := newTestClient(r, &fakeStats{})
	err := c.StartChain(context.Background(), []byte(`{}`), []byte(`{}`), ModeTUN)
	if err == nil {
		t.Fatal("expected error for TUN mode, got nil")
	}
	if r.started {
		t.Fatal("runner must not start for TUN mode")
	}
}

func TestCoreHelper_StartStopLifecycle(t *testing.T) {
	r := &fakeRunner{}
	s := &fakeStats{up: 100, down: 200}
	c := newTestClient(r, s)

	if err := c.StartChain(context.Background(), []byte(`{}`), []byte(`{}`), ModeSysProxy); err != nil {
		t.Fatalf("StartChain: %v", err)
	}
	if !r.started {
		t.Fatal("runner should have started")
	}
	st, _ := c.ServiceStatus(context.Background())
	if !st.Running || st.UpBytes != 100 || st.DownBytes != 200 {
		t.Fatalf("unexpected status: %+v", st)
	}

	if err := c.StartChain(context.Background(), []byte(`{}`), []byte(`{}`), ModeSysProxy); err == nil {
		t.Fatal("double StartChain should error")
	}

	if err := c.StopChain(context.Background()); err != nil {
		t.Fatalf("StopChain: %v", err)
	}
	if !r.stopped || !s.closed {
		t.Fatal("StopChain should stop runner and close stats")
	}
	st, _ = c.ServiceStatus(context.Background())
	if st.Running {
		t.Fatal("status should be not-running after stop")
	}
}

func TestCoreHelper_StopWhenIdleIsNil(t *testing.T) {
	c := newTestClient(&fakeRunner{}, &fakeStats{})
	if err := c.StopChain(context.Background()); err != nil {
		t.Fatalf("StopChain on idle should be nil, got %v", err)
	}
}

func TestCoreHelper_StatusCounterErrorIsZero(t *testing.T) {
	r := &fakeRunner{}
	s := &fakeStats{err: errors.New("dial fail")}
	c := newTestClient(r, s)
	_ = c.StartChain(context.Background(), []byte(`{}`), []byte(`{}`), ModeSysProxy)
	st, err := c.ServiceStatus(context.Background())
	if err != nil {
		t.Fatalf("ServiceStatus must not fail on counter error: %v", err)
	}
	if !st.Running || st.UpBytes != 0 || st.DownBytes != 0 {
		t.Fatalf("expected running with zero counters, got %+v", st)
	}
}
