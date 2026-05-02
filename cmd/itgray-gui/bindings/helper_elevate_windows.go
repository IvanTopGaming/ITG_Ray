//go:build windows

package bindings

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// defaultCLIPath returns the itgray-cli.exe path resolved alongside the
// running GUI executable. Helper management goes through the CLI rather
// than direct svcmgr calls because:
//
//  1. The CLI has been the canonical Helper-install path since B.5 and
//     has been smoke-verified on the VM repeatedly.
//  2. Shell-out via PowerShell Start-Process -Verb RunAs gives us UAC
//     elevation for free — Wails v2 has no built-in elevation flow,
//     and embedding requireAdministrator in the manifest would force
//     UAC on every launch (wrong for a daily-use VPN client).
//  3. If the user is already admin (e.g., installer-run scenario), the
//     RunAs verb is a no-op — no double UAC prompt.
func defaultCLIPath() string {
	exe, err := os.Executable()
	if err != nil {
		return "itgray-cli.exe"
	}
	return filepath.Join(filepath.Dir(exe), "itgray-cli.exe")
}

// elevateCLI runs `itgray-cli.exe <args...>` via PowerShell with the RunAs
// verb. The current GUI process (running as the logged-in user) keeps
// running unprivileged; only the spawned CLI invocation gets the admin
// token. Returns an error if the user dismisses the UAC prompt or the CLI
// exits non-zero.
//
// The PowerShell -Wait flag blocks until the elevated child exits, so the
// frontend's "Working…" indicator stays correct without extra polling.
func elevateCLI(args ...string) error {
	cliPath := defaultCLIPath()
	if _, err := os.Stat(cliPath); err != nil {
		return fmt.Errorf("itgray-cli.exe not found at %s: %w", cliPath, err)
	}

	// Build -ArgumentList: each token wrapped in single quotes (PowerShell
	// string literal syntax). Caller-supplied tokens that already contain a
	// single quote are escaped by doubling them, per PS literal rules.
	quoted := make([]string, 0, len(args))
	for _, a := range args {
		quoted = append(quoted, "'"+strings.ReplaceAll(a, "'", "''")+"'")
	}
	argList := strings.Join(quoted, ", ")

	// `-ErrorAction Stop` + try/catch is required so a UAC dismissal
	// becomes a terminating error and surfaces as a non-zero exit. Without
	// it, Start-Process emits a non-terminating error, $p stays $null, and
	// `exit $p.ExitCode` evaluates to `exit $null` → 0 — Go would see the
	// op as successful and the GUI would never show the inline-error block.
	//
	// The two `OutputEncoding` lines force stdout/stderr to UTF-8 so the
	// localized Windows error message (e.g. Russian under ru-RU locale)
	// reaches Go as valid UTF-8 instead of OEM/CP866 bytes that render as
	// mojibake in the React inline-error block.
	psCmd := fmt.Sprintf(
		"[Console]::OutputEncoding = [System.Text.Encoding]::UTF8; "+
			"$OutputEncoding = [System.Text.Encoding]::UTF8; "+
			"try { $p = Start-Process -FilePath '%s' -ArgumentList @(%s) "+
			"-Verb RunAs -Wait -PassThru -WindowStyle Hidden -ErrorAction Stop; "+
			"exit $p.ExitCode } "+
			"catch { Write-Host $_.Exception.Message; exit 1 }",
		strings.ReplaceAll(cliPath, "'", "''"),
		argList,
	)

	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", psCmd)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("elevated cli %v failed: %w (output: %s)",
			args, err, strings.TrimSpace(string(out)))
	}
	return nil
}
