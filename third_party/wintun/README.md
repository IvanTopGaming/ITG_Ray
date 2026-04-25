# wintun.dll (vendored)

Source: https://www.wintun.net/builds/wintun-0.14.1.zip
Version: 0.14.1
Architecture: amd64 (Windows x86_64)

Provenance hashes:
- ZIP SHA-256: `07c256185d6ee3652e09fa55c0b673e2624b565e02c4b9091c79ca7d2f24ef51`
  (published by the vendor on https://www.wintun.net/ — verify before re-vendoring)
- DLL SHA-256: `e5da8447dc2c320edc0fc52fa01885c103de8c118481f683643cacc3220dafce`
  (computed locally from `wintun/bin/amd64/wintun.dll` inside the verified ZIP)
- DLL size: 427552 bytes

License: see LICENSE.txt (WireGuard project — LGPL-2.1-or-later for dynamically-linked usage)

ITG Ray Helper (`cmd/itgray-helper`) loads this DLL at runtime via
`golang.zx2c4.com/wintun`, which expects `wintun.dll` to sit next to the
helper executable. The NSIS installer (Plan D) places it there; for dev
builds the build script (`scripts/build-windows.sh`) copies it next to the
freshly-built `ITGRayHelper.exe`.

Update procedure:
1. Download the new release ZIP from https://www.wintun.net/.
2. Verify the ZIP SHA-256 against the value published on the wintun.net
   homepage. Do not proceed if it does not match.
3. Replace `wintun.dll`, recompute and update both the ZIP SHA-256 and
   DLL SHA-256 lines above (and the size line).
4. Re-run the VM smoke test.
