//go:build windows

package server

import (
	"errors"
	"fmt"
	"net"
	"strings"

	"golang.org/x/sys/windows"

	"github.com/itg-team/itg-ray/internal/helper/auth"
)

// requireOwnerSID extracts the peer SID from a winio pipe connection and
// asserts it appears in the allow-list seeded by `itgray-cli helper install`.
//
// go-winio v0.6.2 does NOT expose a User() method on its PipeConn type, so we
// instead recover the underlying pipe HANDLE via the promoted Fd() method on
// the embedded *win32File, ask Win32 for the client process id with
// GetNamedPipeClientProcessId, then open that process token and read the
// token user SID.
func requireOwnerSID(c net.Conn) error {
	type fdProvider interface{ Fd() uintptr }
	fp, ok := c.(fdProvider)
	if !ok {
		return errors.New("auth: connection is not a winio pipe with Fd()")
	}
	pipeHandle := windows.Handle(fp.Fd())

	var pid uint32
	if err := windows.GetNamedPipeClientProcessId(pipeHandle, &pid); err != nil {
		return fmt.Errorf("auth: GetNamedPipeClientProcessId: %w", err)
	}

	procHandle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return fmt.Errorf("auth: OpenProcess(pid=%d): %w", pid, err)
	}
	defer windows.CloseHandle(procHandle) //nolint:errcheck // best-effort cleanup

	var token windows.Token
	if err := windows.OpenProcessToken(procHandle, windows.TOKEN_QUERY, &token); err != nil {
		return fmt.Errorf("auth: OpenProcessToken: %w", err)
	}
	defer token.Close() //nolint:errcheck // best-effort cleanup

	user, err := token.GetTokenUser()
	if err != nil {
		return fmt.Errorf("auth: GetTokenUser: %w", err)
	}
	sid := user.User.Sid.String()

	allowed, err := auth.Load()
	if err != nil {
		return fmt.Errorf("auth: load allow-list: %w", err)
	}
	for _, a := range allowed {
		if strings.EqualFold(a, sid) {
			return nil
		}
	}
	return fmt.Errorf("auth: peer sid %s not in allow-list", sid)
}
