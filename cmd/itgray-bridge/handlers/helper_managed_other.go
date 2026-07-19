//go:build !linux

package handlers

// helperIsPackageManaged is Linux-only: Windows and macOS ship the helper
// through the in-app installer exclusively.
func detectPackageManagedHelper() bool { return false }
