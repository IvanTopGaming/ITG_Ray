//go:build !windows

package main

import "path/filepath"

func bridgeLogDir(dataDir string) string { return filepath.Join(dataDir, "logs") }
