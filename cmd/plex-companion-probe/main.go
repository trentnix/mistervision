// plex-companion-probe advertises an isolated diagnostic player on the LAN.
// It never reads saved accounts or starts playback. See docs/PLEX_REMOTE_PLAN.md.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"os/signal"
	"syscall"

	"mistervision/internal/plex/companion"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var peers []netip.Addr
	listen := flag.String("listen", ":32433", "diagnostic HTTP listen address")
	id := flag.String("id", "mistervision-companion-probe", "unique diagnostic player identifier")
	name := flag.String("name", "MiSTerVision Probe", "name displayed in Plex")
	iface := flag.String("interface", "", "LAN interface for multicast (default: system route)")
	flag.Func("allow", "trusted controller or Plex server IPv4 address (repeatable, required)", func(value string) error {
		ip, err := netip.ParseAddr(value)
		if err == nil {
			peers = append(peers, ip)
		}
		return err
	})
	flag.Parse()
	if flag.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	config := companion.Config{Identifier: *id, Name: *name, Version: "prototype", Listen: *listen, AllowedPeers: peers}
	if *iface != "" {
		var err error
		config.Interface, err = net.InterfaceByName(*iface)
		if err != nil {
			return fmt.Errorf("LAN interface is unavailable")
		}
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	config.Observe = func(e companion.Event) {
		logger.Info("companion probe", "operation", e.Operation, "method", e.Method, "status", e.Status, "port", e.Port, "command_id", e.CommandID, "has_client", e.HasClient, "has_token", e.HasToken)
	}
	source, err := companion.New(config)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	source.Run(ctx, nil)
	if ctx.Err() == nil {
		return fmt.Errorf("probe stopped unexpectedly; see diagnostic output")
	}
	return nil
}
