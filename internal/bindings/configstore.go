package bindings

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/itg-team/itg-ray/internal/hub"
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
	// chain would fail to set up routing. Shared with chainctl's
	// vpn:status projection via config.Network.EffectiveMode.
	mode := c.Network.EffectiveMode()
	return hub.SettingsView{
		General: hub.GeneralSettings{
			Language:       c.General.Language,
			Autostart:      c.General.Autostart,
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
			GeoBaseURL:  c.Network.GeoBaseURL,
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
			DefaultUpdateInterval: c.Subscriptions.DefaultUpdateInterval,
			UserAgent:             firstNonEmpty(c.Subscriptions.UserAgent, "ITGRay/"+s.version),
			HWIDEnabled:           c.Subscriptions.HWIDEnabled,
			SendDeviceOS:          c.Subscriptions.SendDeviceOS,
			SendOSVersion:         c.Subscriptions.SendOSVersion,
			SendDeviceModel:       c.Subscriptions.SendDeviceModel,
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
		if err := validateNetwork(&c.Network); err != nil {
			return err
		}
	case "killswitch":
		applyKillSwitch(&c.KillSwitch, patch)
	case "subscriptions":
		applySubscriptions(&c.Subscriptions, patch)
	case "notifications":
		applyNotifications(&c.Notifications, patch)
	case "debug":
		applyDebug(&c.Debug, patch)
	default:
		return errors.New("settings.Update: unknown section " + section)
	}
	return nil
}

// validateNetwork enforces cross-field invariants that the per-key
// applyNetwork branches can't see in isolation. Runs after the patch
// is merged into c.Network so the check sees the post-merge state.
//
// The configgen layer (internal/configgen/singbox.go) has a runtime
// fallback that collapses to a single mixed inbound when ports collide,
// but persisting a collision to disk gives the user a confusing config
// where Settings shows two distinct ports yet only one binds. Reject
// at save time instead.
func validateNetwork(n *config.Network) error {
	if n.SysProxy.SOCKSPort == n.SysProxy.HTTPPort {
		return fmt.Errorf("socksPort (%d) must differ from httpPort (%d)",
			n.SysProxy.SOCKSPort, n.SysProxy.HTTPPort)
	}
	return nil
}

func applyGeneral(g *config.General, p map[string]any) {
	if v, ok := p["language"].(string); ok {
		g.Language = v
	}
	if v, ok := p["autostart"].(bool); ok {
		g.Autostart = v
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
		if port := int(v); port >= 1 && port <= 65535 {
			n.SysProxy.SOCKSPort = port
		}
		// Out-of-range silently dropped (mirrors tunMtu) — frontend
		// flags invalid via isPortValid; clearing the input must not
		// persist port=0 to disk.
	}
	if v, ok := p["httpPort"].(float64); ok {
		if port := int(v); port >= 1 && port <= 65535 {
			n.SysProxy.HTTPPort = port
		}
	}
	if v, ok := p["allowLan"].(bool); ok {
		n.AllowLAN = v
	}
	if v, ok := p["ipv6Mode"].(string); ok {
		n.IPv6Mode = v
	}
	if v, ok := p["geoBaseURL"].(string); ok {
		n.GeoBaseURL = v
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

func applySubscriptions(s *config.Subscriptions, p map[string]any) {
	if v, ok := p["defaultUpdateInterval"].(float64); ok {
		s.DefaultUpdateInterval = int(v)
	}
	if v, ok := p["userAgent"].(string); ok {
		s.UserAgent = v
	}
	if v, ok := p["hwidEnabled"].(bool); ok {
		s.HWIDEnabled = v
	}
	if v, ok := p["sendDeviceOS"].(bool); ok {
		s.SendDeviceOS = v
	}
	if v, ok := p["sendOSVersion"].(bool); ok {
		s.SendOSVersion = v
	}
	if v, ok := p["sendDeviceModel"].(bool); ok {
		s.SendDeviceModel = v
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

// firstNonEmpty returns a if non-empty, else b. Used for UserAgent fallback
// where the version-injected default lives in the bindings layer.
func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
