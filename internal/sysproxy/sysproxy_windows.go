//go:build windows

package sysproxy

import (
	"fmt"
	"syscall"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	regPath              = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`
	wininetSettingChange = 39 // INTERNET_OPTION_SETTINGS_CHANGED
	wininetRefresh       = 37 // INTERNET_OPTION_REFRESH
)

// winManager is the Windows Manager backed by HKCU registry values + WinINet.
type winManager struct{}

func newManager() Manager { return winManager{} }

// Set writes ProxyEnable=1, ProxyServer=addr, ProxyOverride="<local>" under
// HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings and
// notifies WinINet so running browsers pick up the change.
func (winManager) Set(addr string) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, regPath, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return fmt.Errorf("sysproxy.Set: open: %w", err)
	}
	defer func() { _ = k.Close() }()
	if err := k.SetDWordValue("ProxyEnable", 1); err != nil {
		return fmt.Errorf("sysproxy.Set: ProxyEnable: %w", err)
	}
	if err := k.SetStringValue("ProxyServer", addr); err != nil {
		return fmt.Errorf("sysproxy.Set: ProxyServer: %w", err)
	}
	if err := k.SetStringValue("ProxyOverride", "<local>"); err != nil {
		return fmt.Errorf("sysproxy.Set: ProxyOverride: %w", err)
	}
	return notifyWinInet()
}

// Clear sets ProxyEnable=0 and removes ProxyServer/ProxyOverride. If the key
// is already absent, returns nil without error.
func (winManager) Clear() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, regPath, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return nil // already absent
	}
	defer func() { _ = k.Close() }()
	_ = k.SetDWordValue("ProxyEnable", 0)
	_ = k.DeleteValue("ProxyServer")
	_ = k.DeleteValue("ProxyOverride")
	return notifyWinInet()
}

// IsSet returns true when ProxyEnable=1 under the standard HKCU Internet
// Settings key. A missing key or value yields (false, nil).
func (winManager) IsSet() (bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, regPath, registry.QUERY_VALUE)
	if err != nil {
		return false, nil
	}
	defer func() { _ = k.Close() }()
	v, _, err := k.GetIntegerValue("ProxyEnable")
	if err != nil {
		return false, nil
	}
	return v == 1, nil
}

func notifyWinInet() error {
	wininet, err := windows.LoadLibrary("wininet.dll")
	if err != nil {
		return fmt.Errorf("sysproxy: load wininet: %w", err)
	}
	defer func() { _ = windows.FreeLibrary(wininet) }()
	proc, err := windows.GetProcAddress(wininet, "InternetSetOptionW")
	if err != nil {
		return fmt.Errorf("sysproxy: GetProcAddress: %w", err)
	}
	for _, opt := range []uintptr{wininetSettingChange, wininetRefresh} {
		// InternetSetOptionW(hInternet=NULL, dwOption=opt, lpBuffer=NULL, dwBufferLength=0)
		_, _, _ = syscall.SyscallN(proc, 0, opt, 0, 0)
	}
	return nil
}
