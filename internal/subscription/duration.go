package subscription

import (
	"encoding/json"
	"fmt"
	"time"
)

// Duration is a time.Duration that JSON-encodes as a human-readable
// duration string (e.g. "12h", "20s") and decodes either from a string or
// from an int64 number of nanoseconds (the default Go encoding, kept for
// backwards-compatibility with files written before this wrapper).
type Duration time.Duration

// MarshalJSON emits the Duration as a quoted string per time.Duration.String().
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// UnmarshalJSON accepts either a string ("20s") or an int64 (nanoseconds).
func (d *Duration) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		parsed, err := time.ParseDuration(s)
		if err != nil {
			return fmt.Errorf("subscription.Duration: parse %q: %w", s, err)
		}
		*d = Duration(parsed)
		return nil
	}
	var n int64
	if err := json.Unmarshal(b, &n); err != nil {
		return fmt.Errorf("subscription.Duration: %w", err)
	}
	*d = Duration(n)
	return nil
}
