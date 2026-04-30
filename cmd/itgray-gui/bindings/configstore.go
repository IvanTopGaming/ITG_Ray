package bindings

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/itg-team/itg-ray/cmd/itgray-gui/hub"
	"github.com/itg-team/itg-ray/internal/config"
)

// ConfigStore is the on-disk settings backing for SettingsService. It owns
// a path-keyed mutex so concurrent Update calls serialise their
// load-mutate-save cycle; the underlying config.Save uses an atomic
// tmp+rename on top of that for crash safety.
//
// internal/config exposes Load/Save free functions and a typed Config
// shape; the View/UpdateSection projection lives here so internal/* can
// stay process-shape-agnostic. Lowercase JSON keys (snake_case) on the
// disk-side Config are translated to camelCase SettingsView keys here
// rather than tagging both — the frontend never sees the config.Config
// type.
type ConfigStore struct {
	path      string
	version   string
	buildDate string
	mu        sync.Mutex
}

// NewConfigStore returns a ConfigStore rooted at path. version/buildDate
// fill the read-only About section.
func NewConfigStore(path, version, buildDate string) *ConfigStore {
	return &ConfigStore{path: path, version: version, buildDate: buildDate}
}

// View loads the persisted config and projects it into a SettingsView.
// Missing config.json is fine — internal/config.Load returns defaults.
func (s *ConfigStore) View() (hub.SettingsView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.viewLocked()
}

func (s *ConfigStore) viewLocked() (hub.SettingsView, error) {
	c, err := config.Load(s.path)
	if err != nil {
		return hub.SettingsView{}, fmt.Errorf("config.Load: %w", err)
	}
	return s.toView(&c), nil
}

// UpdateSection merges patch into the named section, persists atomically,
// and returns the post-merge SettingsView.
func (s *ConfigStore) UpdateSection(section string, patch map[string]any) (hub.SettingsView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := config.Load(s.path)
	if err != nil {
		return hub.SettingsView{}, fmt.Errorf("config.Load: %w", err)
	}
	if err := applyPatch(&c, section, patch); err != nil {
		return hub.SettingsView{}, err
	}
	if err := config.Save(s.path, c); err != nil {
		return hub.SettingsView{}, fmt.Errorf("config.Save: %w", err)
	}
	return s.toView(&c), nil
}

func (s *ConfigStore) toView(c *config.Config) hub.SettingsView {
	// Normalize legacy on-disk "auto" mode (pre-Tier-2a) to "tun" so the
	// removed ModeAuto runtime branch is not silently exercised on
	// upgrade — chainctl.bringUp would not match Auto post-F2 and the
	// chain would fail to set up routing.
	mode := c.Network.Mode
	if mode == "auto" {
		mode = "tun"
	}
	return hub.SettingsView{
		General: hub.GeneralSettings{
			Language:       c.General.Language,
			Theme:          c.General.Theme,
			Autostart:      c.General.Autostart,
			CloseToTray:    c.General.CloseToTray,
			StartMinimized: c.General.StartMinimized,
		},
		Network: hub.NetworkSettings{
			DefaultMode: mode,
			TunCIDR:     c.Network.TUN.IPv4CIDR,
			TunMtu:      c.Network.TUN.MTU,
			TunName:     "ITGRay-TUN",
			SocksPort:   c.Network.SysProxy.SOCKSPort,
			HttpPort:    c.Network.SysProxy.HTTPPort,
			AllowLAN:    c.Network.AllowLAN,
			IPv6Mode:    c.Network.IPv6Mode,
			DNS: hub.DNSSettings{
				Mode:    c.Network.DNS.Mode,
				Servers: c.Network.DNS.Servers,
			},
		},
		KillSwitch: hub.KillSwitchSettings{
			Enabled:  c.KillSwitch.Enabled,
			AlwaysOn: c.KillSwitch.AlwaysOn,
		},
		Subscriptions: hub.SubscriptionSettings{
			DefaultUpdateInterval: 3600,
			UserAgent:             "ITG-Ray/" + s.version,
		},
		Notifications: hub.NotificationSettings{
			OnConnected:    c.Notifications.Connected,
			OnDisconnected: c.Notifications.Disconnected,
			QuotaLow:       c.Notifications.QuotaLow,
			OnSubSynced:    c.Notifications.SubUpdated,
			Sound:          c.Notifications.Sound,
		},
		Debug: hub.DebugSettings{LogLevel: c.Debug.LogLevel},
		About: hub.AboutSettings{
			Version:   s.version,
			BuildDate: s.buildDate,
		},
		Security: hub.SecuritySettings{
			// internal/secret does not exist yet (v0.2 follow-up).
			Method:    "Unknown",
			Available: false,
			Warning:   "secret-protection method detection not yet implemented",
		},
	}
}

// applyPatch mutates c in place per the named section's patch map. Each
// branch type-asserts only the keys it knows; unknown keys are silently
// ignored so a forward-compatible frontend can ship new fields without
// breaking older binaries.
func applyPatch(c *config.Config, section string, patch map[string]any) error {
	switch section {
	case "general":
		applyGeneral(&c.General, patch)
	case "network":
		applyNetwork(&c.Network, patch)
	case "killswitch":
		applyKillSwitch(&c.KillSwitch, patch)
	case "subscriptions":
		// no persisted fields yet; accept the patch as a no-op so the
		// frontend can wire forms without backend churn.
	case "notifications":
		applyNotifications(&c.Notifications, patch)
	case "debug":
		applyDebug(&c.Debug, patch)
	default:
		return errors.New("settings.Update: unknown section " + section)
	}
	return nil
}

func applyGeneral(g *config.General, p map[string]any) {
	if v, ok := p["language"].(string); ok {
		g.Language = v
	}
	if v, ok := p["theme"].(string); ok {
		g.Theme = v
	}
	if v, ok := p["autostart"].(bool); ok {
		g.Autostart = v
	}
	if v, ok := p["closeToTray"].(bool); ok {
		g.CloseToTray = v
	}
	if v, ok := p["startMinimized"].(bool); ok {
		g.StartMinimized = v
	}
}

func applyNetwork(n *config.Network, p map[string]any) {
	if v, ok := p["defaultMode"].(string); ok {
		n.Mode = v
	}
	if v, ok := p["tunCidr"].(string); ok {
		n.TUN.IPv4CIDR = v
	}
	if v, ok := p["tunMtu"].(float64); ok {
		if mtu := int(v); mtu >= 576 && mtu <= 9000 {
			n.TUN.MTU = mtu
		}
		// Out-of-range silently dropped — frontend should clamp before sending.
	}
	if v, ok := p["socksPort"].(float64); ok {
		n.SysProxy.SOCKSPort = int(v)
	}
	if v, ok := p["httpPort"].(float64); ok {
		n.SysProxy.HTTPPort = int(v)
	}
	if v, ok := p["allowLan"].(bool); ok {
		n.AllowLAN = v
	}
	if v, ok := p["ipv6Mode"].(string); ok {
		n.IPv6Mode = v
	}
	if v, ok := p["dnsMode"].(string); ok {
		n.DNS.Mode = v
	}
	if servers, ok := p["dnsServers"].([]any); ok {
		out := make([]string, 0, len(servers))
		for _, s := range servers {
			if str, ok := s.(string); ok {
				str = strings.TrimSpace(str)
				if str != "" {
					out = append(out, str)
				}
			}
		}
		n.DNS.Servers = out
	}
}

func applyKillSwitch(k *config.KillSwitch, p map[string]any) {
	if v, ok := p["enabled"].(bool); ok {
		k.Enabled = v
	}
	if v, ok := p["alwaysOn"].(bool); ok {
		k.AlwaysOn = v
	}
}

func applyNotifications(n *config.Notifications, p map[string]any) {
	if v, ok := p["onConnected"].(bool); ok {
		n.Connected = v
	}
	if v, ok := p["onDisconnected"].(bool); ok {
		n.Disconnected = v
	}
	if v, ok := p["quotaLow"].(bool); ok {
		n.QuotaLow = v
	}
	if v, ok := p["onSubSynced"].(bool); ok {
		n.SubUpdated = v
	}
	if v, ok := p["sound"].(bool); ok {
		n.Sound = v
	}
}

func applyDebug(d *config.Debug, p map[string]any) {
	if v, ok := p["logLevel"].(string); ok {
		d.LogLevel = v
	}
}
