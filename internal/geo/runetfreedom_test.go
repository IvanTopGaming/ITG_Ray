package geo

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func buildFixtureZip(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	entries := map[string]string{
		"rule-set-geosite/geosite-category-ru.srs": "GEOSITE-RU",
		"rule-set-geoip/geoip-ru.srs":              "GEOIP-RU",
	}
	for name, body := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestResolve_Runetfreedom_ExtractsReferencedTags(t *testing.T) {
	zipBytes := buildFixtureZip(t)
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = w.Write(zipBytes)
	}))
	defer srv.Close()

	m := NewManager(t.TempDir(), nil)
	m.zipURLOverride = srv.URL

	got, err := m.Resolve(context.Background(), Source{Preset: PresetRunetfreedom},
		[]string{"geosite-category-ru", "geoip-ru"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if b, _ := os.ReadFile(got["geosite-category-ru"]); string(b) != "GEOSITE-RU" {
		t.Fatalf("geosite bytes = %q", b)
	}
	if b, _ := os.ReadFile(got["geoip-ru"]); string(b) != "GEOIP-RU" {
		t.Fatalf("geoip bytes = %q", b)
	}

	if _, err := m.Resolve(context.Background(), Source{Preset: PresetRunetfreedom},
		[]string{"geosite-category-ru", "geoip-ru"}); err != nil {
		t.Fatalf("second Resolve: %v", err)
	}
	if hits != 1 {
		t.Fatalf("expected 1 zip download (cache reuse), got %d", hits)
	}
}

func TestResolve_Runetfreedom_AbsentTagErrors(t *testing.T) {
	zipBytes := buildFixtureZip(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(zipBytes)
	}))
	defer srv.Close()
	m := NewManager(t.TempDir(), nil)
	m.zipURLOverride = srv.URL
	_, err := m.Resolve(context.Background(), Source{Preset: PresetRunetfreedom}, []string{"geosite-ghost"})
	if err == nil || !strings.Contains(err.Error(), "geosite-ghost") {
		t.Fatalf("want error naming absent tag, got %v", err)
	}
}

func TestResolve_Runetfreedom_CategoryFallback(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("rule-set-geosite/geosite-category-ru.srs")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("CAT-RU")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(buf.Bytes())
	}))
	defer srv.Close()

	m := NewManager(t.TempDir(), nil)
	m.zipURLOverride = srv.URL
	got, err := m.Resolve(context.Background(), Source{Preset: PresetRunetfreedom}, []string{"geosite-ru"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if b, _ := os.ReadFile(got["geosite-ru"]); string(b) != "CAT-RU" {
		t.Fatalf("fallback bytes = %q", b)
	}
}

func TestResolve_Runetfreedom_BestEffort_SkipsMissing(t *testing.T) {
	zipBytes := buildFixtureZip(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(zipBytes)
	}))
	defer srv.Close()

	m := NewManager(t.TempDir(), nil)
	m.zipURLOverride = srv.URL
	got, err := m.Resolve(context.Background(), Source{Preset: PresetRunetfreedom},
		[]string{"geoip-ru", "geosite-category-does-not-exist"})
	if err != nil {
		t.Fatalf("best-effort should not error when one tag succeeds: %v", err)
	}
	if got["geoip-ru"] == "" {
		t.Fatal("geoip-ru should have been extracted")
	}
	if _, ok := got["geosite-category-does-not-exist"]; ok {
		t.Fatal("absent tag must be skipped")
	}
}

func TestResolve_Runetfreedom_AllBaseTagsResolve(t *testing.T) {
	// Filenames exactly as they appear in the runetfreedom sing-box.zip.
	// Every entry in geo.BaseTags must map to one of these (directly or via
	// sourceNames), otherwise it is a phantom tag that always skips with a warning.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	entries := []string{
		"rule-set-geosite/geosite-category-ru.srs",
		"rule-set-geoip/geoip-ru.srs",
		"rule-set-geosite/geosite-ru-blocked.srs",
		"rule-set-geosite/geosite-category-ads-all.srs",
	}
	for _, n := range entries {
		w, err := zw.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte("X")); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(buf.Bytes())
	}))
	defer srv.Close()

	m := NewManager(t.TempDir(), nil)
	m.zipURLOverride = srv.URL
	got, err := m.Resolve(context.Background(), Source{Preset: PresetRunetfreedom}, BaseTags)
	if err != nil {
		t.Fatalf("Resolve(BaseTags): %v", err)
	}
	for _, tag := range BaseTags {
		if got[tag] == "" {
			t.Errorf("BaseTag %q did not resolve against runetfreedom naming (phantom tag?)", tag)
		}
	}
}

func TestResolve_Runetfreedom_BestEffort_AllMissingErrors(t *testing.T) {
	zipBytes := buildFixtureZip(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(zipBytes)
	}))
	defer srv.Close()

	m := NewManager(t.TempDir(), nil)
	m.zipURLOverride = srv.URL
	if _, err := m.Resolve(context.Background(), Source{Preset: PresetRunetfreedom},
		[]string{"geosite-absent-one", "geosite-absent-two"}); err == nil {
		t.Fatal("expected error when no tag is in the zip")
	}
}
