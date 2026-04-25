package main

import "github.com/spf13/cobra"

func newRunCmd() *cobra.Command { return &cobra.Command{Use: "run", Short: "start the core chain"} }
