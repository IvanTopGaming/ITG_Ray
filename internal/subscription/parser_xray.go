package subscription

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/itg-team/itg-ray/internal/server"
	"github.com/itg-team/itg-ray/internal/vless"
)

var ErrNotXrayJSON = errors.New("not an Xray JSON subscription")

func ParseXray(body string) (ParseResult, error) {
	var docs []json.RawMessage
	if strings.HasPrefix(strings.TrimSpace(body), "[") {
		if json.Unmarshal([]byte(body), &docs) != nil {
			return ParseResult{}, ErrNotXrayJSON
		}
	} else {
		docs = []json.RawMessage{json.RawMessage(body)}
	}

	result := ParseResult{Skipped: map[string]int{}}
	seen := map[string]int{}
	named := map[string]bool{}
	recognized := false
	for _, rawDoc := range docs {
		var doc xrayDocument
		if json.Unmarshal(rawDoc, &doc) != nil || doc.Outbounds == nil {
			result.Invalid++
			continue
		}
		var configs []vless.Config
		for _, raw := range doc.Outbounds {
			var fields map[string]json.RawMessage
			if json.Unmarshal(raw, &fields) != nil {
				result.Invalid++
				continue
			}
			if _, ok := fields["protocol"]; !ok {
				result.Invalid++
				continue
			}
			recognized = true
			var header struct {
				Protocol string `json:"protocol"`
			}
			if json.Unmarshal(raw, &header) != nil {
				result.Invalid++
				continue
			}
			switch header.Protocol {
			case "freedom", "blackhole", "dns":
				continue
			case "vless":
			default:
				result.Skipped[safeProtocol(header.Protocol)]++
				continue
			}
			var out xrayOutbound
			if json.Unmarshal(raw, &out) != nil {
				result.Invalid++
				continue
			}
			nodes, complex, invalid := parseXrayOutbound(out)
			result.Invalid += invalid
			if complex {
				result.Skipped["xray-complex"]++
				continue
			}
			configs = append(configs, nodes...)
		}
		preferredName := len(configs) == 1 && strings.TrimSpace(doc.Remarks) != ""
		for _, c := range configs {
			if preferredName {
				c.Remark = strings.TrimSpace(doc.Remarks)
			}
			key := server.StableID(c)
			if index, ok := seen[key]; ok {
				if preferredName && !named[key] {
					result.Configs[index] = c
					named[key] = true
				}
				continue
			}
			seen[key] = len(result.Configs)
			named[key] = preferredName
			result.Configs = append(result.Configs, c)
		}
	}
	if !recognized {
		return ParseResult{}, ErrNotXrayJSON
	}
	return result, nil
}

func parseXrayOutbound(out xrayOutbound) ([]vless.Config, bool, int) {
	if out.ProxySettings.Tag != "" || out.Stream.Sockopt.DialerProxy != "" {
		return nil, true, 0
	}
	var settings xraySettings
	if json.Unmarshal(out.Settings, &settings) != nil {
		return nil, false, 1
	}
	endpoints := settings.VNext
	if settings.Address != "" {
		endpoints = []xrayEndpoint{{Address: settings.Address, Port: settings.Port, Users: []xrayUser{settings.xrayUser}}}
	}
	if len(endpoints) == 0 {
		return nil, false, 1
	}
	base := vless.Config{Remark: strings.TrimSpace(out.Tag), Encryption: "none"}
	supported, valid := applyXrayStream(&base, out.Stream)
	if !supported {
		return nil, true, 0
	}
	if !valid {
		return nil, false, 1
	}
	var configs []vless.Config
	invalid := 0
	for _, endpoint := range endpoints {
		if strings.TrimSpace(endpoint.Address) == "" || endpoint.Port < 1 || endpoint.Port > 65535 || len(endpoint.Users) == 0 {
			invalid++
			continue
		}
		for _, user := range endpoint.Users {
			if hasJSONValue(user.Reverse) {
				return nil, true, 0
			}
			if strings.TrimSpace(user.ID) == "" {
				invalid++
				continue
			}
			c := base
			c.Address = strings.TrimSpace(endpoint.Address)
			c.Port = uint16(endpoint.Port)
			c.UUID = user.ID
			c.Flow = user.Flow
			if user.Encryption != "" {
				c.Encryption = user.Encryption
			}
			if _, err := c.Normalize(); err != nil || c.Flow != user.Flow {
				invalid++
				continue
			}
			configs = append(configs, c)
		}
	}
	return configs, false, invalid
}

func hasJSONValue(raw json.RawMessage) bool {
	value := strings.TrimSpace(string(raw))
	return value != "" && value != "null" && value != "{}" && value != "[]"
}

func safeProtocol(protocol string) string {
	protocol = strings.ToLower(protocol)
	if len(protocol) > 32 || !validScheme(protocol) {
		return "unknown"
	}
	return protocol
}
