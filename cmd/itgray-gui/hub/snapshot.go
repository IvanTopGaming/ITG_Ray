package hub

import "time"

// ChainStatus is the high-level VPN state shown in the Hero card.
type ChainStatus string

// ChainStatus values surfaced to the frontend.
const (
	StatusIdle          ChainStatus = "idle"
	StatusConnecting    ChainStatus = "connecting"
	StatusConnected     ChainStatus = "connected"
	StatusDisconnecting ChainStatus = "disconnecting"
	StatusError         ChainStatus = "error"
)

// SpeedSample is the most recent 1-second up/down byte counts.
type SpeedSample struct {
	UpBps   uint64    `json:"upBps"`
	DownBps uint64    `json:"downBps"`
	At      time.Time `json:"at"`
}

// ServerView is the read-only DTO for one server.
type ServerView struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Country   string   `json:"country"`   // ISO-2 code or "" if unknown
	Address   string   `json:"address"`   // host:port
	Transport string   `json:"transport"` // "tcp" | "ws" | "grpc" | ...
	Security  string   `json:"security"`  // "tls" | "reality" | "none"
	LatencyMs int      `json:"latencyMs"` // 0 = unknown / never probed
	Origin    string   `json:"origin"`    // subscription name or "manual"
	Favorite  bool     `json:"favorite"`
	Tags      []string `json:"tags,omitempty"`
}

// SubView is the read-only DTO for one subscription.
type SubView struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	URL             string    `json:"url"`            // raw — frontend masks
	UpdateInterval  int       `json:"updateInterval"` // seconds
	LastSyncAt      time.Time `json:"lastSyncAt"`     // zero = never synced
	LastSyncStatus  string    `json:"lastSyncStatus"` // "OK" | "ERROR" | "PARTIAL" | ""
	LastSyncMessage string    `json:"lastSyncMessage,omitempty"`
	ServerCount     int       `json:"serverCount"`
}

// SettingsView is the flat-by-section settings shape.
type SettingsView struct {
	General       GeneralSettings      `json:"general"`
	Network       NetworkSettings      `json:"network"`
	Subscriptions SubscriptionSettings `json:"subscriptions"`
	Notifications NotificationSettings `json:"notifications"`
	Debug         DebugSettings        `json:"debug"`
	About         AboutSettings        `json:"about"`
	Security      SecuritySettings     `json:"security"`
}

// GeneralSettings holds top-level UX preferences.
type GeneralSettings struct {
	Language       string `json:"language"` // "en" | "ru" | "auto"
	Theme          string `json:"theme"`    // "dark" (only)
	Autostart      bool   `json:"autostart"`
	CloseToTray    bool   `json:"closeToTray"`
	StartMinimized bool   `json:"startMinimized"`
}

// NetworkSettings holds proxy / TUN routing knobs.
type NetworkSettings struct {
	DefaultMode string `json:"defaultMode"` // "tun" | "sysproxy" | "auto"
	TunCIDR     string `json:"tunCidr"`     // "198.18.0.1/15"
	TunName     string `json:"tunName"`     // "ITGRay-TUN"
	SocksPort   int    `json:"socksPort"`   // sysproxy mode local port
	XrayPort    int    `json:"xrayPort"`    // internal xray socks port
}

// SubscriptionSettings holds per-app subscription defaults.
type SubscriptionSettings struct {
	DefaultUpdateInterval int    `json:"defaultUpdateInterval"` // seconds
	UserAgent             string `json:"userAgent"`
}

// NotificationSettings toggles desktop notification triggers.
type NotificationSettings struct {
	OnConnected    bool `json:"onConnected"`
	OnDisconnected bool `json:"onDisconnected"`
	OnError        bool `json:"onError"`
	OnSubSynced    bool `json:"onSubSynced"`
}

// DebugSettings holds developer-facing toggles.
type DebugSettings struct {
	LogLevel string `json:"logLevel"` // "debug" | "info" | "warn" | "error"
	OpenLogs bool   `json:"-"`        // not persisted; UI-only action
}

// AboutSettings is build-info metadata for the About panel.
type AboutSettings struct {
	Version   string `json:"version"`
	GitRev    string `json:"gitRev"`
	BuildDate string `json:"buildDate"`
}

// SecuritySettings is the read-only secret-protection summary.
type SecuritySettings struct {
	// Read-only in v0.1: detected secret-protection method.
	Method    string `json:"method"` // "DPAPI" | "Keychain" | "SecretService" | "Unencrypted"
	Available bool   `json:"available"`
	Warning   string `json:"warning,omitempty"`
}

// Snapshot is the single DTO returned by App.GetSnapshot — the frontend's
// initial state. After mounting, deltas arrive via hub events.
type Snapshot struct {
	Status        ChainStatus  `json:"status"`
	CurrentServer *ServerView  `json:"currentServer,omitempty"`
	Mode          string       `json:"mode"`
	Speeds        SpeedSample  `json:"speeds"`
	HelperState   string       `json:"helperState"` // "running" | "stopped" | "missing"
	Servers       []ServerView `json:"servers"`
	Subs          []SubView    `json:"subs"`
	Settings      SettingsView `json:"settings"`
	Onboarded     bool         `json:"onboarded"`
	Version       string       `json:"version"`
}
