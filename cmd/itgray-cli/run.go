package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/itg-team/itg-ray/internal/configgen"
	"github.com/itg-team/itg-ray/internal/core"
	"github.com/itg-team/itg-ray/internal/server"
	"github.com/spf13/cobra"
)

// resolveServerIPv4 resolves host to an IPv4 literal via the system resolver.
// It is called BEFORE any TUN/DNS hijack is installed, so the lookup goes
// through the host's normal LAN DNS path (not FakeIP). This lets xray dial an
// IP literal and avoids a TUN → sing-box → FakeIP → xray loop in helper mode.
// If host is already an IP literal, net.LookupIP returns it as-is.
func resolveServerIPv4(host string) (string, error) {
	ips, err := net.LookupIP(host)
	if err != nil {
		return "", fmt.Errorf("lookup %q: %w", host, err)
	}
	for _, ip := range ips {
		if v4 := ip.To4(); v4 != nil {
			return v4.String(), nil
		}
	}
	return "", fmt.Errorf("no IPv4 address for server host %q", host)
}

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
				sbCfg, err := configgen.BuildSingbox(&configgen.SingboxInput{
					Mode:          configgen.ModeTun,
					FakeIP:        true,
					TunName:       tunName,
					TunIPv4:       tunIPv4,
					XraySOCKSHost: "127.0.0.1",
					XraySOCKSPort: xrayPort,
					Rules:         loadRules(),
				})
				if err != nil {
					return err
				}
				serverIP, err := resolveServerIPv4(srv.Vless.Address)
				if err != nil {
					return fmt.Errorf("resolve server host %q: %w", srv.Vless.Address, err)
				}
				xrCfg, err := configgen.BuildXray(&configgen.XrayInput{Server: srv.Vless, ServerIP: serverIP, SocksPort: xrayPort})
				if err != nil {
					return err
				}
				sess, err := startHelperSession(ctx, srv, sbCfg, xrCfg, tunName)
				if err != nil {
					return fmt.Errorf("helper session: %w", err)
				}
				defer sess.cleanup(context.Background())

				slog.Info("helper session up", slog.String("session", sess.sessionID))
				fmt.Fprintln(os.Stderr, "TUN-mode VPN is up via helper. Logs:")
				fmt.Fprintln(os.Stderr, `  C:\ProgramData\ITG Ray\Helper\runtime\sing-box.log`)
				fmt.Fprintln(os.Stderr, `  C:\ProgramData\ITG Ray\Helper\runtime\xray.log`)
				fmt.Fprintln(os.Stderr, "Ctrl+C to stop.")

				return waitForSignalNoMgr()
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
	serverIP, err := resolveServerIPv4(srv.Vless.Address)
	if err != nil {
		return fmt.Errorf("resolve server host %q: %w", srv.Vless.Address, err)
	}
	xrCfg, err := configgen.BuildXray(&configgen.XrayInput{Server: srv.Vless, ServerIP: serverIP, SocksPort: xrayPort})
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

func waitForSignal(mgr *core.Manager) error {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	if err := mgr.Stop(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}

func waitForSignalNoMgr() error {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	return nil
}
