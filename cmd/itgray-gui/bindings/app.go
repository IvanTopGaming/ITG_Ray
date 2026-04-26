// Package bindings hosts the Wails JS bindings. Each service is a thin
// translator: validate input → call internal/* package → translate to DTO.
package bindings

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/itg-team/itg-ray/cmd/itgray-gui/hub"
	"github.com/itg-team/itg-ray/internal/server"
	"github.com/itg-team/itg-ray/internal/subscription"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ServerStore is the read surface AppService needs from internal/server.
// The real internal/server package exposes free functions Load/Save rather
// than an interface; main.go adapts them via a tiny shim. The interface
// makes AppService unit-testable without touching the on-disk file.
type ServerStore interface {
	Load() ([]server.Server, error)
}

// SubStore is the read surface AppService needs from internal/subscription.
// internal/subscription.FileStore already implements Load(); the field-only
// signature here keeps the binding decoupled from the persistence type.
type SubStore interface {
	Load() ([]subscription.Stored, error)
}

// HelperProber returns the current helper-service state.
type HelperProber func() (state string)

// AppDeps groups the dependencies passed in from main.go.
type AppDeps struct {
	DataDir      string
	Hub          *hub.Hub
	Version      string
	BuildDate    string // set at link time via -ldflags -X (or "" for dev builds)
	ServerStore  ServerStore
	SubStore     SubStore
	HelperProber HelperProber // closure that returns helper state ("running"|"stopped"|"missing")
}

// AppService implements the App.* bindings (GetSnapshot, GetVersion, Quit).
type AppService struct{ d *AppDeps }

// NewAppService constructs a new AppService. AppDeps is taken by pointer so
// later tasks can grow it without re-introducing gocritic hugeParam
// suppressions on every binding constructor in cmd/itgray-gui/bindings/.
func NewAppService(d *AppDeps) *AppService {
	return &AppService{d: d}
}

// GetVersion returns the build version string.
func (a *AppService) GetVersion() string { return a.d.Version }

// Quit asks the Wails runtime to terminate the app. Idempotent at the
// runtime layer — calling on an already-stopping app is a no-op.
func (a *AppService) Quit(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	runtime.Quit(ctx)
}

// GetSnapshot collects the current app state into a Snapshot DTO.
func (a *AppService) GetSnapshot(_ context.Context) (hub.Snapshot, error) {
	servers, err := a.d.ServerStore.Load()
	if err != nil {
		return hub.Snapshot{}, fmt.Errorf("server.Load: %w", err)
	}
	subs, err := a.d.SubStore.Load()
	if err != nil {
		return hub.Snapshot{}, fmt.Errorf("sub.Load: %w", err)
	}
	return hub.Snapshot{
		Status:      hub.StatusIdle,
		Mode:        "auto",
		Speeds:      hub.SpeedSample{At: time.Now()},
		HelperState: a.probeHelper(),
		Servers:     toServerViews(servers, subOriginByID(subs)),
		Subs:        toSubViews(subs, serverCountBySource(servers)),
		Settings:    a.collectSettings(),
		Onboarded:   a.isOnboarded(),
		Version:     a.d.Version,
	}, nil
}

func (a *AppService) probeHelper() string {
	if a.d.HelperProber == nil {
		return "missing"
	}
	return a.d.HelperProber()
}

func (a *AppService) isOnboarded() bool {
	_, err := os.Stat(filepath.Join(a.d.DataDir, ".onboarded"))
	return err == nil
}

func toServerViews(in []server.Server, originByID map[string]string) []hub.ServerView {
	out := make([]hub.ServerView, 0, len(in))
	for i := range in {
		s := in[i]
		latency := 0
		if s.LatencyMS != nil {
			latency = *s.LatencyMS
		}
		origin := originByID[s.SourceID]
		if origin == "" {
			// Manual entries have empty SourceID; subscription entries with an
			// unknown SourceID also fall through to "manual" as a safe label.
			origin = "manual"
		}
		out = append(out, hub.ServerView{
			ID:        s.ID,
			Name:      s.Name,
			Country:   "", // TODO(plan-c-geoip): populate from server.Server once geo-IP enrichment lands
			Address:   hostPort(s.Vless.Address, s.Vless.Port),
			Transport: s.Vless.Transport.String(),
			Security:  s.Vless.Security.String(),
			LatencyMs: latency,
			Origin:    origin,
			Favorite:  s.Favorite,
			Tags:      append([]string(nil), s.Tags...),
		})
	}
	return out
}

func toSubViews(in []subscription.Stored, serverCount map[string]int) []hub.SubView {
	out := make([]hub.SubView, 0, len(in))
	for i := range in {
		s := in[i]
		out = append(out, hub.SubView{
			ID:              s.ID,
			Name:            s.Name,
			URL:             s.URL,
			UpdateInterval:  int(time.Duration(s.UpdateInterval) / time.Second),
			LastSyncAt:      s.LastSyncAt,
			LastSyncStatus:  s.LastStatus,
			LastSyncMessage: "", // internal/subscription does not yet persist a separate message
			ServerCount:     serverCount[s.ID],
		})
	}
	return out
}

// subOriginByID maps a subscription's ID to its display name (for ServerView.Origin).
// The empty-key entry is reserved for manual servers, which carry no SourceID;
// stored entries with empty IDs (corrupt files) are skipped so they cannot
// shadow the sentinel.
func subOriginByID(subs []subscription.Stored) map[string]string {
	m := make(map[string]string, len(subs)+1)
	for i := range subs {
		s := subs[i]
		if s.ID == "" {
			continue
		}
		name := s.Name
		if name == "" {
			name = s.ID
		}
		m[s.ID] = name
	}
	m[""] = "manual"
	return m
}

// serverCountBySource counts servers grouped by their SourceID (subscription).
func serverCountBySource(servers []server.Server) map[string]int {
	m := make(map[string]int)
	for i := range servers {
		m[servers[i].SourceID]++
	}
	return m
}

// hostPort formats address:port, bracketing IPv6 addresses per RFC 3986.
//
// TODO: drop in favour of an exported server.Server.Address() once internal/server
// promotes its private hostPort helper.
func hostPort(addr string, port uint16) string {
	if addr == "" && port == 0 {
		return ""
	}
	if strings.Contains(addr, ":") {
		return fmt.Sprintf("[%s]:%d", addr, port)
	}
	return fmt.Sprintf("%s:%d", addr, port)
}

// Settings collection is stubbed in C.T3 — values are filled by C.T12.
func (a *AppService) collectSettings() hub.SettingsView {
	return hub.SettingsView{
		General:       hub.GeneralSettings{Language: "auto", Theme: "dark", CloseToTray: true},
		Network:       hub.NetworkSettings{DefaultMode: "auto", TunCIDR: "198.18.0.1/15", TunName: "ITGRay-TUN", SocksPort: 1080, XrayPort: 1081},
		Subscriptions: hub.SubscriptionSettings{DefaultUpdateInterval: 3600, UserAgent: "ITG-Ray/0.1"},
		Notifications: hub.NotificationSettings{OnConnected: true, OnDisconnected: true, OnError: true},
		Debug:         hub.DebugSettings{LogLevel: "info"},
		About:         hub.AboutSettings{Version: a.d.Version, BuildDate: a.d.BuildDate},
		Security:      hub.SecuritySettings{Method: "Unencrypted", Available: false, Warning: "secret protection detection not yet wired"},
	}
}
