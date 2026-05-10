package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestCodegenGoldenFile(t *testing.T) {
	want, err := os.ReadFile("testdata/expected_output.ts")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}

	var buf bytes.Buffer
	if err := generate("testdata", &buf); err != nil {
		t.Fatalf("generate: %v", err)
	}

	got := buf.String()
	wantStr := string(want)
	if strings.TrimSpace(got) != strings.TrimSpace(wantStr) {
		t.Fatalf("output mismatch.\nGOT:\n%s\nWANT:\n%s", got, wantStr)
	}
}
