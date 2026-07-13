// Command itgray-bridge is the JSON-RPC backend for the Electron-based
// ITG Ray GUI. It reads requests from stdin, writes responses to stdout.
// Phase 0: app.ping. Phase 3.A: app.getSnapshot + onboarding.*.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/itg-team/itg-ray/cmd/itgray-bridge/bus"
	"github.com/itg-team/itg-ray/cmd/itgray-bridge/dispatcher"
	"github.com/itg-team/itg-ray/cmd/itgray-bridge/forwarder"
	"github.com/itg-team/itg-ray/cmd/itgray-bridge/handlers"
	"github.com/itg-team/itg-ray/internal/bindings"
	"github.com/itg-team/itg-ray/internal/chainctl"
	"github.com/itg-team/itg-ray/internal/config"
	"github.com/itg-team/itg-ray/internal/geo"
	"github.com/itg-team/itg-ray/internal/helper/runtime"
	"github.com/itg-team/itg-ray/internal/hub"
	"github.com/itg-team/itg-ray/internal/hwid"
	"github.com/itg-team/itg-ray/internal/logging"
	"github.com/itg-team/itg-ray/internal/logstream"
	"github.com/itg-team/itg-ray/internal/refresh"
	"github.com/itg-team/itg-ray/internal/rules"
	"github.com/itg-team/itg-ray/internal/server"
	"github.com/itg-team/itg-ray/internal/subscription"
	"github.com/itg-team/itg-ray/internal/sysproxy"
)

// BuildDate is overridden at build time via -ldflags "-X main.BuildDate=...".
var BuildDate = ""

// lockedWriter serializes writes to an underlying io.Writer so dispatcher
// responses and bus notifications never interleave on stdout. Required
// because both subsystems use independent json.Encoder instances and
// Windows pipe writes are not atomic.
type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

// serversFileStore adapts the package-level server.Load / server.Save
// to bindings.ServerStore. Mirrors the same struct in cmd/itgray-gui/main.go.
type serversFileStore struct{ path string }

func (s serversFileStore) Load() ([]server.Server, error)  { return server.Load(s.path) }
func (s serversFileStore) Save(list []server.Server) error { return server.Save(s.path, list) }

// serverStoreGetter adapts the bindings.ServerStore (Load/Save) shape to
// chainctl's per-id Get surface. Mirrors cmd/itgray-gui/main.go:246.
type serverStoreGetter struct{ ss serversFileStore }

// Get returns the server matching id, or (nil, nil) if not found. Errors
// from Load propagate. Mirrors cmd/itgray-gui/main.go:250.
func (g serverStoreGetter) Get(id string) (*server.Server, error) {
	list, err := g.ss.Load()
	if err != nil {
		return nil, err
	}
	for i := range list {
		if list[i].ID == id {
			return &list[i], nil
		}
	}
	return nil, nil
}

func defaultDataDir() string {
	if v := os.Getenv("ITGRAY_DATA_DIR"); v != "" {
		return v
	}
	cfg, err := os.UserConfigDir()
	if err != nil || cfg == "" {
		// Fallback: home-dir/.config-style path. Only triggered if both
		// XDG and HOME are unset; in that case the bridge will fail at
		// store load time and surface an error to the renderer.
		return filepath.Join(".", "ITG Ray")
	}
	return filepath.Join(cfg, "ITG Ray")
}

func main() {
	out := &lockedWriter{w: os.Stdout}

	dataDir := defaultDataDir()
	configPath := filepath.Join(dataDir, "config.json")
	serverStore := serversFileStore{path: filepath.Join(dataDir, "servers.json")}
	subStore := subscription.FileStore{Path: filepath.Join(dataDir, "subscriptions.json")}

	configStore := bindings.NewConfigStore(configPath, handlers.Version, BuildDate)
	onboardingSvc := bindings.NewOnboardingService(bindings.OnboardingDeps{DataDir: dataDir})

	settingsSvc := bindings.NewSettingsService(bindings.SettingsDeps{
		Store: configStore,
		// Hub is nil — Phase 4 wires hub.EventSettings forwarding through
		// bus.Emit. Without a Hub, Update succeeds but no bridge → main
		// notification is published. The renderer either uses the returned
		// SettingsView or calls Get to refresh.
		Hub: nil,
	})
	helperSvc := bindings.NewHelperService()

	// Bridge owns its own hub. ServersService and SubsService publish
	// servers.changed / sub.synced events into it; Phase 4 will subscribe
	// a forwarder that emits them as JSON-RPC notifications over stdout.
	h := hub.New()

	logLevelString := "info"
	if cfg, cerr := config.Load(configPath); cerr == nil && cfg.Debug.LogLevel != "" {
		logLevelString = cfg.Debug.LogLevel
	}
	logPath := filepath.Join(dataDir, "logs", "app.log")
	logWriter := io.MultiWriter(os.Stderr, logging.NewRotatingWriter(logPath, 5*1024*1024, 3))
	logLevel := logging.LevelFromString(logLevelString)
	logBuf := logstream.New(h, 2000)
	slog.SetDefault(slog.New(logstream.NewTapHandler(
		logging.NewHandler(logWriter, logLevel), logBuf)))

	// HWID + DeviceInfo for SubsService HWID-aware sync. Failure is
	// non-fatal: SubsService treats empty HWID as "HWID disabled".
	hwidValue, err := hwid.Get(dataDir)
	if err != nil {
		slog.Warn("hwid.Get returned error; using fallback value", "err", err)
	}
	deviceInfo := hwid.Info()

	subsSvc := bindings.NewSubsService(bindings.SubsDeps{
		SubStore:    subStore,
		ServerStore: serverStore,
		Hub:         h,
		SettingsView: func() hub.SettingsView {
			view, verr := configStore.View()
			if verr != nil {
				return hub.SettingsView{}
			}
			return view
		},
		HWID:       hwidValue,
		DeviceInfo: deviceInfo,
	})

	// helperProber wraps HelperService.Status: bindings.AppService uses
	// it to populate Snapshot.HelperState. SCM error or non-Windows stub
	// collapses to "missing" so the renderer wizard fires on a fresh box.
	helperProber := func() string {
		state, err := helperSvc.Status()
		if err != nil {
			return "missing"
		}
		return state
	}

	// chainctl needs a Get-by-id surface; wrap the existing serverStore
	// via serverStoreGetter (defined in main.go above). helperBootCtx is
	// a short-lived context used only by the Windows named-pipe dial
	// inside newHelperClient; the helper client itself does not capture
	// it, so we cancel inline. Mirrors cmd/itgray-gui/main.go:104-110.
	helperBootCtx, cancelHelperBoot := context.WithCancel(context.Background())
	helperClient := newHelperClient(helperBootCtx)
	cancelHelperBoot()

	// networkLoader reads the user's persisted config.json on every
	// Connect cycle — same closure as cmd/itgray-gui/main.go:121-126
	// so chainctl reads exactly what SettingsService.Update writes.
	networkLoader := func() (config.Network, error) {
		c, err := config.Load(configPath)
		if err != nil {
			return config.Network{}, err
		}
		return c.Network, nil
	}

	// killSwitchLoader mirrors networkLoader: reads the persisted
	// config.json so the poller's chain-drop handler branches on exactly
	// what SettingsService.Update wrote.
	killSwitchLoader := func() (config.KillSwitch, error) {
		c, err := config.Load(configPath)
		if err != nil {
			return config.KillSwitch{}, err
		}
		return c.KillSwitch, nil
	}

	ruleStore := rules.NewStore(dataDir)
	rulesSvc := bindings.NewRulesService(bindings.RulesDeps{
		Store: ruleStore,
		Hub:   h,
	})

	geoMgr := geo.NewManager(dataDir, func(done, total int64) {
		h.Publish(hub.Event{
			Name:    hub.EventGeoProgress,
			Payload: map[string]any{"done": done, "total": total},
		})
	})

	chainCtrl := chainctl.New(&chainctl.Deps{
		DataDir:      dataDir,
		ServerStore:  serverStoreGetter{ss: serverStore},
		Helper:       helperClient,
		Sysproxy:     sysproxy.New(),
		Hub:          h,
		BuildConfigs: buildConfigs(dataDir, configPath, ruleStore, geoMgr),
		Network:      networkLoader,
		KillSwitch:   killSwitchLoader,
	})

	// AppService now wired with live Chain, HelperProber, and the SOCKS
	// port GetPublicIP uses to route the HTTP request via xray. Quit is
	// a no-op on the bridge — the Electron main process owns shutdown via
	// its "app.quit" ipcMain handler.
	appSvc := bindings.NewAppService(&bindings.AppDeps{
		DataDir:       dataDir,
		Version:       handlers.Version,
		ServerStore:   serverStore,
		SubStore:      subStore,
		ConfigViewer:  configStore,
		HelperProber:  helperProber,
		Chain:         chainCtrl,
		XraySOCKSPort: defaultXrayPort,
	})

	runSvc := bindings.NewRunService(bindings.RunDeps{
		Chain: chainCtrl,
		Hub:   h,
	})

	serversSvc := bindings.NewServersService(bindings.ServersDeps{
		ServerStore:  serverStore,
		Hub:          h,
		ActiveServer: chainCtrl,
		SubStore:     subStore,
	})

	logPoller := logstream.NewPoller(logBuf, newLogReader(logHelperAddr))
	logSvc := bindings.NewLogService(bindings.LogDeps{
		Buffer:      logBuf,
		StartPoller: func() { logPoller.Start(context.Background()) },
		StopPoller:  logPoller.Stop,
		LogDir:      runtime.BasePath(),
	})

	d := dispatcher.New()

	quietMethods := map[string]bool{
		"app.getPublicIP": true,
		"app.getSnapshot": true,
		"helper.status":   true,
	}
	d.Observer = func(method string, params json.RawMessage, err error, dur time.Duration) {
		st, srv, mode := chainCtrl.Status()
		serverID := ""
		if srv != nil {
			serverID = srv.ID
		}
		args := logging.Redact(string(params))
		if len(args) > 200 {
			args = args[:200] + "…"
		}
		attrs := []any{
			slog.String("scope", "rpc"),
			slog.String("method", method),
			slog.String("state", string(st)),
			slog.String("server", serverID),
			slog.String("mode", string(mode)),
			slog.String("args", args),
			slog.Duration("dur", dur),
		}
		switch {
		case err != nil:
			slog.Error("rpc call failed", append(attrs, slog.String("err", err.Error()))...)
		case quietMethods[method]:
			slog.Debug("rpc call", attrs...)
		default:
			slog.Info("rpc call", attrs...)
		}
	}

	app := handlers.AppHandlers{Snap: appSvc}
	d.Register("app.ping", app.Ping)
	d.Register("app.getSnapshot", app.GetSnapshot)
	d.Register("app.getPublicIP", app.GetPublicIP)

	onboarding := handlers.OnboardingHandlers{Svc: onboardingSvc}
	d.Register("onboarding.getState", onboarding.GetState)
	d.Register("onboarding.complete", onboarding.Complete)
	d.Register("onboarding.skip", onboarding.Skip)

	settings := handlers.SettingsHandlers{Svc: settingsSvc}
	d.Register("settings.get", settings.Get)
	d.Register("settings.update", settings.Update)

	helper := handlers.HelperHandlers{Svc: helperSvc}
	d.Register("helper.status", helper.Status)
	d.Register("helper.install", helper.Install)
	d.Register("helper.start", helper.Start)
	d.Register("helper.stop", helper.Stop)
	d.Register("helper.restart", helper.Restart)
	d.Register("helper.reinstall", helper.Reinstall)
	d.Register("helper.installLinux", helper.InstallLinux)
	d.Register("helper.uninstallLinux", helper.UninstallLinux)

	servers := handlers.ServersHandlers{Svc: serversSvc}
	d.Register("servers.list", servers.List)
	d.Register("servers.add", servers.Add)
	d.Register("servers.edit", servers.Edit)
	d.Register("servers.remove", servers.Remove)
	d.Register("servers.toggleFavorite", servers.ToggleFavorite)
	d.Register("servers.testLatency", servers.TestLatency)

	subs := handlers.SubsHandlers{Svc: subsSvc}
	d.Register("subs.list", subs.List)
	d.Register("subs.add", subs.Add)
	d.Register("subs.edit", subs.Edit)
	d.Register("subs.remove", subs.Remove)
	d.Register("subs.syncOne", subs.SyncOne)
	d.Register("subs.syncAll", subs.SyncAll)

	rulesH := handlers.RulesHandlers{Svc: rulesSvc}
	d.Register("rules.list", rulesH.List)
	d.Register("rules.replaceAll", rulesH.ReplaceAll)
	d.Register("rules.groupAdd", rulesH.GroupAdd)
	d.Register("rules.groupEdit", rulesH.GroupEdit)
	d.Register("rules.groupRemove", rulesH.GroupRemove)
	d.Register("rules.ruleAdd", rulesH.RuleAdd)
	d.Register("rules.ruleEdit", rulesH.RuleEdit)
	d.Register("rules.ruleRemove", rulesH.RuleRemove)
	d.Register("rules.ruleToggle", rulesH.RuleToggle)
	d.Register("rules.ruleMove", rulesH.RuleMove)

	run := handlers.RunHandlers{Svc: runSvc}
	d.Register("run.connect", run.Connect)
	d.Register("run.disconnect", run.Disconnect)

	geoH := handlers.GeoHandlers{Svc: &geoService{
		mgr:        geoMgr,
		configPath: configPath,
		dataDir:    dataDir,
		ruleStore:  ruleStore,
	}}
	d.Register("geo.refresh", geoH.Refresh)

	logsH := handlers.LogsHandlers{Svc: logSvc}
	d.Register("logs.start", logsH.Start)
	d.Register("logs.stop", logsH.Stop)
	d.Register("logs.openFolder", logsH.OpenFolder)
	d.Register("logs.dirInfo", logsH.DirInfo)

	// Bus serializes outbound JSON-RPC notifications onto stdout.
	b := bus.New(out)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()

	// Start the hub→bus forwarder. Subscribes synchronously (so no
	// events are lost if the dispatcher delivers a request immediately),
	// then drains in a goroutine until ctx is cancelled. waitFwd blocks
	// until the goroutine has flushed any buffered events — call it after
	// d.Serve so in-flight notifications reach stdout before main exits.
	waitFwd := forwarder.Forwarder{Hub: h, Bus: b}.Start(ctx)

	// Start the background subscription auto-refresh loop (per-sub interval
	// from config, first tick within 30s of launch). OnSync republishes hub
	// events so the renderer's subs/servers stores refresh live. This was
	// previously wired only in itgray-cli, so the Electron app never
	// auto-updated subscriptions.
	refreshDriver := refresh.NewDriver(refresh.Config{
		Subs:        &subStore,
		ServersPath: serverStore.path,
		Log:         slog.Default(),
		OnSync: func(subID string) {
			h.Publish(hub.Event{Name: hub.EventSubSynced, Payload: map[string]any{"id": subID}})
			h.Publish(hub.Event{Name: hub.EventServersChanged, Payload: map[string]any{}})
		},
	})
	go func() { _ = refreshDriver.Run(ctx) }()

	// Announce bridge readiness. Renderer gates mutation buttons on
	// this state; "restarting"/"failed" are emitted by Electron main
	// (it owns the spawn lifecycle), not by the bridge itself.
	b.Emit("bridge.state", map[string]string{"state": "running"})

	// Adopt a chain that the helper kept alive across our restart so
	// the renderer's first GetSnapshot/vpn:status reflects connected
	// state instead of forcing the user to reconnect. Safe no-op when
	// the helper reports idle.
	chainCtrl.Reconcile(ctx)

	if err := d.Serve(ctx, os.Stdin, out); err != nil {
		fmt.Fprintln(os.Stderr, "bridge: serve:", err)
		os.Exit(1)
	}

	// Cancel context so the forwarder goroutine exits its drain loop,
	// then wait for it to flush any buffered hub events before main
	// returns (avoids silent drop of notifications queued mid-request).
	cancel()
	waitFwd()
}
