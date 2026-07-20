# ITG Ray

[Русский](README.ru.md)

ITG Ray is a desktop VLESS VPN client for Linux and Windows built on
[sing-box](https://github.com/SagerNet/sing-box) and
[Xray-core](https://github.com/XTLS/Xray-core), with an Electron UI and a
privileged helper daemon so the GUI itself never runs as root.

![Main window](docs/screenshots/main.png)

## Features

- **VLESS + subscriptions** — add servers via `vless://` links or
  subscription URLs, auto-refresh included.
- **TUN mode** — system-wide tunneling through a virtual interface with
  FakeIP DNS; local SOCKS (`127.0.0.1:1080`) and HTTP (`127.0.0.1:8888`)
  inbounds stay reachable.
- **System proxy mode** — lighter alternative that just sets the OS proxy.
- **Routing rules** — drag-and-drop rule editor (domains, IPs, GeoIP/Geosite)
  with per-rule proxy/direct/block actions.
- **Observability** — live core logs, traffic stats and latency probing.
- **Bilingual UI** — English and Russian.

## Install

### Linux

- **Arch Linux (AUR):** `yay -S itgray-bin`, then
  `sudo systemctl enable --now itgray-helper.service`
- **AppImage:** grab `ITGRay-<version>.AppImage` from
  [Releases](https://github.com/IvanTopGaming/ITG_Ray/releases), make it
  executable and run. TUN mode requires the bundled helper to run as a
  systemd service — the AUR/tarball install is recommended for TUN.

### Windows

Download and run `ITGRay-Setup-<version>.exe` from
[Releases](https://github.com/IvanTopGaming/ITG_Ray/releases). The
installer registers the helper service and ships the Wintun driver.

## Build from source

Prerequisites: Go 1.23+, Node 22+, npm. Cross-building the Windows installer additionally requires wine.

```bash
git clone https://github.com/IvanTopGaming/ITG_Ray
cd ITG_Ray
(cd cmd/itgray-electron && npm ci && cd frontend && npm ci)
bash scripts/build-linux.sh     # AppImage + binaries in dist/
bash scripts/build-windows.sh   # NSIS installer (cross-compiled from Linux)
```

## Architecture

```
Electron GUI ──IPC──▶ bridge ──HTTP/unix──▶ itgray-helper (root, systemd/service)
                                                  │
                                          spawns sing-box / xray
```

The helper owns everything privileged (TUN interface, routes, DNS); the GUI
talks to it through a local API and can restart independently — an active
tunnel survives GUI restarts.

## License

GPL-3.0 — see [LICENSE](LICENSE). Bundled third-party components are listed
in [docs/THIRD_PARTY.md](docs/THIRD_PARTY.md).
