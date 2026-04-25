package main

import "github.com/spf13/cobra"

func newRuleCmd() *cobra.Command { return &cobra.Command{Use: "rule", Short: "manage routing rules"} }
