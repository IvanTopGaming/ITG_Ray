package routebind

import (
	"github.com/itg-team/itg-ray/internal/rules"
	"github.com/itg-team/itg-ray/internal/server"
)

func Apply(store *rules.Store, servers []server.Server) (bool, error) {
	m, err := store.Load()
	if err != nil {
		return false, err
	}
	out, _ := Resolve(m, servers)
	if !bindingsDiffer(m, out) {
		return false, nil
	}
	if err := store.Save(out); err != nil {
		return false, err
	}
	return true, nil
}

func bindingsDiffer(a, b rules.Model) bool {
	for gi := range a.Groups {
		for ri := range a.Groups[gi].Rules {
			x, y := a.Groups[gi].Rules[ri], b.Groups[gi].Rules[ri]
			if x.ServerID != y.ServerID || x.ServerName != y.ServerName || x.ServerSourceID != y.ServerSourceID {
				return true
			}
		}
	}
	return false
}
