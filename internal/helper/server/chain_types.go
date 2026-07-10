package server

import "encoding/json"

// StartChainArgs is the JSON payload of OpStartChain. Shared across
// platforms so both the Windows and Linux helpers and their bridge-side
// adapters marshal an identical wire contract.
type StartChainArgs struct {
	SingboxConfig json.RawMessage `json:"singbox_config"`
	XrayConfig    json.RawMessage `json:"xray_config"`
	ServerHost    string          `json:"server_host"`
	ServerPort    int             `json:"server_port"`
	TunName       string          `json:"tun_name"`
	Mode          string          `json:"mode,omitempty"`
	DnsAlias      string          `json:"dns_alias,omitempty"`
	DnsServers    []string        `json:"dns_servers,omitempty"`
}

// StartChainResult is what OpStartChain returns on success.
type StartChainResult struct {
	SessionID  string `json:"session_id"`
	TunLUID    uint64 `json:"tun_luid"`
	SingboxPid int    `json:"singbox_pid"`
	XrayPid    int    `json:"xray_pid"`
}

// StopChainArgs is the JSON payload of OpStopChain.
type StopChainArgs struct {
	SessionID string `json:"session_id"`
}
