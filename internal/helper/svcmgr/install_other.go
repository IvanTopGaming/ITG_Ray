//go:build !windows

package svcmgr

import "errors"

// State is a stub on non-Windows so callers compile on dev box.
type State string

// errPlatform is returned by every function on non-Windows platforms.
var errPlatform = errors.New("svcmgr: Windows-only")

// Install registers the service in the SCM. Stub on non-Windows.
//
//nolint:gocritic // signature mirrors the Windows impl verbatim
func Install(_ string, _ string, _ string) error { return errPlatform }

// Uninstall removes the service. Stub on non-Windows.
func Uninstall(_ string) error { return errPlatform }

// Start asks SCM to start the service. Stub on non-Windows.
func Start(_ string) error { return errPlatform }

// Stop asks SCM to stop the service. Stub on non-Windows.
func Stop(_ string) error { return errPlatform }

// Status reports the current SCM state. Stub on non-Windows.
func Status(_ string) (State, error) { return "", errPlatform }
