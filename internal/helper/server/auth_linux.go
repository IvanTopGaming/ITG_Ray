//go:build linux

package server

import (
	"fmt"
	"net"
	"syscall"
)

// requirePeerUID asserts the connecting peer's uid (from SO_PEERCRED)
// equals allowedUID. The Linux analog of requireOwnerSID on Windows.
func requirePeerUID(c net.Conn, allowedUID uint32) error {
	uc, ok := c.(*net.UnixConn)
	if !ok {
		return fmt.Errorf("auth: connection is not a unix socket")
	}
	raw, err := uc.SyscallConn()
	if err != nil {
		return fmt.Errorf("auth: SyscallConn: %w", err)
	}
	var ucred *syscall.Ucred
	var sockErr error
	ctlErr := raw.Control(func(fd uintptr) {
		ucred, sockErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	})
	if ctlErr != nil {
		return fmt.Errorf("auth: raw control: %w", ctlErr)
	}
	if sockErr != nil {
		return fmt.Errorf("auth: SO_PEERCRED: %w", sockErr)
	}
	if ucred.Uid != allowedUID {
		return fmt.Errorf("auth: peer uid %d not allowed (want %d)", ucred.Uid, allowedUID)
	}
	return nil
}
