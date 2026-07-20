//go:build windows

package sysproxy

import (
	"fmt"
	"strings"
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

// Set writes ProxyEnable=1 and a tri-protocol ProxyServer string under
// HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings, then
// notifies WinINet. Empty fields are omitted from the string. Both empty
// is equivalent to Clear().
func (winManager) Set(s Settings) error {
	if s.Socks == "" && s.HTTP == "" {
		return winManager{}.Clear()
	}
	parts := make([]string, 0, 3)
	if s.Socks != "" {
		parts = append(parts, "socks="+s.Socks)
	}
	if s.HTTP != "" {
		// Windows convention: HTTPS reuses the HTTP entry.
		parts = append(parts, "http="+s.HTTP, "https="+s.HTTP)
	}
	value := strings.Join(parts, ";")

	k, _, err := registry.CreateKey(registry.CURRENT_USER, regPath, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return fmt.Errorf("sysproxy.Set: open: %w", err)
	}
	defer func() { _ = k.Close() }()
	if err := k.SetDWordValue("ProxyEnable", 1); err != nil {
		return fmt.Errorf("sysproxy.Set: ProxyEnable: %w", err)
	}
	// From here on, ProxyEnable=1 is committed to disk. Any further
	// failure below must force it back off before returning — otherwise a
	// partial write leaves the OS proxy enabled with a stale/empty
	// ProxyServer value pointing nowhere.
	if err := k.SetStringValue("ProxyServer", value); err != nil {
		_ = k.SetDWordValue("ProxyEnable", 0)
		return fmt.Errorf("sysproxy.Set: ProxyServer: %w", err)
	}
	if err := k.SetStringValue("ProxyOverride", "<local>"); err != nil {
		_ = k.SetDWordValue("ProxyEnable", 0)
		return fmt.Errorf("sysproxy.Set: ProxyOverride: %w", err)
	}
	if err := notifyWinInet(); err != nil {
		// Registry state is fully correct at this point — only the live
		// WinINet refresh failed. Wrap with ErrNotifyOnly so callers can
		// tell this apart from a real write failure via errors.Is.
		return fmt.Errorf("sysproxy.Set: %w: %w", ErrNotifyOnly, err)
	}
	return nil
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
