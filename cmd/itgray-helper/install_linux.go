//go:build linux

package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

func renderServiceUnit(uid int) string {
	return fmt.Sprintf(`[Unit]
Description=ITG Ray privileged TUN helper
Requires=itgray-helper.socket
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/lib/itgray/itgray-helper
Environment=ITGRAY_ALLOW_UID=%d

[Install]
WantedBy=multi-user.target
`, uid)
}

func renderSocketUnit() string {
	return `[Unit]
Description=ITG Ray helper socket

[Socket]
ListenStream=/run/itgray-helper.sock
SocketMode=0660

[Install]
WantedBy=sockets.target
`
}

// install (run as root via pkexec) copies this binary + the sibling
// sing-box/xray cores into installDir, writes the systemd units with the
// caller's uid, and enables the socket.
func install(uid int) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("install must run as root")
	}
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	selfDir := filepath.Dir(self)
	for _, name := range []string{filepath.Base(self), "sing-box", "xray"} {
		src := filepath.Join(selfDir, name)
		if name == filepath.Base(self) {
			// install the helper under its canonical name regardless of argv0
			if err := copyExecutable(self, filepath.Join(installDir, "itgray-helper")); err != nil {
				return fmt.Errorf("copy helper: %w", err)
			}
			continue
		}
		if err := copyExecutable(src, filepath.Join(installDir, name)); err != nil {
			return fmt.Errorf("copy %s: %w", name, err)
		}
	}
	if err := os.WriteFile(filepath.Join(unitDir, "itgray-helper.service"), []byte(renderServiceUnit(uid)), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(unitDir, "itgray-helper.socket"), []byte(renderSocketUnit()), 0o644); err != nil {
		return err
	}
	if err := run("systemctl", "daemon-reload"); err != nil {
		return err
	}
	return run("systemctl", "enable", "--now", "itgray-helper.socket")
}

func uninstall() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("uninstall must run as root")
	}
	_ = run("systemctl", "disable", "--now", "itgray-helper.socket")
	_ = run("systemctl", "stop", "itgray-helper.service")
	_ = os.Remove(filepath.Join(unitDir, "itgray-helper.service"))
	_ = os.Remove(filepath.Join(unitDir, "itgray-helper.socket"))
	_ = os.RemoveAll(installDir)
	_ = os.Remove(socketPath)
	return run("systemctl", "daemon-reload")
}

func copyExecutable(src, dst string) error {
	in, err := os.Open(src) //nolint:gosec // controlled path
	if err != nil {
		return err
	}
	defer in.Close() //nolint:errcheck
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755) //nolint:gosec
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...) //nolint:gosec // fixed argv
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
