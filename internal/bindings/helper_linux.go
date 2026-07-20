//go:build linux

package bindings

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/itg-team/itg-ray/internal/helper/svcmgr"
)

// daemonSocketPath is the fixed unix socket the privileged helper daemon
// binds on startup (mirrors chainctl.daemonSocketPath). Reachability of
// this socket is the Linux "helper installed & running" signal.
const daemonSocketPath = "/run/itgray-helper.sock"

// realHelperStatus is the Linux status probe: it dials the daemon socket.
// A successful connect means the daemon is up (installed & running); a dial
// failure is mapped to a "does not exist" error so the shared Status()
// classifies it as "missing" (svcmgr.IsNotInstalled substring-matches it).
func realHelperStatus(_ string) (svcmgr.State, error) {
	conn, err := net.DialTimeout("unix", daemonSocketPath, 400*time.Millisecond)
	if err != nil {
		return "", errors.New("itgray helper: socket does not exist")
	}
	_ = conn.Close()
	return svcmgr.State("Running"), nil
}

// InstallLinux elevates the bundled helper via pkexec so the systemd unit
// install runs as root. Task 5's install() copies the sing-box/xray cores
// from the directory holding the helper executable, so before elevating we
// stage the helper + both cores into a fresh temp dir and point pkexec at
// the staged helper. This sidesteps the read-only AppImage mount (we never
// write into resources/) and keeps the elevated logic in the Go install
// subcommand — the RPC only elevates it.
func (h *HelperService) InstallLinux() error {
	helperSrc, cores, err := resolveBundledHelperAndCores()
	if err != nil {
		return err
	}
	staging, err := os.MkdirTemp("", "itgray-helper-install-")
	if err != nil {
		return fmt.Errorf("stage helper: %w", err)
	}
	defer func() { _ = os.RemoveAll(staging) }()

	stagedHelper := filepath.Join(staging, "itgray-helper")
	if err := copyFileMode(helperSrc, stagedHelper, 0o755); err != nil {
		return fmt.Errorf("stage helper binary: %w", err)
	}
	for _, src := range cores {
		if err := copyFileMode(src, filepath.Join(staging, filepath.Base(src)), 0o755); err != nil {
			return fmt.Errorf("stage %s: %w", filepath.Base(src), err)
		}
	}

	name, args := installHelperArgs(stagedHelper, os.Getuid())
	return runElevated(name, args)
}

// installedHelperPath is the root-owned copy the install flow lays down in
// installDir (cmd/itgray-helper/service_linux.go). Uninstall must pkexec this
// copy, not the bundled one: root-via-pkexec cannot traverse the read-only
// AppImage FUSE mount (no allow_root), so pointing at resources/helper/ fails.
const installedHelperPath = "/usr/local/lib/itgray/itgray-helper"

// UninstallLinux elevates the installed helper's `uninstall` subcommand via
// pkexec. It targets the root-owned installed copy (installedHelperPath) rather
// than the bundled binary, which lives on the read-only AppImage FUSE mount
// that root cannot traverse. No staging/cores are needed — uninstall only
// removes units, the install dir, and the socket.
func (h *HelperService) UninstallLinux() error {
	return runElevated("pkexec", []string{installedHelperPath, "uninstall"})
}

const linuxHelperUnit = "itgray-helper.service"

func (h *HelperService) StartLinux() error {
	return runElevated("pkexec", []string{"systemctl", "start", linuxHelperUnit})
}

func (h *HelperService) StopLinux() error {
	return runElevated("pkexec", []string{"systemctl", "stop", linuxHelperUnit})
}

func (h *HelperService) RestartLinux() error {
	return runElevated("pkexec", []string{"systemctl", "restart", linuxHelperUnit})
}

// resolveBundledHelperAndCores locates the bundled itgray-helper and the
// sing-box/xray cores relative to the running bridge executable. Two
// layouts are supported:
//   - dev (`npm run dev`): bridge, helper and cores are all siblings in
//     cmd/itgray-electron/dist-bridge/.
//   - packaged AppImage: bridge lives in resources/bridge/, helper in
//     resources/helper/, cores in resources/cores/ (see paths.ts BUNDLE_LAYOUT).
func resolveBundledHelperAndCores() (helper string, cores []string, err error) {
	self, err := os.Executable()
	if err != nil {
		return "", nil, err
	}
	selfDir := filepath.Dir(self)

	// dev layout: everything colocated beside the bridge binary.
	devHelper := filepath.Join(selfDir, "itgray-helper")
	if fileExists(devHelper) {
		return devHelper, []string{
			filepath.Join(selfDir, "sing-box"),
			filepath.Join(selfDir, "xray"),
		}, nil
	}

	// packaged layout: resources/{bridge,helper,cores}.
	resources := filepath.Dir(selfDir)
	helper = filepath.Join(resources, "helper", "itgray-helper")
	if !fileExists(helper) {
		return "", nil, fmt.Errorf("bundled itgray-helper not found (looked in %s and %s)", devHelper, helper)
	}
	return helper, []string{
		filepath.Join(resources, "cores", "sing-box"),
		filepath.Join(resources, "cores", "xray"),
	}, nil
}

// runElevated runs the pkexec command, surfacing its combined stdout/stderr
// in the error so a polkit denial or install failure is actionable in the
// GUI's inline error block.
func runElevated(name string, args []string) error {
	cmd := exec.Command(name, args...) //nolint:gosec // fixed argv (pkexec + staged helper)
	out, err := cmd.CombinedOutput()
	if err != nil {
		trimmed := string(out)
		if len(trimmed) > 500 {
			trimmed = trimmed[:500]
		}
		return fmt.Errorf("%s %v: %w (output: %s)", name, args, err, trimmed)
	}
	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func copyFileMode(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src) //nolint:gosec // controlled bundle path
	if err != nil {
		return err
	}
	defer in.Close()                                                       //nolint:errcheck
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode) //nolint:gosec
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
