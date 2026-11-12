// Package rules models routing rules and compiles them to sing-box config.
// The compiler treats Geo entries as prefixed rule_set tags: "geosite:NAME" emits
// "geosite-NAME" and "geoip:CC" emits "geoip-CC" into rule_set, expecting the sing-box
// config generator (A9.1) to declare matching route.rule_set[] entries.
package rules

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Compile turns the UI rules model into the JSON used by sing-box's route block.
// The result is suitable for embedding directly under the "route" key of a sing-box config.
func Compile(m Model) ([]byte, error) {
	return CompileWith(m, nil)
}

func CompileWith(m Model, outboundFor func(r *Rule) string) ([]byte, error) {
	return CompileOpts(m, CompileOptions{OutboundFor: outboundFor})
}

type CompileOptions struct {
	OutboundFor func(*Rule) string
	Mark        bool
}

func MarkerInbound(ruleID string) string { return "itg-rule-" + ruleID }

func BlockOutbound(ruleID string) string { return "block-" + ruleID }

func CompileOpts(m Model, o CompileOptions) ([]byte, error) {
	outboundFor := o.OutboundFor
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
			r := &m.Groups[gi].Rules[ri]
			compiled := compileRule(r)
			if r.Action == ActionProxy && outboundFor != nil {
				if tag := outboundFor(r); tag != "" {
					compiled["outbound"] = tag
				}
			}
			if o.Mark {
				compiled = markRule(r, compiled)
			}
			out = append(out, compiled)
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

	var domain, suffix, keyword, regex, ruleset []any
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
			ruleset = append(ruleset, "geosite-"+strings.TrimPrefix(g, "geosite:"))
		case strings.HasPrefix(g, "geoip:"):
			ruleset = append(ruleset, "geoip-"+strings.TrimPrefix(g, "geoip:"))
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

func markRule(r *Rule, compiled map[string]any) map[string]any {
	outbound := compiled["outbound"]
	if r.Action == ActionBlock {
		outbound = BlockOutbound(r.ID)
	}
	inner := make(map[string]any, len(compiled))
	for k, v := range compiled {
		if k != "outbound" {
			inner[k] = v
		}
	}
	return map[string]any{
		"type":     "logical",
		"mode":     "or",
		"rules":    []any{inner, map[string]any{"inbound": []any{MarkerInbound(r.ID)}}},
		"outbound": outbound,
	}
}
