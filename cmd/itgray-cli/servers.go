package main

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/itg-team/itg-ray/internal/latency"
	"github.com/itg-team/itg-ray/internal/server"
	"github.com/spf13/cobra"
)

func newServerCmd() *cobra.Command {
	srv := &cobra.Command{Use: "server", Short: "manage servers"}

	srv.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "list all known servers (sorted by name)",
		RunE: func(*cobra.Command, []string) error {
			all, err := server.Load(serversPath())
			if err != nil {
				return err
			}
			for i := range all {
				lat := "-"
				if all[i].LatencyMS != nil {
					lat = fmt.Sprintf("%dms", *all[i].LatencyMS)
				}
				fmt.Printf("%s\t%s\t%s:%d\t%s\n", all[i].ID, all[i].Name, all[i].Vless.Address, all[i].Vless.Port, lat)
			}
			return nil
		},
	})

	srv.AddCommand(&cobra.Command{
		Use:   "test-latency",
		Short: "TCP-connect probe for every server",
		RunE: func(*cobra.Command, []string) error {
			all, err := server.Load(serversPath())
			if err != nil {
				return err
			}
			sem := make(chan struct{}, 16)
			var wg sync.WaitGroup
			mu := sync.Mutex{}
			for i := range all {
				wg.Go(func() {
					sem <- struct{}{}
					defer func() { <-sem }()
					addr := net.JoinHostPort(all[i].Vless.Address, fmt.Sprintf("%d", all[i].Vless.Port))
					d, err := latency.TCPConnect(context.Background(), addr, 5*time.Second)
					mu.Lock()
					defer mu.Unlock()
					if err != nil {
						all[i].LatencyMS = nil
					} else {
						ms := int(d / time.Millisecond)
						all[i].LatencyMS = &ms
					}
				})
			}
			wg.Wait()
			return server.Save(serversPath(), all)
		},
	})

	return srv
}
