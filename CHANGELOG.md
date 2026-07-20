# Changelog

All notable changes to ITG Ray are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
adheres to [Semantic Versioning](https://semver.org/).

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
