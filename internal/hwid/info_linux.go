//go:build linux

package hwid

import (
	"os"
	"strings"
)

func platformInfo() DeviceInfo {
	info := DeviceInfo{OS: "Linux"}
	if b, err := os.ReadFile("/etc/os-release"); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				info.Version = strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), `"`)
				break
			}
		}
	}
	if b, err := os.ReadFile("/sys/devices/virtual/dmi/id/product_name"); err == nil {
		model := strings.TrimSpace(string(b))
		if !isDMISentinel(model) {
			info.Model = model
		}
	}
	return info
}

// isDMISentinel returns true for well-known SMBIOS placeholder strings that
// OEMs leave unchanged. These would otherwise leak into the x-device-model
// header as misleading garbage. Mirrors what dmidecode/inxi/hostnamectl
// filter internally.
func isDMISentinel(s string) bool {
	switch s {
	case "",
		"System Product Name",
		"To Be Filled By O.E.M.",
		"Default string",
		"OEM",
		"None",
		"Not Specified",
		"Not Applicable":
		return true
	}
	return false
}
