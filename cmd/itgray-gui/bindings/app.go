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

	"github.com/itg-team/itg-ray/cmd/itgray-gui/chainctl"
	"github.com/itg-team/itg-ray/cmd/itgray-gui/hub"
	"github.com/itg-team/itg-ray/internal/config"
	"github.com/itg-team/itg-ray/internal/server"
	"github.com/itg-team/itg-ray/internal/subscription"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// AppCtxFunc returns the Wails app context captured by App.Startup. The
// constructor takes a closure rather than the ctx itself because the
// context only becomes available after Wails calls Startup, while the
// service is constructed earlier in main.go.
type AppCtxFunc func() context.Context

// ServerStore is the read/write surface the binding services need from
// internal/server. The real internal/server package exposes free functions
// Load/Save rather than an interface; main.go adapts them via a tiny shim.
// The interface makes the bindings unit-testable without touching the
// on-disk file. Save is required by ServersService.ToggleFavorite (and
// future mutators); AppService only consumes Load.
type ServerStore interface {
	Load() ([]server.Server, error)
	Save([]server.Server) error
}

// SubStore is the read/write surface the binding services need from
// internal/subscription. AppService consumes only Load(); SubsService.Add /
// Remove / SyncOne use Save() (full-list rewrite, atomic) plus UpdateMeta()
// for post-sync timestamp/status writes. internal/subscription.FileStore
// implements all three; the interface keeps tests insulated from disk I/O.
//
// Concurrency note: FileStore.UpdateMeta acquires a per-path mutex around its
// load-mutate-save cycle, but FileStore.Save does NOT — it relies on the
// atomic tmp+rename only. Today the JS event loop serialises Add / Remove /
// SyncOne calls so the asymmetry is latent. Once tray writes (C.T13) land,
// FileStore.Save should take the same per-path lock or the binding-level
// mutators must hold their own.
type SubStore interface {
	Load() ([]subscription.Stored, error)
	Save([]subscription.Stored) error
	UpdateMeta(id string, at time.Time, status, message string, ui *subscription.Userinfo) error
}

// HelperProber returns the current helper-service state.
type HelperProber func() (state string)

// ChainStatuser is the surface AppService needs from chainctl.Controller.
// chainctl.Controller satisfies it directly via its Status() method.
type ChainStatuser interface {
	Status() (hub.ChainStatus, *server.Server, chainctl.Mode)
}

// ConfigViewer is the surface AppService needs from ConfigStore.
// bindings.ConfigStore satisfies it directly via its View() method.
type ConfigViewer interface {
	View() (hub.SettingsView, error)
}

// NetworkLoader is the closure AppService uses to read the persisted
// Network section (for GetPublicIP's SOCKS5 dialer in sysproxy mode).
// main.go provides a closure over config.Load(configPath); tests mock it.
type NetworkLoader func() (config.Network, error)

// AppDeps groups the dependencies passed in from main.go.
type AppDeps struct {
	DataDir      string
	Hub          *hub.Hub
	Version      string
	ServerStore  ServerStore
	SubStore     SubStore
	HelperProber HelperProber // closure that returns helper state ("running"|"stopped"|"missing")
	// AppCtx returns the Wails app context (App.ctx, set in Startup). Used
	// by Quit, which must call runtime.Quit with the app's ctx — passing
	// context.Background() makes the runtime no-op. Nil is tolerated and
	// causes Quit to fall back to context.Background() (test code path).
	AppCtx AppCtxFunc
	// Chain is the source of live status/server/mode for GetSnapshot. Nil
	// is tolerated (yields StatusIdle / Mode("tun")); production wiring
	// guarantees non-nil.
	Chain         ChainStatuser
	ConfigViewer  ConfigViewer  // NEW: source of SettingsView (replaces hardcoded collectSettings)
	NetworkLoader NetworkLoader // NEW: source of Network for GetPublicIP sysproxy dialer
}

// AppService implements the App.* bindings (GetSnapshot, GetVersion, Quit).
type AppService struct {
	d       *AppDeps
	ipCache publicIPCache
}

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
//
// Wails v2.11 does not auto-inject a ctx into bound service methods (only
// the main App struct), so Quit takes no JS-visible args. The Wails app
// ctx is sourced from AppDeps.AppCtx (a closure into App.ctx, set during
// Startup); when nil (unit tests), runtime.Quit is invoked with
// context.Background() and is a no-op.
func (a *AppService) Quit() {
	ctx := context.Background()
	if a.d.AppCtx != nil {
		if c := a.d.AppCtx(); c != nil {
			ctx = c
		}
	}
	if ctx.Err() != nil {
		return
	}
	runtime.Quit(ctx)
}

// GetSnapshot collects the current app state into a Snapshot DTO.
func (a *AppService) GetSnapshot() (hub.Snapshot, error) {
	servers, err := a.d.ServerStore.Load()
	if err != nil {
		return hub.Snapshot{}, fmt.Errorf("server.Load: %w", err)
	}
	subs, err := a.d.SubStore.Load()
	if err != nil {
		return hub.Snapshot{}, fmt.Errorf("sub.Load: %w", err)
	}
	settings, err := a.d.ConfigViewer.View()
	if err != nil {
		return hub.Snapshot{}, fmt.Errorf("settings.View: %w", err)
	}

	st := hub.StatusIdle
	mode := chainctl.Mode("tun")
	var current *hub.ServerView
	if a.d.Chain != nil {
		var srv *server.Server
		st, srv, mode = a.d.Chain.Status()
		if mode == "" {
			mode = chainctl.ModeTUN
		}
		if srv != nil {
			views := toServerViews([]server.Server{*srv}, subOriginByID(subs))
			view := views[0]
			current = &view
		}
	}

	return hub.Snapshot{
		Status:        st,
		CurrentServer: current,
		Mode:          string(mode),
		Speeds:        hub.SpeedSample{At: time.Now()},
		HelperState:   a.probeHelper(),
		Servers:       toServerViews(servers, subOriginByID(subs)),
		Subs:          toSubViews(subs, serverCountBySource(servers)),
		Settings:      settings,
		Onboarded:     a.isOnboarded(),
		Version:       a.d.Version,
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
			LastSyncMessage: s.LastMessage,
			ServerCount:     serverCount[s.ID],
			Upload:          s.Upload,
			Download:        s.Download,
			Total:           s.Total,
			Expire:          s.Expire,
			UserAgent:       s.UserAgent,
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
