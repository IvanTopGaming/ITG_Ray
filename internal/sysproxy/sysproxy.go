// Package sysproxy sets / clears the per-user system proxy on Windows. On
// non-Windows platforms every call is a no-op.
package sysproxy

import "errors"

// ErrUnsupported is returned by non-Windows builds.
var ErrUnsupported = errors.New("sysproxy: not supported on this platform")

// Settings holds the per-protocol proxy addresses to register at the OS
// level. Empty values mean "do not register this protocol". Both empty
// is the same as Clear() — Set(Settings{}) is a no-op write.
type Settings struct {
	Socks string // e.g. "127.0.0.1:1080"
	HTTP  string // e.g. "127.0.0.1:8888"
}

// Manager is the cross-platform interface.
type Manager interface {
	// Set configures HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings
	// to route HTTP/HTTPS through s.HTTP (Windows convention: HTTPS reuses the
	// HTTP entry) and SOCKS through s.Socks. Empty fields omit their segment.
	Set(s Settings) error
	// Clear restores ProxyEnable=0 and removes ProxyServer/Override.
	Clear() error
	// IsSet returns true if ProxyEnable=1.
	IsSet() (bool, error)
}

// New returns a platform-specific Manager.
func New() Manager { return newManager() }
