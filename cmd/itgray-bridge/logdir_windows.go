//go:build windows

package main

import "github.com/itg-team/itg-ray/internal/helper/runtime"

func bridgeLogDir(_ string) string { return runtime.BasePath() }
