package bindings

import (
	"context"
	"fmt"

	"github.com/itg-team/itg-ray/cmd/itgray-gui/hub"
)

// SubsDeps groups dependencies passed in from main.go. SubStore reads the
// persisted subscriptions; ServerStore is reused so List can compute the
// per-sub server count without re-walking the on-disk servers.json.
type SubsDeps struct {
	SubStore    SubStore
	ServerStore ServerStore
	Hub         *hub.Hub
}

// SubsService implements the Subs.* Wails bindings. C.T6 ships List only —
// Add / Remove / SyncOne / SyncAll land in C.T7.
type SubsService struct{ d SubsDeps }

// NewSubsService constructs a new SubsService. SubsDeps is passed by value
// because the struct is small (three interface/pointer fields) and the
// constructor is invoked once at process start.
func NewSubsService(d SubsDeps) *SubsService { return &SubsService{d: d} }

// List returns every persisted subscription as a SubView, with ServerCount
// computed from the matching servers.json entries (grouped by SourceID).
// The ServerView origin-resolution map is owned by AppService.GetSnapshot;
// here we only need the raw count.
func (s *SubsService) List(_ context.Context) ([]hub.SubView, error) {
	subs, err := s.d.SubStore.Load()
	if err != nil {
		return nil, fmt.Errorf("sub.Load: %w", err)
	}
	servers, err := s.d.ServerStore.Load()
	if err != nil {
		return nil, fmt.Errorf("server.Load: %w", err)
	}
	return toSubViews(subs, serverCountBySource(servers)), nil
}
