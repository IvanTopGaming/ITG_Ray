//go:build !windows

package main

func serverExcludeForTUN(ip string) []string {
	return []string{ip + "/32"}
}
