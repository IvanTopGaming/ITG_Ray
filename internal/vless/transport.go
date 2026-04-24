// Package vless parses and serializes VLESS URIs (vless:// scheme).
package vless

// Transport identifies the network transport layer used by a VLESS connection.
type Transport int

// Supported transport types. Zero value is intentionally invalid/unspecified.
const (
	TransportTCP Transport = iota + 1
	TransportWS
	TransportGRPC
	TransportHTTPUpgrade
	TransportXHTTP
	TransportMKCP
	TransportQUIC
)

func (t Transport) String() string {
	switch t {
	case TransportTCP:
		return "tcp"
	case TransportWS:
		return "ws"
	case TransportGRPC:
		return "grpc"
	case TransportHTTPUpgrade:
		return "httpupgrade"
	case TransportXHTTP:
		return "xhttp"
	case TransportMKCP:
		return "mkcp"
	case TransportQUIC:
		return "quic"
	}
	return ""
}

// ParseTransport converts a string to a Transport value.
// An empty string is treated as "tcp". Returns false for unrecognized values.
func ParseTransport(s string) (Transport, bool) {
	switch s {
	case "", "tcp":
		return TransportTCP, true
	case "ws":
		return TransportWS, true
	case "grpc":
		return TransportGRPC, true
	case "httpupgrade":
		return TransportHTTPUpgrade, true
	case "xhttp":
		return TransportXHTTP, true
	case "mkcp", "kcp":
		return TransportMKCP, true
	case "quic":
		return TransportQUIC, true
	}
	return 0, false
}

// Security identifies the TLS/security layer used by a VLESS connection.
type Security int

// Supported security types. Zero value is intentionally invalid/unspecified.
const (
	SecurityNone Security = iota + 1
	SecurityTLS
	SecurityReality
)

func (s Security) String() string {
	switch s {
	case SecurityNone:
		return "none"
	case SecurityTLS:
		return "tls"
	case SecurityReality:
		return "reality"
	}
	return ""
}

// ParseSecurity converts a string to a Security value.
// An empty string is treated as "none". Returns false for unrecognized values.
func ParseSecurity(s string) (Security, bool) {
	switch s {
	case "", "none":
		return SecurityNone, true
	case "tls":
		return SecurityTLS, true
	case "reality":
		return SecurityReality, true
	}
	return 0, false
}
