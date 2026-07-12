//go:build linux

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/itg-team/itg-ray/internal/helper/protocol"
	"github.com/itg-team/itg-ray/internal/helper/server"
)

const (
	socketPath = "/run/itgray-helper.sock"
	installDir = "/usr/local/lib/itgray"
	unitDir    = "/etc/systemd/system"
)

func runService() error {
	uid, err := strconv.ParseUint(os.Getenv("ITGRAY_ALLOW_UID"), 10, 32)
	if err != nil {
		return fmt.Errorf("ITGRAY_ALLOW_UID must be set to the owning uid: %w", err)
	}
	d := buildDispatcher()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		_ = server.StopActiveChain()
		cancel()
	}()

	return server.Listen(ctx, socketPath, d, uint32(uid))
}

func buildDispatcher() *server.Dispatcher {
	d := server.NewDispatcher()
	d.Register(protocol.OpServiceStatus, server.NewServiceStatusHandler(Version, time.Now(), server.IsChainActive))
	d.Register(protocol.OpStartChain, server.NewStartChainHandler())
	d.Register(protocol.OpStopChain, server.NewStopChainHandler())
	d.Register(protocol.OpReadLogs, server.NewReadLogsHandler())
	return d
}
