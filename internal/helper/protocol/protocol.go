// Package protocol defines the wire types exchanged between the user-level
// itgray-cli client and the privileged itgray-helper server over the SID-gated
// named pipe.
package protocol

import "encoding/json"

// Op identifies one Helper operation.
type Op string

// Op values — keep alphabetical so the test suite is easy to extend.
const (
	OpDnsRestore    Op = "DnsRestore"
	OpDnsSet        Op = "DnsSet"
	OpReadLogs      Op = "ReadLogs"
	OpRouteAdd      Op = "RouteAdd"
	OpRouteRemove   Op = "RouteRemove"
	OpRouteRestore  Op = "RouteRestore"
	OpRouteSnapshot Op = "RouteSnapshot"
	OpServiceStatus Op = "ServiceStatus"
	OpStartChain    Op = "StartChain"
	OpStopChain     Op = "StopChain"
	OpTunCreate     Op = "TunCreate"
	OpTunDestroy    Op = "TunDestroy"
)

// String returns the canonical name of the operation.
func (o Op) String() string { return string(o) }

// Request is the framed unit sent from client to server.
type Request struct {
	ID   uint64          `json:"id"`
	Op   Op              `json:"op"`
	Args json.RawMessage `json:"args,omitempty"`
}

// Response is the framed unit sent from server to client.
type Response struct {
	ID     uint64          `json:"id"`
	OK     bool            `json:"ok"`
	Error  string          `json:"error,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
}

// ReadLogsArgs selects a byte range of an engine log. Tail, when positive and
// Offset is zero, starts the read Tail bytes before the end of the file
// instead of at its beginning, so a reader attaching to a long-running session
// gets the newest lines without draining the whole history first. The first
// line of such a read is usually partial; the caller detects that from the
// returned Offset (start = Offset - len(Data) > 0) and drops it.
type ReadLogsArgs struct {
	Name   string `json:"name"`
	Offset int64  `json:"offset"`
	Tail   int64  `json:"tail,omitempty"`
}

type ReadLogsResult struct {
	Data      []byte `json:"data"`
	Offset    int64  `json:"offset"`
	Truncated bool   `json:"truncated"`
}

// NewOK constructs a successful response.
func NewOK(id uint64, result json.RawMessage) Response {
	return Response{ID: id, OK: true, Result: result}
}

// NewError constructs an error response.
func NewError(id uint64, msg string) Response {
	return Response{ID: id, OK: false, Error: msg}
}
