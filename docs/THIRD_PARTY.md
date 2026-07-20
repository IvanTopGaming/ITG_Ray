# Third-party components

ITG Ray bundles or builds the following third-party software. Each component
remains under its own license; full texts are available at the linked
upstream repositories.

| Component | License | Role | Source |
| --- | --- | --- | --- |
| sing-box | GPL-3.0-or-later | Proxy core (TUN, routing, DNS), spawned as a separate process | https://github.com/SagerNet/sing-box |
| Xray-core | MPL-2.0 | VLESS/XTLS proxy core, spawned as a separate process | https://github.com/XTLS/Xray-core |
| Wintun | Prebuilt Binaries License (see `third_party/wintun/LICENSE.txt`) | Windows TUN driver, shipped as `wintun.dll` | https://www.wintun.net |
| Electron | MIT | Desktop GUI shell | https://github.com/electron/electron |

Go module dependencies are listed in `go.mod`; npm dependencies in
`cmd/itgray-electron/package.json` and
`cmd/itgray-electron/frontend/package.json`. The sing-box and Xray-core
binaries are compiled unmodified from the upstream module sources pinned in
`go.mod` (see `scripts/build-linux.sh` / `scripts/build-windows.sh` for the
exact tags and ldflags).
