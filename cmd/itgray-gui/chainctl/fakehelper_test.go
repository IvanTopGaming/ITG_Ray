package chainctl

import (
	"context"
	"sync"
)

// fakeHelper implements HelperClient in-memory for tests. Op call order
// is recorded in calls; failOn forces errStub on the named op.
type fakeHelper struct {
	mu        sync.Mutex
	running   bool
	failOn    string
	upBytes   uint64
	downBytes uint64
	lastError string
	calls     []string
}

func newFake() *fakeHelper { return &fakeHelper{} }

// note appends op to the call log; if failOn matches the op name it
// returns errStub before the op's normal side effects run.
func (f *fakeHelper) note(op string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, op)
	if f.failOn == op {
		return errFail
	}
	return nil
}

// errStub is a sentinel error type used to force fake helper failures
// without dragging in errors.New from the standard test deps.
type errStub string

func (e errStub) Error() string { return string(e) }

var errFail = errStub("forced failure")

func (f *fakeHelper) StartChain(_ context.Context, _, _ []byte) error {
	if err := f.note("StartChain"); err != nil {
		return err
	}
	f.mu.Lock()
	f.running = true
	f.mu.Unlock()
	return nil
}

func (f *fakeHelper) StopChain(_ context.Context) error {
	if err := f.note("StopChain"); err != nil {
		return err
	}
	f.mu.Lock()
	f.running = false
	f.mu.Unlock()
	return nil
}

func (f *fakeHelper) TunCreate(_ context.Context, _, _ string) error { return f.note("TunCreate") }
func (f *fakeHelper) TunDestroy(_ context.Context) error             { return f.note("TunDestroy") }
func (f *fakeHelper) RouteSnapshot(_ context.Context) error          { return f.note("RouteSnapshot") }
func (f *fakeHelper) RouteAdd(_ context.Context, _ string) error     { return f.note("RouteAdd") }
func (f *fakeHelper) RouteRestore(_ context.Context) error           { return f.note("RouteRestore") }
func (f *fakeHelper) DnsSet(_ context.Context, _ []string) error     { return f.note("DnsSet") }
func (f *fakeHelper) DnsRestore(_ context.Context) error             { return f.note("DnsRestore") }

func (f *fakeHelper) ServiceStatus(_ context.Context) (ChainState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return ChainState{
		Running:   f.running,
		UpBytes:   f.upBytes,
		DownBytes: f.downBytes,
		LastError: f.lastError,
	}, nil
}
