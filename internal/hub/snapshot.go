package hub

import (
	"time"

	"github.com/itg-team/itg-ray/internal/rules"
)

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
	URI       string   `json:"uri"` // canonical vless:// URI form, derived via vless.Config.URL()
}

// SubView is the read-only DTO for one subscription.
type SubView struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	URL             string     `json:"url"`            // raw — frontend masks
	UpdateInterval  int        `json:"updateInterval"` // seconds
	LastSyncAt      time.Time  `json:"lastSyncAt"`     // zero = never synced
	LastSyncStatus  string     `json:"lastSyncStatus"` // "ok" | "error" | ""
	LastSyncMessage string     `json:"lastSyncMessage,omitempty"`
	ServerCount     int        `json:"serverCount"`
	Upload          int64      `json:"upload,omitempty"`
	Download        int64      `json:"download,omitempty"`
	Total           int64      `json:"total,omitempty"`
	Expire          *time.Time `json:"expire,omitempty"`
	UserAgent       string     `json:"userAgent,omitempty"` // per-sub UA override; "" = inherit from Settings
}

// SettingsView is the flat-by-section settings shape.
type SettingsView struct {
	General       GeneralSettings      `json:"general"`
	Network       NetworkSettings      `json:"network"`
	KillSwitch    KillSwitchSettings   `json:"killSwitch"`
	Subscriptions SubscriptionSettings `json:"subscriptions"`
	Notifications NotificationSettings `json:"notifications"`
	Debug         DebugSettings        `json:"debug"`
	About         AboutSettings        `json:"about"`
	Security      SecuritySettings     `json:"security"`
}

// GeneralSettings holds top-level UX preferences.
type GeneralSettings struct {
	Language       string `json:"language"` // "en" | "ru" | "auto"
	Autostart      bool   `json:"autostart"`
	StartMinimized bool   `json:"startMinimized"`
}

// NetworkSettings holds proxy / TUN routing knobs.
type NetworkSettings struct {
	DefaultMode string      `json:"defaultMode"` // "tun" | "sysproxy"
	TunCIDR     string      `json:"tunCidr"`     // "198.18.0.1/15"
	TunMtu      int         `json:"tunMtu"`      // TUN interface MTU
	TunName     string      `json:"tunName"`     // "ITGRay-TUN"
	SocksPort   int         `json:"socksPort"`   // sysproxy mode local port
	HttpPort    int         `json:"httpPort"`    // sysproxy mode local HTTP port
	AllowLAN    bool        `json:"allowLan"`    // expose proxy to LAN
	IPv6Mode    string            `json:"ipv6Mode"`  // "prefer-v4" | "prefer-v6" | "disabled"
	GeoSource   GeoSourceSettings `json:"geoSource"` // rule-set repository selection
	DNS         DNSSettings       `json:"dns"`
}

// GeoSourceSettings surfaces the geo rule-set repository selection.
type GeoSourceSettings struct {
	Preset    string `json:"preset"`
	CustomURL string `json:"customURL"`
}

// DNSSettings holds resolver overrides surfaced via SettingsView.
type DNSSettings struct {
	Mode    string   `json:"mode"`    // "auto" | "custom"
	Servers []string `json:"servers"` // populated when Mode == "custom"
}

// KillSwitchSettings exposes the kill-switch toggles to the GUI.
type KillSwitchSettings struct {
	Enabled  bool `json:"enabled"`
	AlwaysOn bool `json:"alwaysOn"`
}

// SubscriptionSettings holds per-app subscription defaults.
type SubscriptionSettings struct {
	DefaultUpdateInterval int    `json:"defaultUpdateInterval"` // seconds
	UserAgent             string `json:"userAgent"`

	// Identity headers (Remnawave x-hwid contract). New in Tier 3.5.
	// HWIDEnabled gates the entire identity-header set; the 3 metadata
	// flags only fire when HWIDEnabled is true.
	HWIDEnabled     bool `json:"hwidEnabled"`
	SendDeviceOS    bool `json:"sendDeviceOS"`
	SendOSVersion   bool `json:"sendOSVersion"`
	SendDeviceModel bool `json:"sendDeviceModel"`
}

// NotificationSettings toggles desktop notification triggers.
type NotificationSettings struct {
	OnConnected    bool `json:"onConnected"`
	OnDisconnected bool `json:"onDisconnected"`
	QuotaLow       bool `json:"quotaLow"`
	OnSubSynced    bool `json:"onSubSynced"`
	Sound          bool `json:"sound"`
}

// DebugSettings holds developer-facing toggles. The "open logs folder" UI
// affordance is exposed in C.T12 as an action method on SettingsService, not
// as a flag on this DTO.
type DebugSettings struct {
	LogLevel string `json:"logLevel"` // "debug" | "info" | "warn" | "error"
}

// AboutSettings is build-info metadata for the About panel.
type AboutSettings struct {
	Version   string `json:"version"`
	GitRev    string `json:"gitRev"`
	BuildDate string `json:"buildDate"`
	Backend   string `json:"backend"`
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

// RulesView is the wire shape rules.list returns. Mirrors
// internal/rules.Model but uses string Action so the TS codegen
// surfaces the literal "proxy" / "direct" / "block" without a type
// alias hop.
type RulesView struct {
	DefaultAction string      `json:"defaultAction"`
	Groups        []GroupView `json:"groups"`
}

// GroupView is the wire shape for a single rule group.
type GroupView struct {
	ID      string     `json:"id"`
	Name    string     `json:"name"`
	Locked  bool       `json:"locked"`
	Enabled bool       `json:"enabled"`
	Rules   []RuleView `json:"rules"`
}

// RuleView is the wire shape for a single rule.
type RuleView struct {
	ID         string           `json:"id"`
	Name       string           `json:"name"`
	Enabled    bool             `json:"enabled"`
	Action     string           `json:"action"`
	Conditions rules.Conditions `json:"conditions"`
}
