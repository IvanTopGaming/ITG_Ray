# Changelog

All notable changes to ITG Ray are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
adheres to [Semantic Versioning](https://semver.org/).

## [0.2.0-beta.1] - 2026-11-12

Statistics, per-rule servers, an instant routing mode switch, a full tray
menu, in-app notifications, and routing rules delivered by the subscription
provider.

### Added
- Statistics tab. Downloaded, uploaded, blocked and online time for today,
  7 days or 30 days, split into proxy and direct, with an hourly or daily
  chart, top apps and top rules. A live Connections view shows what each app
  is talking to, and "+ Rule" turns a connection into a routing rule placed
  above the rule that caught it. Only aggregates are saved, for 30 days;
  hosts and IPs stay in memory. Clear them in Settings → Logs.
- A proxy rule can go through a specific server, and several servers can be
  in use at once. The binding follows the server through subscription
  refreshes. If the server is gone or unusable, the rule says why.
- Routing mode switch: By rules, All via proxy or All direct, in the Routing
  header and the tray. Switching is instant and does not reconnect the tunnel.
- The tray menu shows the connection status and lets you connect or
  disconnect, pick a server, switch TUN or system proxy, switch the routing
  mode and update subscriptions. The tooltip shows live speed.
- Subscriptions can ship routing rules as an `itgray://rules/import/` link in
  a `Routing` response header or in the subscription body. They land in a
  separate group per subscription, below your own rules. You can switch the
  group off but not edit it. Its rules can only use servers from the same
  subscription. If such a server is down, the traffic is blocked rather than
  sent through another server. If the server is gone, the rule is skipped.
- Notifications in the bottom-right corner when the helper is missing,
  stopped or older than the app, with a button to install, start or update
  it.
- ITG Ray checks GitHub for a new release once per launch and shows a
  notification when one is out. You can skip a version, or turn the check
  off in Settings → About.
- Cancel a subscription sync by clicking its button again. "Sync all" turns
  into "Cancel all" while anything is syncing.
- Shared rule links keep the server name. An imported rule uses the server
  with that name when exactly one matches.

### Fixed
- In TUN mode, GeoIP and IP rules now match traffic to domains. Before,
  FakeIP answers hid the real address, so a rule like `geoip:ru → direct`
  never matched a domain.
- The reconnect notice no longer reverts routing rule edits when you close
  it. It asks first, and its button says what it does.
- The live connections table keeps its column widths while speeds update.
- The sidebar no longer runs the status and the version together.
- The helper's config sanitizer matches keys regardless of case and keeps
  the core's control API on 127.0.0.1 with a secret.

## [0.1.3-beta.1] - 2026-11-09

Engine updates to sing-box 1.14 and Xray 26.11, a responsive Logs tab, and
cancellable connections.

### Added
- Cancel a connection attempt by clicking the orb again while it says
  Connecting. Anything the attempt already set up is rolled back.
- Helper messages appear in the Logs tab, with their own source filter.

### Fixed
- The Logs tab no longer freezes the window during a long session. Before,
  it replayed the whole session log, and you could not switch to another
  tab until it finished. It now opens on the newest 2000 lines and adds new
  lines as they arrive. Cleared lines stay cleared when you reopen the tab.
- Heavy log output no longer delays or drops connection status updates.
- The sidebar shows the version with a single `v` (`v0.1.3-beta.1`, not
  `vv0.1.3-beta.1`).
- About shows Xray's release number instead of an internal module version.

### Changed
- sing-box 1.14.4 and Xray 26.11.9. Generated configs no longer use options
  that these releases deprecate.
- Electron 42, plus security updates to npm and Go dependencies.
- Building from source requires Go 1.27.

## [0.1.2-beta.1] - 2026-09-21

Subscription import improvements and recovery fixes for VPN connections.

### Added
- Import supported VLESS profiles from Xray JSON subscriptions, with an
  explanation when a feed contains no usable servers.
- Use subscription response headers to derive the provider's display name.
- Disconnect and retry incomplete cleanup from the error screen, even when
  the server list is empty.

### Fixed
- TLS connections preserve the original server name when the connection
  address is resolved to an IP and the profile has no explicit SNI.
- Background refresh uses the same HWID and User-Agent settings as manual
  refresh. CLI imports continue past an individual subscription failure.
- Subscriptions added after startup receive automatic refreshes. Removed
  subscriptions and old URLs cannot write stale responses back into the
  server list, and slow downloads no longer block local edits.
- Refresh preserves local server fields and distinguishes profiles sharing
  an endpoint but using different connection settings.
- UI, tray and notifications keep receiving events after the bridge restarts.
- Editing an active server's connection settings offers a reconnect action.
- Explicit disconnect clears a retained system proxy after a core crash.
  Helper requests honor cancellation, and partial cleanup failures remain
  visible and retryable across application restarts.
- Update checks handle local development versions and stalled responses.
- Routing import displays its action icon correctly.
- Packaged helper and bridge report the intended release version.

### Changed
- Update gRPC and frontend dependencies.

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
