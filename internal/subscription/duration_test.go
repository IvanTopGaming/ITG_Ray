package subscription

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDuration_MarshalJSON_HumanReadable(t *testing.T) {
	d := Duration(12 * time.Hour)
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(b) != `"12h0m0s"` {
		t.Fatalf("got %s, want %q", b, `"12h0m0s"`)
	}
}

func TestDuration_UnmarshalJSON_StringForm(t *testing.T) {
	var d Duration
	if err := json.Unmarshal([]byte(`"20s"`), &d); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if time.Duration(d) != 20*time.Second {
		t.Fatalf("got %v, want 20s", time.Duration(d))
	}
}

func TestDuration_UnmarshalJSON_LegacyNanoseconds(t *testing.T) {
	// Old files that were written before this wrapper used int64 nanoseconds.
	var d Duration
	if err := json.Unmarshal([]byte(`60000000000`), &d); err != nil {
		t.Fatalf("unmarshal int64: %v", err)
	}
	if time.Duration(d) != time.Minute {
		t.Fatalf("got %v, want 1m", time.Duration(d))
	}
}

func TestDuration_UnmarshalJSON_Zero(t *testing.T) {
	var d Duration
	if err := json.Unmarshal([]byte(`0`), &d); err != nil {
		t.Fatalf("unmarshal 0: %v", err)
	}
	if time.Duration(d) != 0 {
		t.Fatalf("got %v, want 0", time.Duration(d))
	}
}

func TestDuration_UnmarshalJSON_InvalidString(t *testing.T) {
	var d Duration
	if err := json.Unmarshal([]byte(`"not-a-duration"`), &d); err == nil {
		t.Fatal("expected error for invalid string")
	}
}
