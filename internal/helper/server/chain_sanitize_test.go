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

func TestSanitize_StripsClashExternalUI(t *testing.T) {
	in := []byte(`{"experimental":{"clash_api":{"external_controller":"127.0.0.1:13356","secret":"s","external_ui":"/etc/x","external_ui_download_url":"https://evil/ui.zip","external_ui_download_detour":"direct"}}}`)
	out := sanitizeCoreConfig("sing-box", in, t.TempDir())
	for _, k := range []string{"external_ui", "external_ui_download_url", "external_ui_download_detour"} {
		if _, ok := pathField(t, out, "experimental.clash_api."+k); ok {
			t.Fatalf("%s survived sanitize: %s", k, out)
		}
	}
	if v, _ := pathField(t, out, "experimental.clash_api.external_controller"); v != "127.0.0.1:13356" {
		t.Fatalf("external_controller lost: %s", out)
	}
	if v, _ := pathField(t, out, "experimental.clash_api.secret"); v != "s" {
		t.Fatalf("secret lost: %s", out)
	}
}

func TestSanitize_StripsClashExternalUICaseInsensitive(t *testing.T) {
	in := []byte(`{"Experimental":{"Clash_API":{"external_controller":"127.0.0.1:13356","External_UI":"/etc/x","EXTERNAL_UI_DOWNLOAD_URL":"https://evil/ui.zip"}}}`)
	out := sanitizeCoreConfig("sing-box", in, t.TempDir())
	low := strings.ToLower(string(out))
	if strings.Contains(low, "external_ui") {
		t.Fatalf("case-variant external_ui survived sanitize: %s", out)
	}
	if !strings.Contains(low, "external_controller") {
		t.Fatalf("external_controller lost: %s", out)
	}
}

func TestSanitize_ConfinesCaseVariantPaths(t *testing.T) {
	dir := t.TempDir()
	sb := sanitizeCoreConfig("sing-box", []byte(`{"Log":{"Output":"/etc/evil"},"experimental":{"Cache_File":{"PATH":"/etc/evil.db"}}}`), dir)
	if strings.Contains(string(sb), "/etc/evil") {
		t.Fatalf("case-variant sing-box path escaped sanitize: %s", sb)
	}
	xr := sanitizeCoreConfig("xray", []byte(`{"LOG":{"Access":"/etc/a","ERROR":"/etc/e"}}`), dir)
	if strings.Contains(string(xr), "/etc/") {
		t.Fatalf("case-variant xray path escaped sanitize: %s", xr)
	}
}

func TestSanitize_ForcesClashControllerOntoLoopback(t *testing.T) {
	cases := map[string]string{
		`0.0.0.0:13356`:    "127.0.0.1:13356",
		`:9090`:            "127.0.0.1:9090",
		`192.168.1.5:9090`: "127.0.0.1:9090",
		`[::]:9090`:        "127.0.0.1:9090",
		`127.0.0.1:13356`:  "127.0.0.1:13356",
	}
	for in, want := range cases {
		raw := []byte(`{"experimental":{"Clash_API":{"External_Controller":"` + in + `","secret":"s"}}}`)
		out := sanitizeCoreConfig("sing-box", raw, t.TempDir())
		var doc map[string]map[string]map[string]any
		if err := json.Unmarshal(out, &doc); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		got := doc["experimental"]["Clash_API"]["External_Controller"]
		if got != want {
			t.Fatalf("%s: got %v want %s", in, got, want)
		}
	}
}

func TestSanitize_ClashAPIRequiresSecretAndDropsPrivateNetworkCORS(t *testing.T) {
	for _, in := range []string{
		`{"experimental":{"clash_api":{"external_controller":"127.0.0.1:1","access_control_allow_private_network":true}}}`,
		`{"experimental":{"clash_api":{"external_controller":"127.0.0.1:1","Secret":""}}}`,
	} {
		out := sanitizeCoreConfig("sing-box", []byte(in), t.TempDir())
		var doc map[string]map[string]map[string]any
		if err := json.Unmarshal(out, &doc); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		api := doc["experimental"]["clash_api"]
		secret := ""
		for k, v := range api {
			if strings.EqualFold(k, "secret") {
				secret, _ = v.(string)
			}
			if strings.EqualFold(k, "access_control_allow_private_network") {
				t.Fatalf("private-network CORS survived: %s", out)
			}
		}
		if len(secret) < 32 {
			t.Fatalf("empty clash secret not replaced: %s", out)
		}
	}
}

func TestSanitize_KeepsProvidedClashSecret(t *testing.T) {
	out := sanitizeCoreConfig("sing-box", []byte(`{"experimental":{"clash_api":{"external_controller":"127.0.0.1:1","secret":"abc"}}}`), t.TempDir())
	if v, _ := pathField(t, out, "experimental.clash_api.secret"); v != "abc" {
		t.Fatalf("provided secret changed: %s", out)
	}
}

func TestWithinDir_RootRelativePathsAreOutside(t *testing.T) {
	dir := t.TempDir()
	for _, p := range []string{`\Windows\System32\evil.log`, `/etc/evil`, `\\server\share\x`} {
		if withinDir(p, dir) {
			t.Fatalf("%q treated as inside %q", p, dir)
		}
	}
	if !withinDir("sing-box-log.txt", dir) || !withinDir(filepath.Join(dir, "a.log"), dir) {
		t.Fatal("paths inside the runtime dir must stay inside")
	}
}
