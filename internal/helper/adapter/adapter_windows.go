//go:build windows

// Package adapter enumerates Windows network adapters by LUID. Used by
// OpStartChain to discover the WinTUN adapter that sing-box just created
// (sing-box does not support attach-by-name; we discover by before/after diff).
package adapter

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Snapshot returns all current adapters with their LUIDs and human-readable
// names. Order is whatever GetAdaptersAddresses returns; do not rely on it.
func Snapshot() ([]Adapter, error) {
	const flags = windows.GAA_FLAG_INCLUDE_PREFIX | windows.GAA_FLAG_SKIP_DNS_SERVER |
		windows.GAA_FLAG_SKIP_MULTICAST | windows.GAA_FLAG_SKIP_ANYCAST

	var size uint32
	// First call: discover required buffer size.
	err := windows.GetAdaptersAddresses(windows.AF_UNSPEC, flags, 0, nil, &size)
	if err != nil && !errors.Is(err, windows.ERROR_BUFFER_OVERFLOW) {
		return nil, fmt.Errorf("GetAdaptersAddresses(size): %w", err)
	}

	buf := make([]byte, size)
	first := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0])) // #nosec G103 -- canonical Win32 buffer-pointer pattern
	if err := windows.GetAdaptersAddresses(windows.AF_UNSPEC, flags, 0, first, &size); err != nil {
		return nil, fmt.Errorf("GetAdaptersAddresses: %w", err)
	}

	var out []Adapter
	for cur := first; cur != nil; cur = cur.Next {
		out = append(out, Adapter{
			LUID:         cur.Luid,
			FriendlyName: windows.UTF16PtrToString(cur.FriendlyName),
			Description:  windows.UTF16PtrToString(cur.Description),
		})
	}
	return out, nil
}
