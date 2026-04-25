package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/itg-team/itg-ray/internal/rules"
	"github.com/spf13/cobra"
)

func rulesPath() string { return filepath.Join(dataDir, "rules.json") }

func loadRules() rules.Model {
	b, err := os.ReadFile(rulesPath()) //nolint:gosec // path is controlled by --data-dir flag
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return rules.Model{
				DefaultAction: rules.ActionProxy,
				Groups: []rules.Group{
					{ID: "safety", Name: "Safety", Locked: true, Enabled: true, Rules: []rules.Rule{
						{ID: "private", Name: "Private IPs", Enabled: true, Action: rules.ActionDirect,
							Conditions: rules.Conditions{IPCIDRs: []string{
								"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "127.0.0.0/8",
								"fc00::/7", "fe80::/10", "224.0.0.0/4",
							}}},
					}},
					{ID: "user", Name: "My Rules", Enabled: true},
				},
			}
		}
		return rules.Model{DefaultAction: rules.ActionProxy}
	}
	var m rules.Model
	_ = json.Unmarshal(b, &m)
	return m
}

func saveRules(m rules.Model) error {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(m, "", "  ")
	return os.WriteFile(rulesPath(), b, 0o600)
}

func newRuleCmd() *cobra.Command {
	r := &cobra.Command{Use: "rule", Short: "manage routing rules"}

	add := &cobra.Command{
		Use:   "add",
		Short: "add a rule to the 'user' group",
		RunE: func(cmd *cobra.Command, _ []string) error {
			name, _ := cmd.Flags().GetString("name")
			action, _ := cmd.Flags().GetString("action")
			procs, _ := cmd.Flags().GetStringSlice("process")
			doms, _ := cmd.Flags().GetStringSlice("domain-suffix")
			cidrs, _ := cmd.Flags().GetStringSlice("ip-cidr")

			nr := rules.Rule{
				ID:      fmt.Sprintf("r%d", time.Now().UnixNano()),
				Name:    name,
				Enabled: true,
				Action:  rules.Action(action),
				Conditions: rules.Conditions{
					Processes: procs,
					IPCIDRs:   cidrs,
				},
			}
			for _, d := range doms {
				nr.Conditions.Domains = append(nr.Conditions.Domains, rules.DomainMatcher{Kind: "suffix", Value: d})
			}
			if err := nr.Validate(); err != nil {
				return err
			}

			m := loadRules()
			for i := range m.Groups {
				if m.Groups[i].ID == "user" {
					m.Groups[i].Rules = append(m.Groups[i].Rules, nr)
					break
				}
			}
			if err := saveRules(m); err != nil {
				return err
			}
			fmt.Println("added:", nr.ID)
			return nil
		},
	}
	add.Flags().String("name", "", "rule name")
	add.Flags().String("action", "proxy", "proxy|direct|block")
	add.Flags().StringSlice("process", nil, "process name (repeatable)")
	add.Flags().StringSlice("domain-suffix", nil, "domain suffix (repeatable)")
	add.Flags().StringSlice("ip-cidr", nil, "CIDR (repeatable)")
	r.AddCommand(add)

	r.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "list all rules grouped by folder",
		RunE: func(*cobra.Command, []string) error {
			m := loadRules()
			for i := range m.Groups {
				g := &m.Groups[i]
				fmt.Printf("[%s] %s (locked=%v enabled=%v)\n", g.ID, g.Name, g.Locked, g.Enabled)
				for j := range g.Rules {
					rl := &g.Rules[j]
					cond := []string{}
					if len(rl.Conditions.Processes) > 0 {
						cond = append(cond, "proc="+strings.Join(rl.Conditions.Processes, ","))
					}
					if len(rl.Conditions.IPCIDRs) > 0 {
						cond = append(cond, "cidr="+strings.Join(rl.Conditions.IPCIDRs, ","))
					}
					fmt.Printf("  %s  %s  %s  %s\n", rl.ID, rl.Action, rl.Name, strings.Join(cond, " "))
				}
			}
			return nil
		},
	})
	return r
}
