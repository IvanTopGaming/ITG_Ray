package adapter

// Adapter is one row from the adapter table.
type Adapter struct {
	LUID         uint64 `json:"luid"`
	FriendlyName string `json:"friendly_name"`
	Description  string `json:"description"`
}

// Diff returns adapters present in `after` but not in `before`, keyed by LUID.
// Use this after spawning a process that creates a new adapter to find it.
func Diff(before, after []Adapter) []Adapter {
	beforeSet := make(map[uint64]struct{}, len(before))
	for _, a := range before {
		beforeSet[a.LUID] = struct{}{}
	}
	var added []Adapter
	for _, a := range after {
		if _, existed := beforeSet[a.LUID]; !existed {
			added = append(added, a)
		}
	}
	return added
}
