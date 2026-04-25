//go:build !windows

package main

import "errors"

func currentUserSID() (string, error) { return "", errors.New("Windows-only") }
