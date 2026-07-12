//go:build windows

package server

import "github.com/itg-team/itg-ray/internal/helper/runtime"

func engineLogDir() string { return runtime.BasePath() }
