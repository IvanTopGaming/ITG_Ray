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
	out, bindings := Resolve(m, servers)
	rebound := false
	for _, b := range bindings {
		if b.State == StateRebound {
			rebound = true
			break
		}
	}
	if !rebound {
		return false, nil
	}
	if err := store.Save(out); err != nil {
		return false, err
	}
	return true, nil
}
