package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/itg-team/itg-ray/internal/configgen"
	"github.com/itg-team/itg-ray/internal/core"
	"github.com/itg-team/itg-ray/internal/server"
	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	var serverID string
	var socksPort, xrayPort int
	cmd := &cobra.Command{
		Use:   "run",
		Short: "start sing-box + xray chain pointing at the given server, exposing a local SOCKS5",
		RunE: func(_ *cobra.Command, _ []string) error {
			all, err := server.Load(serversPath())
			if err != nil {
				return err
			}
			var srv *server.Server
			for i := range all {
				if all[i].ID == serverID {
					srv = &all[i]
					break
				}
			}
			if srv == nil {
				return fmt.Errorf("server id %q not found", serverID)
			}

			sbCfg, err := configgen.BuildSingbox(&configgen.SingboxInput{
				SocksInboundPort: socksPort,
				XraySOCKSHost:    "127.0.0.1",
				XraySOCKSPort:    xrayPort,
				Rules:            loadRules(),
			})
			if err != nil {
				return err
			}
			xrCfg, err := configgen.BuildXray(&configgen.XrayInput{Server: srv.Vless, SocksPort: xrayPort})
			if err != nil {
				return err
			}

			mgr := core.NewManager()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if err := mgr.Start(ctx, sbCfg, xrCfg); err != nil {
				return err
			}
			slog.Info("running", slog.Int("socks", socksPort), slog.String("server", srv.Name))
			fmt.Fprintf(os.Stderr, "SOCKS5 inbound listening on 127.0.0.1:%d — Ctrl+C to stop\n", socksPort)

			sig := make(chan os.Signal, 1)
			signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
			<-sig
			if err := mgr.Stop(); err != nil && !errors.Is(err, context.Canceled) {
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&serverID, "server", "", "server id (from `server list`)")
	cmd.Flags().IntVar(&socksPort, "socks-port", 1080, "local SOCKS5 inbound port")
	cmd.Flags().IntVar(&xrayPort, "xray-port", 1081, "internal xray SOCKS inbound port")
	_ = cmd.MarkFlagRequired("server")
	return cmd
}
