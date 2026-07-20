// Package sysproxy sets / clears the per-user system proxy on Windows. On
// non-Windows platforms every call is a no-op.
package sysproxy

import "errors"

// Settings holds the per-protocol proxy addresses to register at the OS
// level. Empty values mean "do not register this protocol". Both empty
// is the same as Clear() — Set(Settings{}) is a no-op write.
type Settings struct {
	Socks string // e.g. "127.0.0.1:1080"
	HTTP  string // e.g. "127.0.0.1:8888"
}

// ErrNotifyOnly wraps a Set() failure where every registry write already
// succeeded and only the live WinINet hot-reload notification failed (e.g.
// blocked by AV/sandboxing). The OS proxy state on disk is fully correct in
// this case — callers can use errors.Is(err, sysproxy.ErrNotifyOnly) to
// distinguish it from a real write failure and choose to treat it as
// success rather than rolling back a perfectly good proxy configuration.
var ErrNotifyOnly = errors.New("sysproxy: notifyWinInet failed (registry state is correct)")

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
