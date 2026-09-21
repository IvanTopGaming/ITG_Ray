package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"

	"github.com/itg-team/itg-ray/internal/vless"
)

func ConnectionID(c vless.Config) string {
	c.Remark = ""
	if len(c.ALPN) == 0 {
		c.ALPN = nil
	}
	raw, _ := json.Marshal(c)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:16])
}

func reconcileIncomingIDs(existing, incoming []Server, sourceID string) []Server {
	oldByConnection := make(map[string]Server)
	oldByEndpoint := make(map[string][]Server)
	reserved := make(map[string]bool)
	for _, s := range existing {
		reserved[s.ID] = true
		if s.Origin == OriginSubscription && s.SourceID == sourceID {
			key := ConnectionID(s.Vless)
			if _, ok := oldByConnection[key]; !ok {
				oldByConnection[key] = s
			}
			endpoint := StableID(s.Vless)
			oldByEndpoint[endpoint] = append(oldByEndpoint[endpoint], s)
		}
	}
	counts := make(map[string]int)
	seen := make(map[string]bool)
	unique := make([]Server, 0, len(incoming))
	for _, s := range incoming {
		key := ConnectionID(s.Vless)
		if seen[key] {
			continue
		}
		seen[key] = true
		counts[StableID(s.Vless)]++
		unique = append(unique, s)
	}
	for i := range unique {
		s := &unique[i]
		key := ConnectionID(s.Vless)
		endpoint := StableID(s.Vless)
		old := oldByEndpoint[endpoint]
		if match, ok := oldByConnection[key]; ok {
			s.ID = match.ID
		} else if len(old) == 1 && counts[endpoint] == 1 {
			s.ID = old[0].ID
		} else if counts[endpoint] > 1 || reserved[s.ID] || len(old) > 0 {
			s.ID = availableVariantID(sourceID, key, reserved)
		}
		reserved[s.ID] = true
	}
	return unique
}

func availableVariantID(sourceID, key string, reserved map[string]bool) string {
	seed := sourceID + "\x00" + key
	sum := sha256.Sum256([]byte(seed))
	id := "sub-" + hex.EncodeToString(sum[:16])
	for attempt := 1; reserved[id]; attempt++ {
		sum = sha256.Sum256([]byte(seed + "\x00" + strconv.Itoa(attempt)))
		id = "sub-" + hex.EncodeToString(sum[:16])
	}
	return id
}
