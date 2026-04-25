# internal/helper/wintun

Thin Windows-only wrapper around the WinTUN driver via
`golang.zx2c4.com/wintun`. The user must place `wintun.dll` next to the
running executable (the helper or any test binary that imports this
package); the build script does this automatically into `dist/`.

`Create(name)` allocates an adapter that survives until `Close()` is called.
The returned LUID is the handle that the route-table API (Phase B7) uses to
scope additions to this adapter.

Sing-box attaches to the adapter by name (`"interface_name": "<name>"`) in
its `tun` inbound config, so once `Create` returns successfully, the user-
level main app can configure sing-box to use the same name.
