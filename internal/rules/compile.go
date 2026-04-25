// Package rules models routing rules and compiles them to sing-box config.
// The compiler treats Geo entries as bare tag names: "geosite:NAME" emits "NAME"
// into rule_set, expecting the sing-box config generator (A9.1) to declare matching
// route.rule_set[] entries. "geoip:CC" emits "CC" into the legacy geoip field.
package rules

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Compile turns the UI rules model into the JSON used by sing-box's route block.
// The result is suitable for embedding directly under the "route" key of a sing-box config.
func Compile(m Model) ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	doc := map[string]any{
		"final": string(m.DefaultAction),
	}
	var out []map[string]any
	for gi := range m.Groups {
		if !m.Groups[gi].Enabled {
			continue
		}
		for ri := range m.Groups[gi].Rules {
			if !m.Groups[gi].Rules[ri].Enabled {
				continue
			}
			out = append(out, compileRule(&m.Groups[gi].Rules[ri]))
		}
	}
	if len(out) > 0 {
		doc["rules"] = out
	} else {
		doc["rules"] = []map[string]any{}
	}
	return json.Marshal(doc)
}

func compileRule(r *Rule) map[string]any {
	rule := map[string]any{"outbound": string(r.Action)}

	if len(r.Conditions.Processes) > 0 {
		rule["process_name"] = toAny(r.Conditions.Processes)
	}
	if len(r.Conditions.IPCIDRs) > 0 {
		rule["ip_cidr"] = toAny(r.Conditions.IPCIDRs)
	}
	if len(r.Conditions.Protocols) > 0 {
		rule["network"] = toAny(r.Conditions.Protocols)
	}

	var singles []any
	var ranges []any
	for _, p := range r.Conditions.Ports {
		if p.Single != 0 {
			singles = append(singles, int(p.Single))
		} else {
			ranges = append(ranges, fmt.Sprintf("%d:%d", p.From, p.To))
		}
	}
	if len(singles) > 0 {
		rule["port"] = singles
	}
	if len(ranges) > 0 {
		rule["port_range"] = ranges
	}

	var domain, suffix, keyword, regex, geosite, geoip, ruleset []any
	for _, d := range r.Conditions.Domains {
		switch d.Kind {
		case "exact":
			domain = append(domain, d.Value)
		case "suffix":
			suffix = append(suffix, d.Value)
		case "keyword":
			keyword = append(keyword, d.Value)
		case "regex":
			regex = append(regex, d.Value)
		}
	}
	for _, g := range r.Conditions.Geo {
		switch {
		case strings.HasPrefix(g, "geosite:"):
			// sing-box rule_set takes the bare tag name; A9.1 declares matching route.rule_set[] entries.
			geosite = append(geosite, strings.TrimPrefix(g, "geosite:"))
		case strings.HasPrefix(g, "geoip:"):
			geoip = append(geoip, strings.TrimPrefix(g, "geoip:"))
		}
	}
	set := func(k string, v []any) {
		if len(v) > 0 {
			rule[k] = v
		}
	}
	set("domain", domain)
	set("domain_suffix", suffix)
	set("domain_keyword", keyword)
	set("domain_regex", regex)
	if len(geosite) > 0 {
		ruleset = append(ruleset, geosite...)
	}
	if len(geoip) > 0 {
		rule["geoip"] = geoip
	}
	set("rule_set", ruleset)

	return rule
}

func toAny[T any](in []T) []any {
	out := make([]any, len(in))
	for i := range in {
		out[i] = in[i]
	}
	return out
}
