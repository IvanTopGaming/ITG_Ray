//go:build !windows

package server

import (
	"errors"
	"net"
)

//nolint:unused // mirror of the Windows symbol; kept so non-Windows builds stay symmetrical and any accidental call fails loudly
func requireOwnerSID(_ net.Conn) error { return errors.New("auth: Windows-only") }
