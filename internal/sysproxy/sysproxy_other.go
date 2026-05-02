//go:build !windows

package sysproxy

// stubManager is the no-op Manager used on non-Windows builds.
type stubManager struct{}

func newManager() Manager { return stubManager{} }

// Set is a no-op on non-Windows; returns ErrUnsupported.
func (stubManager) Set(Settings) error { return ErrUnsupported }

// Clear is a no-op on non-Windows; returns nil.
func (stubManager) Clear() error { return nil }

// IsSet always returns (false, nil) on non-Windows.
func (stubManager) IsSet() (bool, error) { return false, nil }
