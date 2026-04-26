//go:build windows

package main

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func currentUserSID() (string, error) {
	tok := windows.GetCurrentProcessToken()
	user, err := tok.GetTokenUser()
	if err != nil {
		return "", fmt.Errorf("token user: %w", err)
	}
	return user.User.Sid.String(), nil
}
