// Package protocol declares the JSON-RPC method surface of itgray-bridge
// as Go interfaces. A codegen tool (internal/bridge/codegen) reads these
// declarations via go/types reflection and emits a TypeScript module
// (cmd/itgray-electron/src/shared/protocol.ts) that the Electron preload
// and renderer consume.
//
// Edit this package, then run `go generate ./internal/bridge/protocol/`
// (or `go generate ./...`) to regenerate the TS file. CI verifies no
// drift via scripts/check-codegen.sh.
package protocol

//go:generate go run ../codegen
