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
	var useHelper bool
	var tunName, tunIPv4 string
	cmd := &cobra.Command{
		Use:   "run",
		Short: "start sing-box + xray chain pointing at the given server",
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

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if useHelper {
				cleanup, luid, err := startHelperSession(ctx, srv, tunName, tunIPv4)
				if err != nil {
					return fmt.Errorf("helper session: %w", err)
				}
				defer cleanup()
				slog.Info("helper session up", slog.Uint64("tun_luid", luid))

				return runWithTun(ctx, srv, tunName, tunIPv4, xrayPort)
			}
			return runWithSysProxy(ctx, srv, socksPort, xrayPort)
		},
	}
	cmd.Flags().StringVar(&serverID, "server", "", "server id (from `server list`)")
	cmd.Flags().IntVar(&socksPort, "socks-port", 1080, "local SOCKS5 inbound port (sysproxy mode)")
	cmd.Flags().IntVar(&xrayPort, "xray-port", 1081, "internal xray SOCKS inbound port")
	cmd.Flags().BoolVar(&useHelper, "use-helper", false, "use the Helper service to attach a WinTUN adapter (TUN mode)")
	cmd.Flags().StringVar(&tunName, "tun-name", "ITGRay-TUN", "TUN adapter name (TUN mode)")
	cmd.Flags().StringVar(&tunIPv4, "tun-ipv4", "198.18.0.1/15", "TUN IPv4 CIDR (TUN mode)")
	_ = cmd.MarkFlagRequired("server")
	return cmd
}

func runWithSysProxy(ctx context.Context, srv *server.Server, socksPort, xrayPort int) error {
	sbCfg, err := configgen.BuildSingbox(&configgen.SingboxInput{
		Mode:             configgen.ModeSysProxy,
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
	if err := mgr.Start(ctx, sbCfg, xrCfg); err != nil {
		return err
	}
	slog.Info("running sysproxy", slog.Int("socks", socksPort), slog.String("server", srv.Name))
	fmt.Fprintf(os.Stderr, "SOCKS5 inbound listening on 127.0.0.1:%d — Ctrl+C to stop\n", socksPort)
	return waitForSignal(mgr)
}

func runWithTun(ctx context.Context, srv *server.Server, tunName, tunIPv4 string, xrayPort int) error {
	sbCfg, err := configgen.BuildSingbox(&configgen.SingboxInput{
		Mode:          configgen.ModeTun,
		TunName:       tunName,
		TunIPv4:       tunIPv4,
		XraySOCKSHost: "127.0.0.1",
		XraySOCKSPort: xrayPort,
		Rules:         loadRules(),
	})
	if err != nil {
		return err
	}
	xrCfg, err := configgen.BuildXray(&configgen.XrayInput{Server: srv.Vless, SocksPort: xrayPort})
	if err != nil {
		return err
	}
	mgr := core.NewManager()
	if err := mgr.Start(ctx, sbCfg, xrCfg); err != nil {
		return err
	}
	slog.Info("running tun", slog.String("tun", tunName), slog.String("server", srv.Name))
	fmt.Fprintln(os.Stderr, "TUN attached to", tunName, "— Ctrl+C to stop")
	return waitForSignal(mgr)
}

func waitForSignal(mgr *core.Manager) error {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	if err := mgr.Stop(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}
