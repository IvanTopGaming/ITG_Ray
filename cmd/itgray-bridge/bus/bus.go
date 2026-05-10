// Package bus serializes bridge → main JSON-RPC notifications onto a
// single io.Writer (typically os.Stdout). All exported methods are safe
// for concurrent use.
package bus

import (
	"encoding/json"
	"io"
	"sync"
)

// Bus emits JSON-RPC notifications with an "event:" prefixed method name.
type Bus struct {
	mu  sync.Mutex
	enc *json.Encoder
	w   io.Writer
}

// New returns a Bus that writes to w. The Encoder is created once and reused.
func New(w io.Writer) *Bus {
	return &Bus{enc: json.NewEncoder(w), w: w}
}

// Emit writes one notification line. payload may be nil. Errors are dropped:
// on the bridge stdout transport, a write error means the parent process is
// gone and the bridge will exit shortly anyway.
func (b *Bus) Emit(topic string, payload any) {
	notif := struct {
		JSONRPC string `json:"jsonrpc"`
		Method  string `json:"method"`
		Params  any    `json:"params,omitempty"`
	}{
		JSONRPC: "2.0",
		Method:  "event:" + topic,
		Params:  payload,
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	_ = b.enc.Encode(notif) // Encode appends a newline
}
