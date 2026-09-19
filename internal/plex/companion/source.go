// Package companion translates Plex LAN discovery, video commands, and timeline
// polls into the shared remote-control interfaces. An explicit probe mode exposes
// protocol diagnostics without authorizing or executing commands.
package companion

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"

	"mistervision/internal/remote"
)

// Config identifies a receiver. Control supplies account authorization. Without
// Control, AllowedPeers must explicitly name trusted diagnostic IPv4 peers.
// With Control and no allowlist, only private and loopback IPv4 peers are accepted.
type Config struct {
	Identifier, Name, Version string
	Listen                    string
	AllowedPeers              []netip.Addr
	// Interface optionally selects the LAN interface used for multicast.
	Interface *net.Interface
	// Observe receives bounded, credential-free results from network workers.
	// It must return promptly and be safe for concurrent use.
	Observe func(Event)
	// Control enables authenticated video commands for one active server/viewer.
	// Without Control, the receiver is an inert, allowlisted diagnostic probe.
	Control *ControlConfig
}

// Event records protocol evidence without request URLs, identities, or tokens.
// CommandID is receipt evidence only. Rejected commands are never completed.
type Event struct {
	Operation           string
	Method              string // Recognized HTTP method, or "other". Empty for socket events.
	Status              int
	Port                int // Bound HTTP port, present only in the ready event.
	CommandID           int64
	HasClient, HasToken bool
}

// Source owns LAN sockets and an immutable playback snapshot. Run must not
// overlap another Run. PublishPlayback can overlap Run. An inert
// diagnostic source has no Control configuration and never emits commands.
type Source struct {
	config   Config
	mu       sync.Mutex
	state    remote.PlaybackState
	changed  chan struct{}
	requests chan struct{}
	control  *receiver
}

var _ remote.Source = (*Source)(nil)
var _ remote.PlaybackObserver = (*Source)(nil)

// New validates configuration before any socket is opened.
func New(config Config) (*Source, error) {
	for _, value := range []string{config.Identifier, config.Name, config.Version} {
		if value == "" || len(value) > 128 || strings.ContainsAny(value, "\r\n\x00") {
			return nil, errors.New("probe identity must contain 1–128 characters without line breaks")
		}
	}
	if len(config.AllowedPeers) == 0 && config.Control == nil {
		return nil, errors.New("specify at least one trusted probe peer")
	}
	for _, peer := range config.AllowedPeers {
		if !peer.Is4() || !(peer.IsPrivate() || peer.IsLoopback()) {
			return nil, errors.New("probe peers must be private or loopback IPv4 addresses")
		}
	}
	config.AllowedPeers = append([]netip.Addr(nil), config.AllowedPeers...)
	s := &Source{config: config, state: remote.PlaybackState{Status: remote.Stopped}, changed: make(chan struct{}), requests: make(chan struct{}, 8)}
	if config.Control != nil {
		if config.Control.ServerID == "" || config.Control.Authorize == nil {
			return nil, errors.New("Companion requires an active server and authorizer")
		}
		s.control = &receiver{config: *config.Control, controllers: make(map[controllerKey]*controller)}
	}
	return s, nil
}

// Run serves the receiver until cancellation or socket failure. Startup failures
// are reported through Observe. Only authenticated Control sources call emit.
func (s *Source) Run(ctx context.Context, emit func(remote.Command)) {
	if ctx.Err() != nil {
		return
	}
	if s.control != nil {
		s.control.start(ctx, emit)
		defer s.control.stop()
	}
	listener, err := net.Listen("tcp4", s.config.Listen)
	if err != nil {
		s.record(Event{Operation: "listen-failed"})
		return
	}
	defer listener.Close()
	udp, err := net.ListenMulticastUDP("udp4", s.config.Interface, &net.UDPAddr{IP: net.IPv4(239, 0, 0, 250), Port: 32412})
	if err != nil {
		s.record(Event{Operation: "discovery-failed"})
		return
	}
	defer udp.Close()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var handlerMu sync.Mutex
	var handlers sync.WaitGroup
	closing := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerMu.Lock()
		if closing {
			handlerMu.Unlock()
			return
		}
		handlers.Add(1)
		handlerMu.Unlock()
		defer handlers.Done()
		s.ServeHTTP(w, r)
	})
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 10 * time.Second, MaxHeaderBytes: 8192,
		BaseContext: func(net.Listener) context.Context { return ctx }}
	closed := context.AfterFunc(ctx, func() { server.Close(); udp.SetReadDeadline(time.Now()) })
	defer closed()
	port := listener.Addr().(*net.TCPAddr).Port
	var workers sync.WaitGroup
	workers.Add(2)
	go func() { defer workers.Done(); s.announce(ctx, udp, port) }()
	go func() { defer workers.Done(); s.discover(ctx, udp, port); cancel() }()
	s.record(Event{Operation: "ready", Port: port})
	err = server.Serve(listener)
	if err != nil && ctx.Err() == nil {
		s.record(Event{Operation: "http-failed"})
	}
	handlerMu.Lock()
	closing = true
	handlerMu.Unlock()
	cancel()
	server.Close()
	udp.SetReadDeadline(time.Now())
	workers.Wait()
	handlers.Wait()
}

// PublishPlayback copies facts and wakes waiting polls without network I/O.
func (s *Source) PublishPlayback(state remote.PlaybackState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != state {
		s.state = state
		close(s.changed)
		s.changed = make(chan struct{})
	}
}

func (s *Source) record(event Event) {
	if s.config.Observe != nil {
		s.config.Observe(event)
	}
}

func (s *Source) allowed(ip netip.Addr) bool {
	for _, peer := range s.config.AllowedPeers {
		if ip.Unmap() == peer {
			return true
		}
	}
	return s.control != nil && len(s.config.AllowedPeers) == 0 && ip.Is4() && (ip.IsPrivate() || ip.IsLoopback())
}
