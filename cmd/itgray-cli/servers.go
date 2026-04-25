package main

import "github.com/spf13/cobra"

func newServerCmd() *cobra.Command { return &cobra.Command{Use: "server", Short: "manage servers"} }
