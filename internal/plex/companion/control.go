package companion

import (
	"context"
	"encoding/xml"
	"net/http"
	"net/netip"
	"sync"
	"time"

	"mistervision/internal/remote"
)

// ControlConfig binds a receiver to one active server and viewer. Authorize must
// validate the supplied account credential against that viewer, honor cancellation,
// and never use a controller-supplied address. It may run concurrently.
type ControlConfig struct {
	ServerID  string
	Authorize func(context.Context, string) error
}

const (
	messageUnauthorized = "The controller must use the active Plex profile."
	messageUnavailable  = "Remote playback is unavailable."
	messageRejected     = "The application could not accept this command."
	messageOrder        = "The command is older than the last processed command."
	messageVideo        = "Only movies and episodes on the active Plex server are supported."
)

// receiver keeps protocol cursors separate from application playback facts.
// Controller state is bounded and expires. Only credential hashes are retained.
type receiver struct {
	config               ControlConfig
	serial               sync.Mutex
	mu                   sync.Mutex
	ctx                  context.Context
	emit                 func(remote.Command)
	controllers          map[controllerKey]*controller
	itemID, containerKey string
}
type controllerKey struct {
	client, peer string
	credential   [32]byte
}
type controller struct {
	verified, seen time.Time
	commandID      int64
	receivedID     int64
	status         int
	sawPlayback    bool
}

func (c *receiver) start(ctx context.Context, emit func(remote.Command)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ctx, c.emit = ctx, emit
	c.controllers = make(map[controllerKey]*controller)
	c.itemID, c.containerKey = "", ""
}
func (c *receiver) stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.emit = nil
	c.controllers = make(map[controllerKey]*controller)
	c.itemID, c.containerKey = "", ""
}

func (c *receiver) serve(s *Source, w http.ResponseWriter, r *http.Request, q map[string]string, client, token string, event *Event) {
	id := event.CommandID
	if r.URL.Path == "/player/timeline/poll" && q["wait"] != "" && q["wait"] != "0" && q["wait"] != "1" {
		http.Error(w, messagePoll, 400)
		return
	}
	// Public idle polling lets Plex discover/select a receiver before it supplies
	// credentials. No media state or command cursor is exposed without authorization.
	if r.URL.Path == "/player/timeline/poll" && token == "" {
		if client == "" {
			http.Error(w, messagePoll, 400)
			return
		}
		waitPoll(s, r, q["wait"] == "1")
		writeXML(w, timelineContainer{Location: "navigation"})
		return
	}
	peer, _ := netip.ParseAddrPort(r.RemoteAddr)
	entry, err := c.authenticate(r.Context(), client, peer.Addr().String(), token)
	if err != nil {
		http.Error(w, messageUnauthorized, http.StatusForbidden)
		return
	}
	switch r.URL.Path {
	case "/player/timeline/poll":
		if q["wait"] != "1" && id == 0 {
			c.serial.Lock()
			c.mu.Lock()
			entry.receivedID, entry.commandID, entry.status = 0, 0, 0
			entry.sawPlayback = false
			c.mu.Unlock()
			c.serial.Unlock()
		}
		waitPoll(s, r, q["wait"] == "1")
		if r.Context().Err() != nil {
			return
		}
		s.mu.Lock()
		state := s.state
		s.mu.Unlock()
		c.mu.Lock()
		result := c.timeline(state, entry)
		c.mu.Unlock()
		writeXML(w, result)
	case "/player/timeline/unsubscribe":
		// Polling needs no callback subscription. A disconnect is always idempotent.
		writeXML(w, commandResponse{Code: 200, Status: "OK"})
	default:
		cmd, container, err := c.command(r.URL.Path, q)
		if err != nil {
			http.Error(w, messageVideo, http.StatusBadRequest)
			return
		}
		if id < 1 {
			http.Error(w, messageCommandID, http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		state := s.state
		s.mu.Unlock()
		if cmd.Kind != remote.Play && (state.Audio || state.Live) {
			http.Error(w, messageVideo, http.StatusConflict)
			return
		}
		c.dispatch(w, r, entry, id, cmd, container)
	}
}

func waitPoll(s *Source, r *http.Request, wait bool) {
	if !wait {
		return
	}
	s.mu.Lock()
	changed := s.changed
	s.mu.Unlock()
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-changed:
	case <-timer.C:
	case <-r.Context().Done():
	}
}

type commandResponse struct {
	XMLName xml.Name `xml:"Response"`
	Code    int      `xml:"code,attr"`
	Status  string   `xml:"status,attr"`
}

// dispatch serializes command admission, not playback. A positive application
// receipt advances the cursor. Decoder progress remains independently reported.
func (c *receiver) dispatch(w http.ResponseWriter, r *http.Request, entry *controller, id int64, cmd remote.Command, container string) {
	c.serial.Lock()
	defer c.serial.Unlock()
	if r.Context().Err() != nil {
		return
	}
	c.mu.Lock()
	if id <= entry.receivedID {
		status := entry.status
		if id < entry.receivedID {
			status = http.StatusConflict
		}
		c.mu.Unlock()
		if status != 200 {
			http.Error(w, messageOrder, status)
		} else {
			writeXML(w, commandResponse{Code: 200, Status: "OK"})
		}
		return
	}
	if c.emit == nil || c.ctx.Err() != nil {
		c.mu.Unlock()
		http.Error(w, messageUnavailable, http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	stopCancel := context.AfterFunc(c.ctx, cancel)
	defer stopCancel()
	accepted := make(chan bool, 1)
	cmd.Accepted, cmd.Done = accepted, ctx.Done()
	entry.receivedID, entry.status = id, http.StatusGatewayTimeout
	// Holding mu prevents stop from returning while an emission is in progress.
	c.emit(cmd)
	c.mu.Unlock()
	status := http.StatusOK
	select {
	case ok := <-accepted:
		if !ok {
			status = http.StatusConflict
		}
	case <-ctx.Done():
		status = http.StatusGatewayTimeout
	}
	c.mu.Lock()
	entry.status = status
	if status == http.StatusOK {
		entry.commandID = id
	}
	if status == 200 && cmd.Kind == remote.Play {
		c.itemID, c.containerKey = cmd.IDs[0], container
	}
	c.mu.Unlock()
	if status != 200 {
		http.Error(w, messageRejected, status)
		return
	}
	writeXML(w, commandResponse{Code: 200, Status: "OK"})
}
