package buildinfo

import (
	"runtime/debug"
	"strings"

	xraycore "github.com/xtls/xray-core/core"
)

func setting(key string) string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, s := range bi.Settings {
		if s.Key == key {
			return s.Value
		}
	}
	return ""
}

func GitRev() string {
	rev := setting("vcs.revision")
	if rev == "" {
		return ""
	}
	if len(rev) > 7 {
		rev = rev[:7]
	}
	if setting("vcs.modified") == "true" {
		rev += "-dirty"
	}
	return rev
}

func BuildDate() string {
	t := setting("vcs.time")
	if len(t) >= 10 {
		return t[:10]
	}
	return t
}

func depVersion(path string) string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, d := range bi.Deps {
		if d.Path == path {
			return strings.TrimPrefix(d.Version, "v")
		}
	}
	return ""
}

// Engines reports the engine versions this build was linked against.
//
// xray is read from its own core.Version() rather than from the module
// version: upstream stopped tagging the Go module (v1.YYMMDD.N), so a current
// release can only be pinned by a pseudo-version such as
// 1.260327.1-0.20261109052709-676eaf66f8f0, which says nothing to a user. The
// constants in xray-core/core always carry the real release number (26.11.9).
// Every binary that shows this already links xray-core, so the import is free.
func Engines() string {
	var parts []string
	if v := depVersion("github.com/sagernet/sing-box"); v != "" {
		parts = append(parts, "sing-box "+v)
	}
	if depVersion("github.com/xtls/xray-core") != "" {
		parts = append(parts, "xray "+xraycore.Version())
	}
	return strings.Join(parts, " · ")
}
