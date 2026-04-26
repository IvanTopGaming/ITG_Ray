// Package icons embeds the tray icon PNGs as byte slices. Exists as a
// dedicated package because Go's //go:embed pattern cannot escape the
// package directory ("../icons/*.png" is rejected at compile time), so
// the embed must sit alongside the assets.
package icons

import _ "embed"

// TrayConnected is the green "connected" tray icon (32x32 PNG).
//
//go:embed tray-connected.png
var TrayConnected []byte

// TrayConnecting is the yellow "connecting/disconnecting" tray icon.
//
//go:embed tray-connecting.png
var TrayConnecting []byte

// TrayError is the red "error" tray icon.
//
//go:embed tray-error.png
var TrayError []byte

// TrayIdle is the gray "idle/disconnected" tray icon.
//
//go:embed tray-idle.png
var TrayIdle []byte
