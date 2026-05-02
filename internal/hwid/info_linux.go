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
		info.Model = strings.TrimSpace(string(b))
	}
	return info
}
