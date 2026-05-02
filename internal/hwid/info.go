package hwid

// DeviceInfo reports best-effort device metadata for the optional Remnawave
// metadata headers. All fields are short user-visible strings; if a probe
// fails the field is empty (caller skips the corresponding header).
type DeviceInfo struct {
	OS      string // "Windows" / "macOS" / "Linux"
	Version string // OS version, e.g. "10.0.22631"
	Model   string // hardware/host model
}

// Info returns DeviceInfo via build-tagged platform probes. Probes never
// error — they return empty strings on failure.
func Info() DeviceInfo { return platformInfo() }
