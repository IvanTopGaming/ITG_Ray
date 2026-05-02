package config

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
)

// General holds UI and behaviour preferences.
type General struct {
	Language       string `json:"language"`
	Autostart      bool   `json:"autostart"`
	StartMinimized bool   `json:"start_minimized"`
}

// TUN holds the TUN-mode adapter parameters.
type TUN struct {
	IPv4CIDR string `json:"ipv4_cidr"`
	MTU      int    `json:"mtu"`
}

// SysProxy holds the System Proxy ports.
type SysProxy struct {
	HTTPPort  int `json:"http_port"`
	SOCKSPort int `json:"socks_port"`
}

// Network bundles the operation-mode-specific options.
type Network struct {
	Mode     string   `json:"mode"`
	TUN      TUN      `json:"tun"`
	SysProxy SysProxy `json:"sysproxy"`
	AllowLAN bool     `json:"allow_lan"`
	IPv6Mode string   `json:"ipv6_mode"` // "prefer-v4" | "prefer-v6" | "disabled"
	DNS      DNS      `json:"dns"`
}

// EffectiveMode returns the user-facing Mode, normalizing the legacy
// pre-Tier-2a "auto" sentinel to "tun" so the camelCase frontend shape
// (whether emitted via SettingsView or via the vpn:status hub payload)
// stays consistent. Today only "tun" and "sysproxy" are valid modes;
// "auto" is treated as a synonym for "tun" until next save normalizes it.
func (n Network) EffectiveMode() string {
	if n.Mode == "auto" {
		return "tun"
	}
	return n.Mode
}

// DNS holds resolver overrides that apply across both TUN and SysProxy modes.
type DNS struct {
	Mode    string   `json:"mode"`    // "auto" | "custom"
	Servers []string `json:"servers"` // populated when Mode == "custom"
}

// KillSwitch toggles the soft killswitch and always-on mode.
type KillSwitch struct {
	Enabled  bool `json:"enabled"`
	AlwaysOn bool `json:"always_on"`
}

// Subscriptions holds per-app subscription defaults and identity-header
// preferences (Tier 3.5).
type Subscriptions struct {
	DefaultUpdateInterval int    `json:"defaultUpdateInterval"`
	UserAgent             string `json:"userAgent"`
	HWIDEnabled           bool   `json:"hwidEnabled"`
	SendDeviceOS          bool   `json:"sendDeviceOS"`
	SendOSVersion         bool   `json:"sendOSVersion"`
	SendDeviceModel       bool   `json:"sendDeviceModel"`
}

// Notifications enables/disables event-driven OS toasts.
type Notifications struct {
	Connected    bool `json:"connected"`
	Disconnected bool `json:"disconnected"`
	QuotaLow     bool `json:"quota_low"`
	SubUpdated   bool `json:"sub_updated"`
	Sound        bool `json:"sound"`
}

// Debug captures developer-facing toggles persisted to config.json.
type Debug struct {
	LogLevel string `json:"log_level"` // "error" | "info" | "debug" | "trace"
}

// Config is the top-level application configuration persisted as config.json.
type Config struct {
	Version       int           `json:"version"`
	General       General       `json:"general"`
	Network       Network       `json:"network"`
	KillSwitch    KillSwitch    `json:"killswitch"`
	Subscriptions Subscriptions `json:"subscriptions"`
	Notifications Notifications `json:"notifications"`
	Debug         Debug         `json:"debug"`
}

func defaults() Config {
	return Config{
		Version: 1,
		General: General{Language: "en"},
		Network: Network{
			Mode:     "tun",
			TUN:      TUN{IPv4CIDR: "198.18.0.1/15", MTU: 1500},
			SysProxy: SysProxy{HTTPPort: 8888, SOCKSPort: 1080},
			AllowLAN: false,
			IPv6Mode: "prefer-v4",
			DNS:      DNS{Mode: "auto", Servers: nil},
		},
		KillSwitch: KillSwitch{Enabled: true},
		Subscriptions: Subscriptions{
			DefaultUpdateInterval: 3600,
			UserAgent:             "", // empty = inherits from version-injected default downstream
			HWIDEnabled:           true,
			SendDeviceOS:          true,
			SendOSVersion:         true,
			SendDeviceModel:       true,
		},
		Notifications: Notifications{Connected: true, Disconnected: true, QuotaLow: true, SubUpdated: true, Sound: true},
		Debug:         Debug{LogLevel: "info"},
	}
}

// Defaults returns the default Config — same shape callers get when
// config.Load encounters a missing or empty config.json. Exported so
// non-package consumers (chainctl, tests) can build fallback Network
// values without reaching for the unexported defaults().
func Defaults() Config { return defaults() }

// Load reads config.json and overlays its values onto the defaults.
// A missing file returns the defaults with no error.
func Load(path string) (Config, error) {
	c := defaults()
	b, err := os.ReadFile(path) //nolint:gosec // path is caller-supplied config file location
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return c, nil
		}
		return Config{}, err
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return Config{}, err
	}
	return c, nil
}

// Save writes the configuration to disk atomically with 0600 permissions.
func Save(path string, c Config) error { //nolint:gocritic // hugeParam: value copy is intentional for API clarity
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return WriteAtomic(path, b, 0o600)
}
