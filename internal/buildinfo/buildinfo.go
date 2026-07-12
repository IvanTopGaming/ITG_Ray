package buildinfo

import (
	"runtime/debug"
	"strings"
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

func Engines() string {
	var parts []string
	if v := depVersion("github.com/sagernet/sing-box"); v != "" {
		parts = append(parts, "sing-box "+v)
	}
	if v := depVersion("github.com/xtls/xray-core"); v != "" {
		parts = append(parts, "xray "+xrayVersion(v))
	}
	return strings.Join(parts, " · ")
}

func xrayVersion(mod string) string {
	fields := strings.Split(mod, ".")
	if len(fields) == 3 && len(fields[1]) == 6 {
		d := fields[1]
		mm := strings.TrimLeft(d[2:4], "0")
		if mm == "" {
			mm = "0"
		}
		dd := strings.TrimLeft(d[4:6], "0")
		if dd == "" {
			dd = "0"
		}
		return d[0:2] + "." + mm + "." + dd
	}
	return mod
}
