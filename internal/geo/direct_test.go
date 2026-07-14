package geo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDirectURL(t *testing.T) {
	cases := map[string]string{
		"geosite-category-ru": "https://x/sing-geosite/rule-set/geosite-category-ru.srs",
		"geoip-ru":            "https://x/sing-geoip/rule-set/geoip-ru.srs",
	}
	for tag, want := range cases {
		if got := directURL(tag, "https://x"); got != want {
			t.Fatalf("directURL(%q) = %q, want %q", tag, got, want)
		}
	}
	if got := directURL("geoip-ru", "https://x/"); got != "https://x/sing-geoip/rule-set/geoip-ru.srs" {
		t.Fatalf("trailing slash not trimmed: %q", got)
	}
}

func TestResolve_Direct_DownloadsAndCaches(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if !strings.HasSuffix(r.URL.Path, "/geosite-category-ru.srs") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte("SRS-BYTES"))
	}))
	defer srv.Close()

	m := NewManager(t.TempDir(), nil)
	src := Source{Preset: PresetCustom, CustomURL: srv.URL}

	got, err := m.Resolve(context.Background(), src, []string{"geosite-category-ru"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	p := got["geosite-category-ru"]
	if p == "" {
		t.Fatal("no path returned")
	}
	if b, _ := os.ReadFile(p); string(b) != "SRS-BYTES" {
		t.Fatalf("cached bytes = %q", b)
	}
	if want := filepath.Join(m.DataDir, "geo", "custom", "geosite-category-ru.srs"); p != want {
		t.Fatalf("cache path = %q, want %q", p, want)
	}

	if _, err := m.Resolve(context.Background(), src, []string{"geosite-category-ru"}); err != nil {
		t.Fatalf("second Resolve: %v", err)
	}
	if hits != 1 {
		t.Fatalf("expected 1 HTTP hit (cache reuse), got %d", hits)
	}
}

func TestResolve_RejectsPathTraversalTag(t *testing.T) {
	m := NewManager(t.TempDir(), nil)
	for _, bad := range []string{"geosite-../../etc", "geosite-a/b", `geosite-a\b`} {
		if _, err := m.Resolve(context.Background(), Source{Preset: PresetSagerNet}, []string{bad}); err == nil {
			t.Fatalf("expected error for tag %q", bad)
		}
	}
}

func TestResolve_Direct_MissingTagErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	m := NewManager(t.TempDir(), nil)
	_, err := m.Resolve(context.Background(), Source{Preset: PresetCustom, CustomURL: srv.URL}, []string{"geosite-nope"})
	if err == nil || !strings.Contains(err.Error(), "geosite-nope") {
		t.Fatalf("want error naming tag, got %v", err)
	}
}

func TestResolve_Direct_CategoryFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/geosite-category-ru.srs") {
			_, _ = w.Write([]byte("CAT-RU"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	m := NewManager(t.TempDir(), nil)
	got, err := m.Resolve(context.Background(), Source{Preset: PresetCustom, CustomURL: srv.URL}, []string{"geosite-ru"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if b, _ := os.ReadFile(got["geosite-ru"]); string(b) != "CAT-RU" {
		t.Fatalf("fallback bytes = %q", b)
	}
}

func TestResolve_Direct_BestEffort_SkipsMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/geoip-ru.srs") {
			_, _ = w.Write([]byte("GEOIP"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	m := NewManager(t.TempDir(), nil)
	src := Source{Preset: PresetCustom, CustomURL: srv.URL}
	got, err := m.Resolve(context.Background(), src, []string{"geoip-ru", "geosite-nope"})
	if err != nil {
		t.Fatalf("best-effort should not error when one tag succeeds: %v", err)
	}
	if got["geoip-ru"] == "" {
		t.Fatal("geoip-ru should have been cached")
	}
	if _, ok := got["geosite-nope"]; ok {
		t.Fatal("missing tag must be skipped, not present")
	}
}

func TestResolve_Direct_BestEffort_AllMissingErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	m := NewManager(t.TempDir(), nil)
	src := Source{Preset: PresetCustom, CustomURL: srv.URL}
	if _, err := m.Resolve(context.Background(), src, []string{"geosite-a", "geosite-b"}); err == nil {
		t.Fatal("expected error when every tag is unavailable")
	}
}
