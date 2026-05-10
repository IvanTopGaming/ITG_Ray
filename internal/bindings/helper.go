package bindings

import (
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
	Restart(name string) error
	Reinstall(name string) error
}

// realSvcOps is the production implementation. Status is a read-only SCM
// query that runs as the GUI's standard-user token. Install / Start / Stop
// require admin elevation — they shell out through `itgray-cli.exe` via
// PowerShell `Start-Process -Verb RunAs`, so the GUI itself stays
// unprivileged and a single UAC prompt covers each mutation.
//
// The binPath / desc args on Install are accepted (and ignored) for
// interface parity with the test fake — the elevated CLI call resolves the
// helper binary by its own canonical sibling-path lookup.
type realSvcOps struct{}

// Status delegates to svcmgr.Status — no elevation needed.
func (realSvcOps) Status(n string) (svcmgr.State, error) { return svcmgr.Status(n) }

// Install registers the service via the elevated CLI (UAC prompt).
func (realSvcOps) Install(_, _, _ string) error {
	return elevateCLI("helper", "install")
}

// Start asks SCM to start the helper via the elevated CLI (UAC prompt).
func (realSvcOps) Start(_ string) error {
	return elevateCLI("helper", "start")
}

// Stop asks SCM to stop the helper via the elevated CLI (UAC prompt).
func (realSvcOps) Stop(_ string) error {
	return elevateCLI("helper", "stop")
}

// Restart asks the elevated CLI to stop+start the helper in a single
// process so the user sees one UAC prompt for the whole operation.
func (realSvcOps) Restart(_ string) error {
	return elevateCLI("helper", "restart")
}

// Reinstall asks the elevated CLI to stop+uninstall+install+start the
// helper in a single process. Single UAC prompt.
func (realSvcOps) Reinstall(_ string) error {
	return elevateCLI("helper", "reinstall")
}

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
func (h *HelperService) Status() (string, error) {
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
func (h *HelperService) Install(exePath string) error {
	if strings.TrimSpace(exePath) == "" {
		exePath = defaultHelperExePath()
	}
	return h.ops.Install(helperServiceName, exePath, "ITG Ray helper service")
}

// Start asks SCM to start the helper.
func (h *HelperService) Start() error { return h.ops.Start(helperServiceName) }

// Stop asks SCM to stop the helper.
func (h *HelperService) Stop() error { return h.ops.Stop(helperServiceName) }

// Restart asks SCM to stop and then start the helper. One UAC prompt.
func (h *HelperService) Restart() error { return h.ops.Restart(helperServiceName) }

// Reinstall stops the helper, removes its SCM registration, re-registers
// it with the canonical helper.exe path, and starts it. One UAC prompt.
func (h *HelperService) Reinstall() error { return h.ops.Reinstall(helperServiceName) }

// isNotInstalled is a thin wrapper kept for backwards-compatible call
// sites; the real logic lives in svcmgr.IsNotInstalled.
func isNotInstalled(err error) bool { return svcmgr.IsNotInstalled(err) }

// defaultHelperExePath resolves the helper binary alongside the running
// GUI executable. Falls back to the literal name if os.Executable fails.
func defaultHelperExePath() string {
	exe, err := os.Executable()
	if err != nil {
		return "itgray-helper.exe"
	}
	return filepath.Join(filepath.Dir(exe), "itgray-helper.exe")
}
