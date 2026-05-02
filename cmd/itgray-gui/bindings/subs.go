package bindings

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/itg-team/itg-ray/cmd/itgray-gui/hub"
	"github.com/itg-team/itg-ray/internal/subscription"
)

// syncTimeout bounds a single subscription fetch. Mirrors the CLI's
// `subs sync` 30s budget so a slow provider does not block the GUI's
// SyncOne / SyncAll for longer than the user is likely to wait.
const syncTimeout = 30 * time.Second

// defaultUpdateInterval is the polling cadence written to a freshly-added
// Stored when the user does not supply one in the AddSubDialog. Matches
// SubscriptionSettings.DefaultUpdateInterval (1 hour) returned by
// AppService.collectSettings.
const defaultUpdateInterval = time.Hour

// errInvalidURL is returned by Add when the URL string fails the
// http/https scheme check. Kept as a sentinel so the frontend can surface
// a translatable message instead of leaking the raw url.Parse error.
var errInvalidURL = errors.New("subscription URL must be http or https")

// errSubNotFound is returned by SyncOne / Remove when the supplied id has
// no entry in the on-disk subscriptions file. Mirrors ErrServerNotFound
// from servers.go.
var errSubNotFound = errors.New("subscription not found")

// SubsDeps groups dependencies passed in from main.go. SubStore reads/writes
// the persisted subscriptions; ServerStore is reused so List can compute
// the per-sub server count without re-walking the on-disk servers.json,
// and Sync can persist merged servers atomically.
type SubsDeps struct {
	SubStore    SubStore
	ServerStore ServerStore
	Hub         *hub.Hub
}

// SubsService implements the Subs.* Wails bindings. C.T7 ships List + Add
// + Remove + SyncOne + SyncAll. Edit / Export / per-sub UpdateInterval
// changes land in later tasks.
type SubsService struct{ d SubsDeps }

// NewSubsService constructs a new SubsService. SubsDeps is passed by value
// because the struct is small (three interface/pointer fields) and the
// constructor is invoked once at process start.
func NewSubsService(d SubsDeps) *SubsService { return &SubsService{d: d} }

// List returns every persisted subscription as a SubView, with ServerCount
// computed from the matching servers.json entries (grouped by SourceID).
// The ServerView origin-resolution map is owned by AppService.GetSnapshot;
// here we only need the raw count.
func (s *SubsService) List() ([]hub.SubView, error) {
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

// Add validates the URL, generates a CLI-compatible "s<unix-seconds>" id,
// appends the entry to the on-disk file, and kicks off a one-shot SyncOne
// in the background so the new subscription's servers materialize without
// the user having to click "Sync now" first. The SyncOne goroutine uses a
// fresh context.Background() so the fetch survives the JS promise unwinding.
//
// Returns the SubView so the frontend can optimistically insert the new
// row before the next snapshot refresh arrives.
func (s *SubsService) Add(rawURL, name string) (hub.SubView, error) {
	rawURL = strings.TrimSpace(rawURL)
	if err := validateSubURL(rawURL); err != nil {
		return hub.SubView{}, err
	}
	stored := subscription.Stored{
		ID:             generateSubID(),
		Name:           strings.TrimSpace(name),
		URL:            rawURL,
		UpdateInterval: subscription.Duration(defaultUpdateInterval),
	}
	subs, err := s.d.SubStore.Load()
	if err != nil {
		return hub.SubView{}, fmt.Errorf("sub.Load: %w", err)
	}
	subs = append(subs, stored)
	if err := s.d.SubStore.Save(subs); err != nil {
		return hub.SubView{}, fmt.Errorf("sub.Save: %w", err)
	}
	view := toSubViews([]subscription.Stored{stored}, nil)[0]
	go func(id string) {
		_ = s.SyncOne(id)
	}(stored.ID)
	return view, nil
}

// Remove deletes the subscription with the given id and rewrites the file.
// Servers carrying the removed sub's SourceID are deliberately *not*
// cascaded — they keep showing in the table with origin "manual" until
// the user prunes them. Mirrors the CLI's `sub remove` semantics so the
// two surfaces remain interchangeable.
func (s *SubsService) Remove(id string) error {
	subs, err := s.d.SubStore.Load()
	if err != nil {
		return fmt.Errorf("sub.Load: %w", err)
	}
	out := subs[:0]
	for _, sub := range subs {
		if sub.ID != id {
			out = append(out, sub)
		}
	}
	if err := s.d.SubStore.Save(out); err != nil {
		return fmt.Errorf("sub.Save: %w", err)
	}
	return nil
}

// Edit updates name and/or URL of an existing subscription. When the URL
// changes, all servers tagged with this sub's SourceID are removed from
// servers.json before SubStore.Save persists the new metadata, so the
// next SyncOne brings in a fresh, ghost-free server set. LastSyncAt and
// quota fields are reset on URL change; rename-only edits preserve them.
//
// Returns the updated SubView for optimistic frontend reconciliation.
func (s *SubsService) Edit(id, rawURL, name string) (hub.SubView, error) {
	rawURL = strings.TrimSpace(rawURL)
	if err := validateSubURL(rawURL); err != nil {
		return hub.SubView{}, err
	}
	subs, err := s.d.SubStore.Load()
	if err != nil {
		return hub.SubView{}, fmt.Errorf("sub.Load: %w", err)
	}
	idx := -1
	for i := range subs {
		if subs[i].ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return hub.SubView{}, errSubNotFound
	}

	urlChanged := subs[idx].URL != rawURL

	if urlChanged {
		// URL-change branch lands in Task 2.
		return hub.SubView{}, fmt.Errorf("URL change not yet implemented")
	}

	subs[idx].Name = strings.TrimSpace(name)
	if err := s.d.SubStore.Save(subs); err != nil {
		return hub.SubView{}, fmt.Errorf("sub.Save: %w", err)
	}

	servers, err := s.d.ServerStore.Load()
	if err != nil {
		return hub.SubView{}, fmt.Errorf("server.Load: %w", err)
	}
	return toSubViews([]subscription.Stored{subs[idx]}, serverCountBySource(servers))[0], nil
}

// SyncOne fetches one subscription, merges its servers into servers.json,
// updates the sub's LastSyncAt/LastStatus, and publishes a sub:synced
// event so subscribers (the frontend store reducer, the tray) can refresh
// without re-fetching the full snapshot.
//
// The hub event always fires — both on success and on failure — because
// the frontend's applySubSync reducer treats an ERROR status as a UI
// signal (red badge) just as much as OK is (green badge with new count).
//
// The Wails binding signature drops ctx (Wails v2.11 does not auto-inject
// it for service methods); subscription.Sync receives a fresh
// context.Background() bounded by syncTimeout (30s). Accepted tradeoff:
// no per-call cancellation from the frontend.
func (s *SubsService) SyncOne(id string) error {
	ctx := context.Background()
	subs, err := s.d.SubStore.Load()
	if err != nil {
		return fmt.Errorf("sub.Load: %w", err)
	}
	var found *subscription.Stored
	for i := range subs {
		if subs[i].ID == id {
			found = &subs[i]
			break
		}
	}
	if found == nil {
		return errSubNotFound
	}

	existing, err := s.d.ServerStore.Load()
	if err != nil {
		return fmt.Errorf("server.Load: %w", err)
	}

	merged, meta, syncErr := subscription.Sync(ctx, found.ToSyncInput(), existing, syncTimeout)
	// Capture the upstream-fetch outcome before the Save branch may
	// overwrite syncErr — Userinfo is meaningful exactly when the fetch
	// itself succeeded, regardless of whether the disk write that follows
	// it succeeds.
	syncOK := syncErr == nil

	// Local override pattern: status/msg start from meta and are explicitly
	// overridden only when ServerStore.Save fails after a successful Sync.
	// Keeps a single UpdateMeta call site at the bottom.
	status := meta.Status
	msg := meta.Message
	imported := 0
	if syncOK {
		// Successful sync: persist merged servers + count post-merge entries
		// belonging to this sub. importedCount is what the reducer uses to
		// keep the badge fresh between snapshot refreshes.
		if err := s.d.ServerStore.Save(merged); err != nil {
			// Demote to error so the frontend's red badge surfaces the
			// disk failure; the user can retry. syncErr is set so the
			// return value reflects the disk failure too.
			status = "error"
			msg = fmt.Sprintf("server.Save: %v", err)
			syncErr = err
		} else {
			for i := range merged {
				if merged[i].SourceID == id {
					imported++
				}
			}
		}
	}

	// UpdateMeta is best-effort: a write failure here is intentionally
	// swallowed so a transient meta-write hiccup does not mask the real
	// fetch error in syncErr below. CLI truncates the message to 120
	// bytes; mirror that so the on-disk LastMessage stays manageable.
	// Attach Userinfo whenever the upstream fetch succeeded; the gate must
	// be syncOK, not syncErr, since a post-fetch Save failure overwrites
	// syncErr but does not invalidate the parsed quota.
	var ui *subscription.Userinfo
	if syncOK {
		ui = meta.Headers.Userinfo
	}
	_ = s.d.SubStore.UpdateMeta(id, time.Now(), status, truncate(msg, 120), ui)

	s.d.Hub.Publish(hub.Event{
		Name: hub.EventSubSynced,
		Payload: map[string]any{
			"id":            id,
			"status":        status,
			"at":            time.Now().UTC().Format(time.RFC3339),
			"importedCount": imported,
			"message":       msg,
		},
	})
	return syncErr
}

// SyncAll iterates over every subscription, calling SyncOne on each. A
// failure on one sub does not abort the loop — each sub's own sub:synced
// event already carries the per-entry status, so the frontend can react
// independently. The aggregate return is nil unless the initial Load
// itself failed.
func (s *SubsService) SyncAll() error {
	subs, err := s.d.SubStore.Load()
	if err != nil {
		return fmt.Errorf("sub.Load: %w", err)
	}
	for _, sub := range subs {
		_ = s.SyncOne(sub.ID)
	}
	return nil
}

// validateSubURL accepts a non-empty http(s) URL with a host. We resist
// pulling in a heavier validator: the actual fetch happens inside
// subscription.Sync, which surfaces network errors with full context;
// this check exists only to reject obvious typos before any disk I/O.
func validateSubURL(s string) error {
	if s == "" {
		return errInvalidURL
	}
	u, err := url.Parse(s)
	if err != nil {
		// Wrap both: errInvalidURL is the sentinel callers match on, the
		// parse error gives developers the actual reason in logs.
		return fmt.Errorf("%w: %w", errInvalidURL, err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return errInvalidURL
	}
	if u.Host == "" {
		return errInvalidURL
	}
	return nil
}

// generateSubID returns a fresh subscription ID of the shape s<unix-millis>-<hex4>.
// Millisecond resolution + a 16-bit random suffix prevents collisions when the
// user clicks "+ Add subscription" twice within the same wall-clock second
// (the prior unix-second IDs from cmd/itgray-cli/subs.go could clobber each
// other under rapid Add/Remove). Existing on-disk IDs from the CLI keep the
// `s<digits>` shape and remain valid.
func generateSubID() string {
	var b [2]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failure is essentially impossible on supported platforms;
		// degrade to a millisecond-only ID rather than panicking the binding.
		return fmt.Sprintf("s%d", time.Now().UnixMilli())
	}
	return fmt.Sprintf("s%d-%s", time.Now().UnixMilli(), hex.EncodeToString(b[:]))
}

// truncate clips s so the result is at most n bytes, appending "…" if cut.
// Mirrors cmd/itgray-cli/subs.go.truncate so the on-disk LastStatus shape
// stays consistent across the two surfaces.
func truncate(s string, n int) string {
	const ellipsis = "…"
	if len(s) <= n {
		return s
	}
	if n <= len(ellipsis) {
		return s[:n]
	}
	return s[:n-len(ellipsis)] + ellipsis
}
