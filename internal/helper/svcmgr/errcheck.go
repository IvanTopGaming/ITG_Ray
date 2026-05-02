package svcmgr

import "strings"

// IsNotInstalled reports whether err is a wrapped svcmgr "service does not
// exist" condition (Windows errno 1060). svcmgr does not export a typed
// sentinel for this case, so callers substring-match the wrapped error
// instead. Lower-cased to absorb capitalization differences from
// golang.org/x/sys/windows/svc/mgr.
func IsNotInstalled(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "does not exist")
}

// IsNotRunning reports whether err indicates the service was already in
// the stopped state when a Stop call was issued. Used by the CLI
// `helper restart` and `helper reinstall` subcommands to swallow this
// benign mid-sequence error.
func IsNotRunning(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "has not been started") ||
		strings.Contains(msg, "is not started") ||
		strings.Contains(msg, "not running")
}
