package bindings

import (
	"errors"
	"testing"

	"github.com/itg-team/itg-ray/internal/helper/svcmgr"
	"github.com/stretchr/testify/require"
)

// fakeSvcOps is a deterministic in-memory svcOps for HelperService unit
// tests. It records calls so the test asserts both inputs and outputs
// without touching the real Windows SCM.
type fakeSvcOps struct {
	state     svcmgr.State
	statusErr error

	installArgs [3]string
	installErr  error
	startName   string
	stopName    string
	startErr    error
	stopErr     error
}

func (f *fakeSvcOps) Status(string) (svcmgr.State, error) { return f.state, f.statusErr }
func (f *fakeSvcOps) Install(n, p, d string) error {
	f.installArgs = [3]string{n, p, d}
	return f.installErr
}
func (f *fakeSvcOps) Start(n string) error { f.startName = n; return f.startErr }
func (f *fakeSvcOps) Stop(n string) error  { f.stopName = n; return f.stopErr }

// TestHelperService_Status_Mapping covers all three status flavours: a
// running service ("running"), a stopped service ("stopped"), and a
// missing service ("missing"). Missing is detected via error-string
// substring match because svcmgr does not export a sentinel.
func TestHelperService_Status_Mapping(t *testing.T) {
	cases := []struct {
		name    string
		state   svcmgr.State
		err     error
		want    string
		wantErr bool
	}{
		{"running", svcmgr.State("Running"), nil, "running", false},
		{"stopped", svcmgr.State("Stopped"), nil, "stopped", false},
		{"missing-substring", "", errors.New("open service: service does not exist"), "missing", false},
		{"unexpected-error", "", errors.New("scm connect: access denied"), "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ops := &fakeSvcOps{state: tc.state, statusErr: tc.err}
			h := newHelperServiceWithOps(ops)
			got, err := h.Status()
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

// TestHelperService_Install_DefaultPath verifies that an empty exePath
// triggers the os.Executable-relative fallback. We can't predict the exact
// path under `go test`, but we can assert it's non-empty and ends with
// the canonical helper binary name.
func TestHelperService_Install_DefaultPath(t *testing.T) {
	ops := &fakeSvcOps{}
	h := newHelperServiceWithOps(ops)
	require.NoError(t, h.Install(""))
	require.Equal(t, helperServiceName, ops.installArgs[0])
	require.Contains(t, ops.installArgs[1], "itgray-helper.exe")
	require.Equal(t, "ITG Ray helper service", ops.installArgs[2])
}

// TestHelperService_StartStop confirms the service-name plumbing — the
// methods are otherwise pass-throughs.
func TestHelperService_StartStop(t *testing.T) {
	ops := &fakeSvcOps{}
	h := newHelperServiceWithOps(ops)
	require.NoError(t, h.Start())
	require.Equal(t, helperServiceName, ops.startName)
	require.NoError(t, h.Stop())
	require.Equal(t, helperServiceName, ops.stopName)
}
