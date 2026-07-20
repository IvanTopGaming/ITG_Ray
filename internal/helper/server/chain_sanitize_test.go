package server

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// pathField unmarshals cfg and returns the string at the given dotted path,
// e.g. "log.output" or "experimental.cache_file.path".
func pathField(t *testing.T, cfg []byte, dotted string) (string, bool) {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(cfg, &doc); err != nil {
		t.Fatalf("unmarshal sanitized config: %v", err)
	}
	var cur any = doc
	for _, k := range strings.Split(dotted, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return "", false
		}
		cur, ok = m[k]
		if !ok {
			return "", false
		}
	}
	s, ok := cur.(string)
	return s, ok
}

// mustJSONBytes marshals v so embedded OS paths (which contain backslashes on
// Windows) are correctly escaped — building the JSON by string concatenation
// would emit invalid escape sequences on Windows.
func mustJSONBytes(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestSanitizeCoreConfig_SingboxLogOutputEscape_Rewritten(t *testing.T) {
	rt := t.TempDir()
	escape := filepath.Join(rt, "..", "..", "evil.service")
	in := mustJSONBytes(t, map[string]any{"log": map[string]any{"level": "info", "output": escape}})
	out := sanitizeCoreConfig("sing-box", in, rt)

	got, ok := pathField(t, out, "log.output")
	if !ok {
		t.Fatal("log.output missing after sanitize")
	}
	if !withinDir(got, rt) {
		t.Fatalf("log.output = %q escaped runtimeDir %q", got, rt)
	}
	if lvl, _ := pathField(t, out, "log.level"); lvl != "info" {
		t.Fatalf("log.level = %q, want info (config corrupted)", lvl)
	}
}

func TestSanitizeCoreConfig_SingboxCacheFilePathEscape_Rewritten(t *testing.T) {
	rt := t.TempDir()
	escape := filepath.Join(rt, "..", "..", "bashrc")
	in := mustJSONBytes(t, map[string]any{
		"experimental": map[string]any{"cache_file": map[string]any{"enabled": true, "path": escape}},
	})
	out := sanitizeCoreConfig("sing-box", in, rt)

	got, ok := pathField(t, out, "experimental.cache_file.path")
	if !ok {
		t.Fatal("cache_file.path missing after sanitize")
	}
	if !withinDir(got, rt) {
		t.Fatalf("cache_file.path = %q escaped runtimeDir", got)
	}
}

func TestSanitizeCoreConfig_XrayLogPathsEscape_Rewritten(t *testing.T) {
	rt := t.TempDir()
	in := mustJSONBytes(t, map[string]any{"log": map[string]any{
		"loglevel": "warning",
		"access":   filepath.Join(rt, "..", "..", "cron"),
		"error":    filepath.Join(rt, "..", "..", "passwd"),
	}})
	out := sanitizeCoreConfig("xray", in, rt)

	for _, f := range []string{"log.access", "log.error"} {
		got, ok := pathField(t, out, f)
		if !ok {
			t.Fatalf("%s missing after sanitize", f)
		}
		if !withinDir(got, rt) {
			t.Fatalf("%s = %q escaped runtimeDir", f, got)
		}
	}
}

func TestSanitizeCoreConfig_PathAlreadyInsideRuntimeDir_Untouched(t *testing.T) {
	rt := t.TempDir()
	safe := filepath.Join(rt, "sing-box.log")
	in := mustJSONBytes(t, map[string]any{"log": map[string]any{"output": safe}})
	out := sanitizeCoreConfig("sing-box", in, rt)

	got, _ := pathField(t, out, "log.output")
	if got != safe {
		t.Fatalf("in-runtimeDir path was rewritten: %q -> %q", safe, got)
	}
}

func TestSanitizeCoreConfig_NoPathFields_ReturnsInputUnchanged(t *testing.T) {
	rt := t.TempDir()
	// Shape configgen actually emits: log without an output path.
	in := []byte(`{"log":{"level":"info","timestamp":true},"outbounds":[{"type":"vless"}]}`)
	out := sanitizeCoreConfig("sing-box", in, rt)

	if string(out) != string(in) {
		t.Fatalf("no path fields present but bytes changed:\n in: %s\nout: %s", in, out)
	}
}

func TestSanitizeCoreConfig_NonJSON_ReturnedAsIs(t *testing.T) {
	rt := t.TempDir()
	in := []byte("not json at all")
	out := sanitizeCoreConfig("sing-box", in, rt)
	if string(out) != string(in) {
		t.Fatalf("non-JSON input was altered: %s", out)
	}
}
