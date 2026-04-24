// Package server models VLESS servers and their persistence.
package server

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/itg-team/itg-ray/internal/vless"
)

// Origin indicates whether a server came from a subscription or was added manually.
type Origin string

// Origin values for server entries.
const (
	OriginManual       Origin = "manual"
	OriginSubscription Origin = "subscription"
)

// Server is a VLESS server entry with derived metadata (ID, display name, user flags).
type Server struct {
	ID        string       `json:"id"`
	Origin    Origin       `json:"origin"`
	SourceID  string       `json:"source_id,omitempty"`
	Name      string       `json:"name"`
	Remark    string       `json:"remark,omitempty"`
	Vless     vless.Config `json:"vless"`
	Tags      []string     `json:"tags,omitempty"`
	LatencyMS *int         `json:"latency_ms,omitempty"`
	Favorite  bool         `json:"favorite,omitempty"`
	Disabled  bool         `json:"disabled,omitempty"`
}

// StableID derives a deterministic ID from the vless config's address, port, and uuid only.
func StableID(c vless.Config) string { //nolint:gocritic // Config is a value type; caller convenience outweighs copy cost
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%s", c.Address, c.Port, c.UUID)))
	return hex.EncodeToString(sum[:8])
}

// New constructs a Server from a vless Config, inferring the display name from the remark
// or falling back to "address:port" when no remark is set.
func New(c vless.Config, origin Origin, sourceID string) Server { //nolint:gocritic // Config is a value type; caller convenience outweighs copy cost
	name := c.Remark
	if name == "" {
		name = fmt.Sprintf("%s:%d", c.Address, c.Port)
	}
	return Server{
		ID:       StableID(c),
		Origin:   origin,
		SourceID: sourceID,
		Name:     name,
		Remark:   c.Remark,
		Vless:    c,
	}
}
