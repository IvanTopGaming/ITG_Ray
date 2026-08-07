# Changelog

All notable changes to ITG Ray are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
adheres to [Semantic Versioning](https://semver.org/).

## [0.1.1-beta.1] - 2026-08-07

Bug-fix beta, plus a jump to Electron 41.

### Fixed
- Incoming LAN connections no longer time out while the tunnel is up. TUN's
  `auto_route` claimed the private ranges too, so replies from services on
  this host (SSH, samba, a web UI) were pulled into the tunnel and dropped;
  RFC1918 and IPv4 link-local are now excluded from the tunnel's routes.
- Subscriptions refresh in parallel instead of queueing behind each other —
  the bridge served one request at a time, so a 30s fetch blocked everything
  behind it, the UI included.
- The per-subscription User-Agent is actually sent. It was dropped between
  the form and the request, so providers always saw the default one.
- Latency is no longer measured over and over. A server that never answers
  kept the auto-probe re-triggering itself, worst right after disconnecting.
- Connecting and Disconnecting are visible again, instead of the UI jumping
  straight to the end state.
- Auto-connect fires once per launch. Closing the window to the tray and
  reopening it re-ran it, which is not a launch.
- The Logs tab opens on the newest lines rather than shipping the whole
  buffer to the renderer.
- Saving a routing rule takes one round-trip instead of three, and a first
  load that fails is retried rather than leaving the tab stuck on the error.
- `servers.json` could be corrupted by concurrent writes: overlapping saves
  fought over one temp file, and a sync could write back a list that
  predated another sync, dropping its servers.
- Packaging: Electron's binary is fetched again under npm 12 (install
  scripts are blocked by default), and the window is matched to its desktop
  entry (`StartupWMClass`), so icons and window rules apply.

### Changed
- Electron 31 → 41 (Chromium 126 → 146), electron-builder 26.15.3.
- Frontend toolchain: vite 5 → 8, vitest 2 → 4.
- Security updates: `ws` (high), `undici`, `brace-expansion`, `form-data`,
  `js-yaml`, `grpc`, `postcss`, `react-router-dom`.

## [0.1.0-beta.1] - 2026-07-20

First public beta.

### Added
- VLESS client with subscription support (sing-box and Xray cores).
- TUN mode with a privileged helper (systemd service on Linux, Windows
  service on Windows) — the GUI never runs as root.
- Local SOCKS (`:1080`) and HTTP (`:8888`) proxy inbounds, available in
  both system-proxy and TUN modes.
- Routing rules editor with drag-and-drop ordering and rollback on
  reconnect-dismiss.
- Live logs page, traffic stats, latency probing.
- System tray with connection state, English and Russian UI.
- Linux AppImage, Arch package (`itgray-bin`), Windows NSIS installer.
