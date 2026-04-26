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
	Theme          string `json:"theme"`
	Autostart      bool   `json:"autostart"`
	CloseToTray    bool   `json:"close_to_tray"`
	StartMinimized bool   `json:"start_minimized"`
}

// TUN holds the TUN-mode adapter parameters.
type TUN struct {
	IPv4CIDR string   `json:"ipv4_cidr"`
	MTU      int      `json:"mtu"`
	DNS      []string `json:"dns"`
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
}

// KillSwitch toggles the soft killswitch and always-on mode.
type KillSwitch struct {
	Enabled  bool `json:"enabled"`
	AlwaysOn bool `json:"always_on"`
}

// Updates configures the auto-update check policy.
type Updates struct {
	AutoCheck bool   `json:"auto_check"`
	Channel   string `json:"channel"`
}

// Notifications enables/disables event-driven OS toasts.
type Notifications struct {
	Connected    bool `json:"connected"`
	Disconnected bool `json:"disconnected"`
	QuotaLow     bool `json:"quota_low"`
	SubUpdated   bool `json:"sub_updated"`
}

// Config is the top-level application configuration persisted as config.json.
type Config struct {
	Version       int           `json:"version"`
	General       General       `json:"general"`
	Network       Network       `json:"network"`
	KillSwitch    KillSwitch    `json:"killswitch"`
	Updates       Updates       `json:"updates"`
	Notifications Notifications `json:"notifications"`
}

func defaults() Config {
	return Config{
		Version: 1,
		General: General{Language: "en", Theme: "dark", CloseToTray: true},
		Network: Network{
			Mode:     "tun",
			TUN:      TUN{IPv4CIDR: "198.18.0.1/15", MTU: 1500, DNS: []string{"1.1.1.1", "8.8.8.8"}},
			SysProxy: SysProxy{HTTPPort: 8888, SOCKSPort: 1080},
		},
		KillSwitch:    KillSwitch{Enabled: true},
		Updates:       Updates{AutoCheck: true, Channel: "stable"},
		Notifications: Notifications{Connected: true, Disconnected: true, QuotaLow: true, SubUpdated: true},
	}
}

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
