package bindings

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/itg-team/itg-ray/internal/helper/svcmgr"
)

// helperServiceName matches cmd/itgray-cli/helper.go and itgray-helper itself.
const helperServiceName = "ITGRayHelper"

// svcOps abstracts the four svcmgr functions HelperService consumes so unit
// tests can inject a fake without going through the real Windows SCM. The
// production constructor wires the real svcmgr functions; tests call the
// constructor with a custom implementation. ErrNotInstalled is required
// because svcmgr does not expose a typed sentinel for the Windows
// "service does not exist" error (errno 1060) — the mapping logic below
// inspects the wrapped error string instead.
type svcOps interface {
	Status(name string) (svcmgr.State, error)
	Install(name, binPath, desc string) error
	Start(name string) error
	Stop(name string) error
}

// realSvcOps is the production implementation that delegates to svcmgr.
// Kept private — only HelperService talks to it.
type realSvcOps struct{}

// Status delegates to svcmgr.Status.
func (realSvcOps) Status(n string) (svcmgr.State, error) { return svcmgr.Status(n) }

// Install delegates to svcmgr.Install.
func (realSvcOps) Install(n, p, d string) error { return svcmgr.Install(n, p, d) }

// Start delegates to svcmgr.Start.
func (realSvcOps) Start(n string) error { return svcmgr.Start(n) }

// Stop delegates to svcmgr.Stop.
func (realSvcOps) Stop(n string) error { return svcmgr.Stop(n) }

// HelperService implements the Helper.* Wails bindings (Status / Install /
// Start / Stop) the onboarding wizard uses. The methods translate svcmgr
// types into JSON-friendly strings ("running"/"stopped"/"missing").
type HelperService struct {
	ops svcOps
}

// NewHelperService returns a HelperService bound to the real Windows SCM.
// Tests construct via newHelperServiceWithOps to inject a fake.
func NewHelperService() *HelperService { return &HelperService{ops: realSvcOps{}} }

// newHelperServiceWithOps is the test-only constructor that lets a unit
// test feed in a fake svcOps. Lowercase so it stays inside the package.
func newHelperServiceWithOps(ops svcOps) *HelperService { return &HelperService{ops: ops} }

// Status returns "running", "stopped", or "missing". Any other svcmgr error
// surfaces back to the caller so the wizard can show a real failure.
func (h *HelperService) Status(_ context.Context) (string, error) {
	st, err := h.ops.Status(helperServiceName)
	if err != nil {
		if isNotInstalled(err) {
			return "missing", nil
		}
		return "", err
	}
	if st == svcmgr.State("Running") {
		return "running", nil
	}
	return "stopped", nil
}

// Install registers the helper service. exePath may be empty — in that case
// we resolve to <gui-exe-dir>/itgray-helper.exe so a fresh user does not
// need to type a Windows path.
func (h *HelperService) Install(_ context.Context, exePath string) error {
	if strings.TrimSpace(exePath) == "" {
		exePath = defaultHelperExePath()
	}
	return h.ops.Install(helperServiceName, exePath, "ITG Ray helper service")
}

// Start asks SCM to start the helper.
func (h *HelperService) Start(_ context.Context) error { return h.ops.Start(helperServiceName) }

// Stop asks SCM to stop the helper.
func (h *HelperService) Stop(_ context.Context) error { return h.ops.Stop(helperServiceName) }

// isNotInstalled inspects the wrapped svcmgr error for the Windows
// "service does not exist" condition. svcmgr does not expose a typed
// sentinel today, so we substring-match the wrapped errno description
// emitted by golang.org/x/sys/windows/svc/mgr.
func isNotInstalled(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "does not exist") || strings.Contains(msg, "service does not exist")
}

// defaultHelperExePath resolves the helper binary alongside the running
// GUI executable. Falls back to the literal name if os.Executable fails.
func defaultHelperExePath() string {
	exe, err := os.Executable()
	if err != nil {
		return "itgray-helper.exe"
	}
	return filepath.Join(filepath.Dir(exe), "itgray-helper.exe")
}
