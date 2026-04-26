//go:build !windows

package main

import "errors"

func runService() error {
	return errors.New("itgray-helper is Windows-only")
}
