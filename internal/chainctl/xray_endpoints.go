package chainctl

import (
	"encoding/json"
	"fmt"
)

func xrayServerEndpoints(xrayJSON []byte) (host string, port int, extra []string, err error) {
	var doc struct {
		Outbounds []struct {
			Protocol string `json:"protocol"`
			Settings struct {
				Vnext []struct {
					Address string `json:"address"`
					Port    int    `json:"port"`
				} `json:"vnext"`
			} `json:"settings"`
		} `json:"outbounds"`
	}
	if err := json.Unmarshal(xrayJSON, &doc); err != nil {
		return "", 0, nil, fmt.Errorf("decode xray config: %w", err)
	}
	if len(doc.Outbounds) == 0 || len(doc.Outbounds[0].Settings.Vnext) == 0 {
		return "", 0, nil, fmt.Errorf("xray config: vnext outbound missing")
	}
	v := doc.Outbounds[0].Settings.Vnext[0]
	if v.Address == "" || v.Port == 0 {
		return "", 0, nil, fmt.Errorf("xray config: vnext address/port empty")
	}
	for _, o := range doc.Outbounds[1:] {
		if o.Protocol != "vless" || len(o.Settings.Vnext) == 0 || o.Settings.Vnext[0].Address == "" {
			continue
		}
		extra = append(extra, o.Settings.Vnext[0].Address)
	}
	return v.Address, v.Port, extra, nil
}
