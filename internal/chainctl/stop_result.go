package chainctl

import (
	"encoding/json"
	"fmt"
	"strings"
)

func checkStopResult(raw json.RawMessage) error {
	var result struct {
		Status        string   `json:"status"`
		PartialErrors []string `json:"partial_errors"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return fmt.Errorf("decode StopChain result: %w", err)
	}
	if len(result.PartialErrors) > 0 {
		return fmt.Errorf("StopChain cleanup incomplete: %s", strings.Join(result.PartialErrors, "; "))
	}
	if result.Status != "stopped" && result.Status != "no-active-chain" {
		return fmt.Errorf("unexpected StopChain status: %q", result.Status)
	}
	return nil
}
