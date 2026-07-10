package bindings

import "strconv"

// installHelperArgs builds the privileged-install command line: the GUI
// elevates the bundled helper via pkexec so the systemd unit install runs
// as root while the GUI itself stays unprivileged. Pure (no I/O) so the
// argv construction is unit-testable without spawning pkexec.
func installHelperArgs(helperPath string, uid int) (name string, args []string) {
	return "pkexec", []string{helperPath, "install", "--uid", strconv.Itoa(uid)}
}
