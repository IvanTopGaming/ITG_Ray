package main

import "github.com/spf13/cobra"

func newSubCmd() *cobra.Command { return &cobra.Command{Use: "sub", Short: "manage subscriptions"} }
