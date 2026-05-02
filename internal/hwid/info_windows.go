//go:build windows

package hwid

import (
	"fmt"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func platformInfo() DeviceInfo {
	info := DeviceInfo{OS: "Windows"}
	if v := windows.RtlGetVersion(); v != nil {
		info.Version = fmt.Sprintf("%d.%d.%d", v.MajorVersion, v.MinorVersion, v.BuildNumber)
	}
	if k, err := registry.OpenKey(registry.LOCAL_MACHINE, `HARDWARE\DESCRIPTION\System\BIOS`, registry.QUERY_VALUE); err == nil {
		defer k.Close()
		if model, _, err := k.GetStringValue("SystemProductName"); err == nil {
			info.Model = model
		}
	}
	return info
}
