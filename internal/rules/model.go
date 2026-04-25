// Package rules models routing rules and compiles them to sing-box config.
package rules

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
)

// Action is a rule verdict.
type Action string

const (
	// ActionProxy routes the matched flow through the VPN tunnel.
	ActionProxy Action = "proxy"
	// ActionDirect lets the matched flow bypass the VPN.
	ActionDirect Action = "direct"
	// ActionBlock drops the matched flow.
	ActionBlock Action = "block"
)

// Valid reports whether a is one of the three known actions.
func (a Action) Valid() bool {
	switch a {
	case ActionProxy, ActionDirect, ActionBlock:
		return true
	}
	return false
}

// PortSpec is either a single port (Single != 0) or a [From, To] range (inclusive).
type PortSpec struct {
	Single uint16 `json:"single,omitempty"`
	From   uint16 `json:"from,omitempty"`
	To     uint16 `json:"to,omitempty"`
}

// Covers reports whether port falls within the spec.
func (p PortSpec) Covers(port uint16) bool {
	if p.Single != 0 {
		return p.Single == port
	}
	return port >= p.From && port <= p.To
}

// DomainMatcher is one element in a domain condition; Kind is one of
// "exact", "suffix", "keyword", or "regex".
type DomainMatcher struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

// Conditions is the AND-of-types / OR-within-type matching block of a rule.
type Conditions struct {
	Processes []string        `json:"processes,omitempty"`
	Domains   []DomainMatcher `json:"domains,omitempty"`
	IPCIDRs   []string        `json:"ip_cidrs,omitempty"`
	Geo       []string        `json:"geo,omitempty"`
	Ports     []PortSpec      `json:"ports,omitempty"`
	Protocols []string        `json:"protocols,omitempty"`
}

// IsEmpty reports whether all condition slices are empty.
func (c *Conditions) IsEmpty() bool {
	return len(c.Processes)+len(c.Domains)+len(c.IPCIDRs)+len(c.Geo)+len(c.Ports)+len(c.Protocols) == 0
}

// Rule is one routing entry.
type Rule struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Enabled    bool       `json:"enabled"`
	Action     Action     `json:"action"`
	Conditions Conditions `json:"conditions"`
}

// Validate checks structural invariants required for the rule to compile.
func (r *Rule) Validate() error {
	if r.ID == "" {
		return errors.New("rule.id is required")
	}
	if !r.Action.Valid() {
		return fmt.Errorf("rule.action %q is invalid", r.Action)
	}
	if r.Conditions.IsEmpty() {
		return errors.New("rule.conditions cannot be all-empty")
	}
	for i, p := range r.Conditions.Ports {
		if p.Single != 0 && (p.From != 0 || p.To != 0) {
			return fmt.Errorf("rule.conditions.ports[%d] cannot set both Single and From/To", i)
		}
		if p.Single == 0 && (p.From == 0 || p.To == 0 || p.From > p.To) {
			return fmt.Errorf("rule.conditions.ports[%d] invalid range", i)
		}
	}
	for i, d := range r.Conditions.Domains {
		switch d.Kind {
		case "exact", "suffix", "keyword", "regex":
		default:
			return fmt.Errorf("rule.conditions.domains[%d].kind %q invalid (must be exact|suffix|keyword|regex)", i, d.Kind)
		}
		if d.Value == "" {
			return fmt.Errorf("rule.conditions.domains[%d].value is empty", i)
		}
		if d.Kind == "regex" {
			if _, err := regexp.Compile(d.Value); err != nil {
				return fmt.Errorf("rule.conditions.domains[%d].value %q: %w", i, d.Value, err)
			}
		}
	}
	for i, cidr := range r.Conditions.IPCIDRs {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			if net.ParseIP(cidr) == nil {
				return fmt.Errorf("rule.conditions.ip_cidrs[%d] %q invalid", i, cidr)
			}
		}
	}
	for i, g := range r.Conditions.Geo {
		if !(strings.HasPrefix(g, "geosite:") || strings.HasPrefix(g, "geoip:")) {
			return fmt.Errorf("rule.conditions.geo[%d] %q must start with geosite: or geoip:", i, g)
		}
	}
	return nil
}

// Group bundles rules under a user-facing label. Locked groups (e.g. "Safety")
// cannot be removed by the UI.
type Group struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Locked  bool   `json:"locked,omitempty"`
	Enabled bool   `json:"enabled"`
	Rules   []Rule `json:"rules"`
}

// Model is the top-level routing configuration: ordered groups and the
// fallback action when no rule matches.
type Model struct {
	Groups        []Group `json:"groups"`
	DefaultAction Action  `json:"default_action"`
}

// Validate checks that all rules in all groups validate and that DefaultAction is known.
func (m Model) Validate() error {
	if !m.DefaultAction.Valid() {
		return fmt.Errorf("model.default_action %q invalid", m.DefaultAction)
	}
	for gi := range m.Groups {
		for ri := range m.Groups[gi].Rules {
			if err := m.Groups[gi].Rules[ri].Validate(); err != nil {
				return fmt.Errorf("group %q: %w", m.Groups[gi].Name, err)
			}
		}
	}
	return nil
}
