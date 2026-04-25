//go:build windows

// Package route configures the Windows IPv4 route table via the IP Helper API
// (iphlpapi.dll). Snapshot uses the typed binding from golang.org/x/sys/windows;
// Add/Remove call CreateIpForwardEntry2 / DeleteIpForwardEntry2 through a
// LazyDLL because that version of x/sys/windows does not expose them yet.
//
// See https://learn.microsoft.com/en-us/windows/win32/api/netioapi/.
package route

import (
	"fmt"
	"net/netip"
	"unsafe" // needed to repack RawSockaddrInet (Win32 union) and to call iphlpapi procs

	"golang.org/x/sys/windows"
)

// Entry is a single route row exchanged with callers.
type Entry struct {
	DestCIDR      string `json:"dest_cidr"`
	NextHop       string `json:"next_hop"`
	InterfaceLUID uint64 `json:"interface_luid"`
	Metric        uint32 `json:"metric"`
}

// iphlpapi procs not yet exported by golang.org/x/sys/windows v0.43.0.
var (
	iphlpapi                  = windows.NewLazySystemDLL("iphlpapi.dll")
	procCreateIpForwardEntry2 = iphlpapi.NewProc("CreateIpForwardEntry2")
	procDeleteIpForwardEntry2 = iphlpapi.NewProc("DeleteIpForwardEntry2")
)

// infiniteLifetime is the Win32 sentinel for ValidLifetime/PreferredLifetime
// meaning the route never expires. Default zero is treated by Windows as
// already-expired, causing the row to be silently ignored during route
// resolution. See MIB_IPFORWARD_ROW2 docs.
const infiniteLifetime uint32 = 0xFFFFFFFF

// Snapshot reads the IPv4 forwarding table only. IPv6 routes are out of
// scope for v0.1 — we add a v6 catch-all to TUN at OpStartChain time
// (see chain_windows.go), but we don't snapshot v6 because we don't
// restore them either; the helper is not allowed to evict host v6
// routes that pre-existed the session.
func Snapshot() ([]Entry, error) {
	var table *windows.MibIpForwardTable2
	if err := windows.GetIpForwardTable2(windows.AF_INET, &table); err != nil {
		return nil, fmt.Errorf("GetIpForwardTable2: %w", err)
	}
	// FreeMibTable wants the raw pointer back as unsafe.Pointer.
	defer windows.FreeMibTable(unsafe.Pointer(table)) // #nosec G103 -- FreeMibTable signature requires unsafe.Pointer

	rows := table.Rows()
	out := make([]Entry, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		dst, ok := rawAddrToString(&r.DestinationPrefix.Prefix)
		if !ok {
			continue
		}
		nh, ok := rawAddrToString(&r.NextHop)
		if !ok {
			nh = "0.0.0.0"
		}
		out = append(out, Entry{
			DestCIDR:      fmt.Sprintf("%s/%d", dst, r.DestinationPrefix.PrefixLength),
			NextHop:       nh,
			InterfaceLUID: r.InterfaceLuid,
			Metric:        r.Metric,
		})
	}
	return out, nil
}

// Add inserts a route entry. The InterfaceLUID determines which adapter owns
// the route — typically the WinTUN LUID returned from helper.wintun.Create.
func Add(e Entry) error {
	row, err := entryToRow(e)
	if err != nil {
		return err
	}
	r1, _, _ := procCreateIpForwardEntry2.Call(uintptr(unsafe.Pointer(row))) // #nosec G103 -- Win32 proc takes a pointer to MIB_IPFORWARD_ROW2
	if r1 != 0 {
		return fmt.Errorf("CreateIpForwardEntry2 dest=%s: %w", e.DestCIDR, windows.Errno(r1))
	}
	return nil
}

// Remove deletes a route entry matching dest+next-hop+luid+metric.
func Remove(e Entry) error {
	row, err := entryToRow(e)
	if err != nil {
		return err
	}
	r1, _, _ := procDeleteIpForwardEntry2.Call(uintptr(unsafe.Pointer(row))) // #nosec G103 -- Win32 proc takes a pointer to MIB_IPFORWARD_ROW2
	if r1 != 0 {
		return fmt.Errorf("DeleteIpForwardEntry2 dest=%s: %w", e.DestCIDR, windows.Errno(r1))
	}
	return nil
}

// helpers ------------------------------------------------------------

// entryToRow packs an Entry into a MibIpForwardRow2. Accepts both IPv4 and
// IPv6 destinations; the family is inferred from the parsed prefix.
func entryToRow(e Entry) (*windows.MibIpForwardRow2, error) {
	prefix, err := netip.ParsePrefix(e.DestCIDR)
	if err != nil {
		return nil, fmt.Errorf("dest cidr %q: %w", e.DestCIDR, err)
	}
	dst := prefix.Addr()
	maxBits := 32
	if dst.Is6() {
		maxBits = 128
	}
	bits := prefix.Bits()
	if bits < 0 || bits > maxBits {
		return nil, fmt.Errorf("dest cidr %q: invalid prefix length %d for family", e.DestCIDR, bits)
	}

	nh, err := netip.ParseAddr(e.NextHop)
	if err != nil {
		if dst.Is6() {
			nh = netip.IPv6Unspecified()
		} else {
			nh = netip.IPv4Unspecified()
		}
	}

	row := &windows.MibIpForwardRow2{
		InterfaceLuid:     e.InterfaceLUID,
		Metric:            e.Metric,
		ValidLifetime:     infiniteLifetime, // infinite per Win32 docs; default zero is treated as expired
		PreferredLifetime: infiniteLifetime,
	}
	row.DestinationPrefix.PrefixLength = uint8(bits) // #nosec G115 -- bits is bounded above by maxBits (32 or 128), both fit in uint8
	setRawAddr(&row.DestinationPrefix.Prefix, dst)
	setRawAddr(&row.NextHop, nh)
	return row, nil
}

// setRawAddr4 writes an IPv4 address into a RawSockaddrInet union slot. The
// underlying RawSockaddrInet matches the layout of RawSockaddrInet4
// (Family/Port/Addr[4]/Zero[8] = 16 bytes, fits inside the 28-byte union),
// so we overlay one onto the other.
func setRawAddr4(raw *windows.RawSockaddrInet, addr netip.Addr) {
	a4 := (*windows.RawSockaddrInet4)(unsafe.Pointer(raw)) // #nosec G103 -- intentional Win32 union cast
	a4.Family = windows.AF_INET
	a4.Port = 0
	a4.Addr = addr.As4()
	a4.Zero = [8]uint8{}
}

// setRawAddr6 writes an IPv6 address into a RawSockaddrInet union slot
// by overlaying RawSockaddrInet6 onto the wider RawSockaddrInet.
func setRawAddr6(raw *windows.RawSockaddrInet, addr netip.Addr) {
	r6 := (*windows.RawSockaddrInet6)(unsafe.Pointer(raw)) // #nosec G103 -- canonical Win32 union cast (RawSockaddrInet ⊃ RawSockaddrInet6)
	r6.Family = windows.AF_INET6
	a := addr.As16()
	copy(r6.Addr[:], a[:])
}

// setRawAddr dispatches based on the address family.
func setRawAddr(raw *windows.RawSockaddrInet, addr netip.Addr) {
	if addr.Is4() {
		setRawAddr4(raw, addr)
		return
	}
	setRawAddr6(raw, addr)
}

// rawAddrToString reads either an IPv4 or IPv6 RawSockaddrInet to a string.
// Returns ok=false if the family is unrecognised.
func rawAddrToString(raw *windows.RawSockaddrInet) (string, bool) {
	switch raw.Family {
	case windows.AF_INET:
		a4 := (*windows.RawSockaddrInet4)(unsafe.Pointer(raw)) // #nosec G103 -- intentional Win32 union cast
		return netip.AddrFrom4(a4.Addr).String(), true
	case windows.AF_INET6:
		a6 := (*windows.RawSockaddrInet6)(unsafe.Pointer(raw)) // #nosec G103 -- intentional Win32 union cast
		return netip.AddrFrom16(a6.Addr).String(), true
	default:
		return "", false
	}
}
