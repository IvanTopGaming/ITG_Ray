//go:build darwin

package hwid

import (
	"os/exec"
	"strings"
)

func platformInfo() DeviceInfo {
	info := DeviceInfo{OS: "macOS"}
	if out, err := exec.Command("sw_vers", "-productVersion").Output(); err == nil {
		info.Version = strings.TrimSpace(string(out))
	}
	if out, err := exec.Command("sysctl", "-n", "hw.model").Output(); err == nil {
		info.Model = strings.TrimSpace(string(out))
	}
	return info
}
