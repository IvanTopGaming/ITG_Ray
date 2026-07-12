//go:build linux

package server

import "os"

func engineLogDir() string {
	if override := os.Getenv("ITGRAY_RUNTIME_BASE"); override != "" {
		return override
	}
	return runtimeDir
}
