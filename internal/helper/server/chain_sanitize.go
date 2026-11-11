package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net"
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
		for k, raw := range m {
			if !strings.EqualFold(k, key) {
				continue
			}
			v, ok := raw.(string)
			if !ok || v == "" {
				continue
			}
			if !withinDir(v, runtimeDir) {
				m[k] = filepath.Join(runtimeDir, fallback)
				changed = true
			}
		}
	}

	switch kind {
	case "sing-box":
		for _, log := range foldMaps(doc, "log") {
			confine(log, "output", "sing-box-log.txt")
		}
		for _, exp := range foldMaps(doc, "experimental") {
			for _, cf := range foldMaps(exp, "cache_file") {
				confine(cf, "path", "cache.db")
			}
		}
		for _, exp := range foldMaps(doc, "experimental") {
			for _, api := range foldMaps(exp, "clash_api") {
				hasSecret := false
				for k, v := range api {
					if strings.EqualFold(k, "secret") {
						if sv, ok := v.(string); ok && sv != "" {
							hasSecret = true
						} else {
							delete(api, k)
						}
					}
				}
				if !hasSecret {
					api["secret"] = randomSecret()
					changed = true
				}
				for k, v := range api {
					switch strings.ToLower(k) {
					case "external_ui", "external_ui_download_url", "external_ui_download_detour", "access_control_allow_private_network":
						delete(api, k)
						changed = true
					case "external_controller":
						if addr, ok := v.(string); ok {
							if fixed := loopbackController(addr); fixed != addr {
								api[k] = fixed
								changed = true
							}
						}
					}
				}
			}
		}
	case "xray":
		for _, log := range foldMaps(doc, "log") {
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

func foldMaps(m map[string]any, key string) []map[string]any {
	var out []map[string]any
	for k, v := range m {
		if !strings.EqualFold(k, key) {
			continue
		}
		if sub, ok := v.(map[string]any); ok {
			out = append(out, sub)
		}
	}
	return out
}

func loopbackController(addr string) string {
	_, port, err := net.SplitHostPort(addr)
	if err != nil || port == "" {
		return "127.0.0.1:0"
	}
	return net.JoinHostPort("127.0.0.1", port)
}

func randomSecret() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
