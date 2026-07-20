package server

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

// sanitizeCoreConfig neutralizes file-writing path fields in a core config
// before it is written to disk and handed to a root-privileged sing-box/xray
// process (backend-review finding H3). The only gate on OpStartChain's config
// payload is the peer-uid/SID auth check; a compromised same-uid GUI could
// otherwise point sing-box's log.output / experimental.cache_file.path or
// xray's log.access / log.error at an arbitrary root-owned path, turning
// "control the proxy config" into an arbitrary root file write.
//
// configgen never emits any of these fields, so in normal operation this is a
// no-op and the bytes are returned unchanged. When a field is present, any
// path that would escape runtimeDir is rewritten to a safe path inside it;
// paths already inside runtimeDir (or relative paths that stay within it — the
// cores run with cmd.Dir == runtimeDir) are left untouched. A payload that
// doesn't parse as a JSON object is returned as-is: the field-based primitive
// requires structured JSON, and the core rejects malformed input itself.
//
// SCOPE: this is a best-effort denylist of the highest-value, top-level
// file-writing fields — not a complete sandbox. sing-box/xray expose other
// path-bearing knobs nested inside inbound/outbound TLS settings (e.g.
// tls.acme.data_directory) that this does not walk. It meaningfully reduces
// the "compromised same-uid GUI → arbitrary root file write" surface but does
// not eliminate it; a caller that has already defeated the peer-uid auth gate
// is inside the trust boundary. Full containment (an OS-level sandbox/firewall
// around the spawned cores) is tracked as a separate hardening item.
//
// kind is "sing-box" or "xray".
func sanitizeCoreConfig(kind string, raw []byte, runtimeDir string) []byte {
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return raw
	}

	changed := false
	confine := func(m map[string]any, key, fallback string) {
		v, ok := m[key].(string)
		if !ok || v == "" {
			return
		}
		if !withinDir(v, runtimeDir) {
			m[key] = filepath.Join(runtimeDir, fallback)
			changed = true
		}
	}

	switch kind {
	case "sing-box":
		if log, ok := doc["log"].(map[string]any); ok {
			confine(log, "output", "sing-box-log.txt")
		}
		if exp, ok := doc["experimental"].(map[string]any); ok {
			if cf, ok := exp["cache_file"].(map[string]any); ok {
				confine(cf, "path", "cache.db")
			}
		}
	case "xray":
		if log, ok := doc["log"].(map[string]any); ok {
			confine(log, "access", "xray-access.log")
			confine(log, "error", "xray-error.log")
		}
	}

	if !changed {
		return raw
	}
	out, err := json.Marshal(doc)
	if err != nil {
		return raw
	}
	return out
}

// withinDir reports whether path resolves inside dir. Relative paths are
// resolved against dir (the cores' working directory), so a plain relative
// name stays contained while a "../.."-style escape does not.
func withinDir(path, dir string) bool {
	d := filepath.Clean(dir)
	clean := filepath.Clean(path)
	if !filepath.IsAbs(path) {
		clean = filepath.Clean(filepath.Join(d, path))
	}
	return clean == d || strings.HasPrefix(clean, d+string(filepath.Separator))
}
