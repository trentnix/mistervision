package companion

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
)

// discover answers player searches only from explicitly allowed peers. Server
// discovery elsewhere in Plex uses a different port and remains independent.
func (s *Source) discover(ctx context.Context, socket *net.UDPConn, port int) {
	buffer := make([]byte, 4097)
	for ctx.Err() == nil {
		n, peer, err := socket.ReadFromUDPAddrPort(buffer)
		if err != nil {
			return
		}

		if n > 4096 || !s.allowed(peer.Addr()) || !strings.HasPrefix(string(buffer[:n]), "M-SEARCH * HTTP/1.0\r\n") {
			continue
		}
		_, err = socket.WriteToUDPAddrPort([]byte(s.advertisement(port)), peer)
		status := 200
		if err != nil {
			status = 0
		}
		s.record(Event{Operation: "discovery", Status: status})
	}
}

func (s *Source) advertisement(port int) string {
	return fmt.Sprintf("HTTP/1.0 200 OK\r\nContent-Type: plex/media-player\r\nName: %s\r\nPort: %d\r\nProduct: MiSTerVision\r\nProtocol: plex\r\nProtocol-Version: 1\r\nProtocol-Capabilities: %s\r\nResource-Identifier: %s\r\nVersion: %s\r\nDevice-Class: pc\r\n\r\n", s.config.Name, port, probeCapabilities, s.config.Identifier, s.config.Version)
}

// announce registers the probe with PMS and withdraws it before socket closure.
// It runs separately from search handling so protocol unit tests stay local.
func (s *Source) announce(ctx context.Context, socket *net.UDPConn, port int) {
	send := func(method string) {
		packet := strings.Replace(s.advertisement(port), "HTTP/1.0 200 OK", method+" * HTTP/1.0", 1)
		_, _ = socket.WriteToUDP([]byte(packet), &net.UDPAddr{IP: net.IPv4(239, 0, 0, 250), Port: 32413})
	}
	send("HELLO")
	defer send("BYE")
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			send("HELLO")
		}
	}
}
