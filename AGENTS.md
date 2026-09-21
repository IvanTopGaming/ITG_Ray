# Repository Guidelines

## Project Structure & Module Organization

ITG Ray combines Go services with an Electron/React VPN interface. `cmd/itgray-cli`, `cmd/itgray-bridge`, and `cmd/itgray-helper` provide executable entry points; `internal/` contains shared domain logic and platform adapters. Electron main/preload code lives in `cmd/itgray-electron/src/`; React pages, components, stores, and translations live in `cmd/itgray-electron/frontend/src/`. Assets are in `cmd/itgray-electron/resources/` and `internal/icons/`. Tests sit beside implementation files; fixtures use `testdata/`. Packaging lives in `scripts/` and `packaging/`.

## Build, Test, and Development Commands

Use Go 1.26+ and Node 22+. From the repository root:

- `go test -race -count=1 -timeout 120s ./...` — run Go tests as CI does.
- `golangci-lint run` — run the Go lint checks.
- `bash scripts/build-linux.sh` — build Linux binaries and AppImage.
- `bash scripts/build-windows.sh` — cross-build Windows binaries and installer; requires Wine.
- `bash scripts/check-codegen.sh` — regenerate bridge types and detect uncommitted differences.

From `cmd/itgray-electron/`:

- `npm ci && npm --prefix frontend ci` — install both dependency sets.
- `npm run dev` — start bridge, Vite, TypeScript watchers, and Electron; requires Air on PATH.
- `npm run build` — compile main, preload, frontend, and Go bridge.
- `npm run build:main && node --test dist-main/main/*.test.js` — compile and test Electron main code.
- `npm --prefix frontend test` — run frontend tests.

## Coding Style & Naming Conventions

Format Go with `gofmt`; use platform suffixes such as `_linux.go` and `_windows.go`. TypeScript uses two-space indentation and semicolons; follow each file's quote style. Use PascalCase React component filenames and camelCase functions/stores. Keep new code comment-free. Update both `en.json` and `ru.json` for UI strings. Change bridge schemas in `internal/bridge/protocol/`, then run `go generate ./...`; commit generated output.

## Testing Guidelines

Use Go's testing package and Testify in `*_test.go`; frontend tests use Vitest, jsdom, and Testing Library in `*.test.ts(x)`. Electron main tests use `node:test`. Add regression coverage for changed behavior; no numeric coverage threshold is configured. Run CLI smoke tests with `ITGRAY_E2E=1 go test -tags e2e -race -timeout 120s ./cmd/itgray-cli/...`.

## Commit & Pull Request Guidelines

Follow history's Conventional Commits: `fix(routing): ...`, `chore(deps): ...`, or `docs: ...`. Create a separate branch and open a PR against `main`. Explain behavior changes, link relevant issues, list validation, and include screenshots for UI changes. Never add coauthor trailers.

## Security & Configuration

Keep privileged tunnel, route, and DNS operations in the helper. Use demo endpoints in fixtures/screenshots; exclude subscription credentials and redact sensitive logs.
