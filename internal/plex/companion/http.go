package companion

import (
	"encoding/xml"
	"errors"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	"mistervision/internal/remote"
)

// The isolated probe advertises playback to exercise controller dispatch, but
// rejects every playback operation. This is not a production capability claim.
const probeCapabilities = "timeline,playback"

const (
	messageUnsupported = "This compatibility probe does not execute playback commands."
	messagePeerDenied  = "Peer is not allowed."
	messageBusy        = "Probe is busy."
	messageTooLarge    = "Request is too large."
	messageMethod      = "Use GET."
	messageParameters  = "Invalid parameters."
	messageCommandID   = "Invalid command ID."
	messagePoll        = "Invalid poll."
)

// ServeHTTP applies LAN, request-size, concurrency, and routing checks.
// Authenticated receivers delegate controls and private timelines to the active
// viewer policy. Diagnostic receivers reject controls and retain no tokens.
func (s *Source) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	event := Event{Operation: operation(r.URL.Path), CommandID: -1, Method: "other"}
	switch r.Method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodHead, http.MethodOptions:
		event.Method = r.Method
	}
	response := &probeResponse{ResponseWriter: w, status: http.StatusOK}
	s.serve(response, r, &event)
	event.Status = response.status
	s.record(event)
}

// probeResponse records only the HTTP result. It never inspects response bodies.
type probeResponse struct {
	http.ResponseWriter
	status int
}

func (w *probeResponse) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (s *Source) serve(w http.ResponseWriter, r *http.Request, event *Event) {
	peer, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil || !s.allowed(peer.Addr()) {
		http.Error(w, messagePeerDenied, http.StatusForbidden)
		return
	}
	select {
	case s.requests <- struct{}{}:
		defer func() { <-s.requests }()
	default:
		http.Error(w, messageBusy, http.StatusServiceUnavailable)
		return
	}
	if len(r.RequestURI) > 8192 {
		http.Error(w, messageTooLarge, http.StatusRequestURITooLong)
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "X-Plex-Client-Identifier, X-Plex-Target-Client-Identifier, X-Plex-Device-Name, X-Plex-Token, X-Plex-Product, X-Plex-Version, X-Plex-Platform, X-Plex-Device, X-Plex-Provides")
	w.Header().Set("Access-Control-Expose-Headers", "X-Plex-Client-Identifier")
	w.Header().Set("X-Plex-Client-Identifier", s.config.Identifier)
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, messageMethod, http.StatusMethodNotAllowed)
		return
	}
	query, err := parseQuery(r)
	if err != nil {
		http.Error(w, messageParameters, http.StatusBadRequest)
		return
	}
	header := func(name string) string {
		if v := r.Header.Get(name); v != "" {
			return v
		}
		return query[name]
	}
	if target := header("X-Plex-Target-Client-Identifier"); target != "" && target != s.config.Identifier {
		http.NotFound(w, r)
		return
	}
	event.HasClient = header("X-Plex-Client-Identifier") != ""
	event.HasToken = header("X-Plex-Token") != "" || query["token"] != ""
	if value, ok := query["commandID"]; ok {
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil || id < 0 {
			http.Error(w, messageCommandID, http.StatusBadRequest)
			return
		}
		event.CommandID = id
	}
	if s.control != nil && r.URL.Path != "/resources" {
		s.control.serve(s, w, r, query, header("X-Plex-Client-Identifier"), header("X-Plex-Token"), event)
		return
	}
	status := http.StatusOK
	switch r.URL.Path {
	case "/resources":
		writeXML(w, resources{Players: []resource{{Title: s.config.Name, ID: s.config.Identifier, Product: "MiSTerVision", Version: s.config.Version, Platform: "Linux", Protocol: "plex", ProtocolVersion: "1", Capabilities: probeCapabilities, Class: "pc"}}})
	case "/player/timeline/poll":
		if !event.HasClient || (query["wait"] != "" && query["wait"] != "0" && query["wait"] != "1") {
			status = 400
			http.Error(w, messagePoll, status)
			return
		}
		s.mu.Lock()
		changed := s.changed
		s.mu.Unlock()
		if query["wait"] == "1" {
			timer := time.NewTimer(time.Second)
			defer timer.Stop()
			select {
			case <-changed:
			case <-timer.C:
			case <-r.Context().Done():
				return
			}
		}
		s.mu.Lock()
		state := s.state
		s.mu.Unlock()
		// Zero is the initial command baseline. Never echo a received command
		// as completed: this probe does not apply playback operations.
		writeXML(w, timelineXML(state))
	default:
		if strings.HasPrefix(r.URL.Path, "/player/") {
			status = http.StatusNotImplemented
			http.Error(w, messageUnsupported, status)
		} else {
			status = 404
			http.NotFound(w, r)
		}
	}
}

type resources struct {
	XMLName xml.Name   `xml:"MediaContainer"`
	Players []resource `xml:"Player"`
}
type resource struct {
	Title           string `xml:"title,attr"`
	ID              string `xml:"machineIdentifier,attr"`
	Product         string `xml:"product,attr"`
	Version         string `xml:"version,attr"`
	Platform        string `xml:"platform,attr"`
	Protocol        string `xml:"protocol,attr"`
	ProtocolVersion string `xml:"protocolVersion,attr"`
	Capabilities    string `xml:"protocolCapabilities,attr"`
	Class           string `xml:"deviceClass,attr"`
}
type timelineContainer struct {
	XMLName   xml.Name   `xml:"MediaContainer"`
	CommandID int64      `xml:"commandID,attr"`
	Location  string     `xml:"location,attr"`
	Timelines []timeline `xml:"Timeline"`
}
type timeline struct {
	Type               string `xml:"type,attr"`
	State              string `xml:"state,attr"`
	Time               int64  `xml:"time,attr"`
	Duration           int64  `xml:"duration,attr"`
	Controllable       string `xml:"controllable,attr"`
	MachineIdentifier  string `xml:"machineIdentifier,attr,omitempty"`
	ProviderIdentifier string `xml:"providerIdentifier,attr,omitempty"`
	Key                string `xml:"key,attr,omitempty"`
	RatingKey          string `xml:"ratingKey,attr,omitempty"`
	ContainerKey       string `xml:"containerKey,attr,omitempty"`
}

func timelineXML(state remote.PlaybackState) timelineContainer {
	result := timelineContainer{Location: "navigation"}
	// Plex Web treats every stopped timeline as a queue-reset event, which
	// also closes a pending resume dialog. An idle receiver with no item has
	// no playback transition to report. Keep the connection alive without
	// synthesizing stop events. A real stop retains its item and is reported.
	if state.ItemID == "" {
		return result
	}
	result.Timelines = []timeline{{Type: "video", State: "stopped"}, {Type: "music", State: "stopped"}, {Type: "photo", State: "stopped"}}
	i := 0
	if state.Audio {
		i = 1
	}
	t := &result.Timelines[i]
	t.State = string(state.Status)
	switch state.Status {
	case remote.Loading, remote.Buffering, remote.Seeking:
		t.State = "buffering"
	case "":
		t.State = "stopped"
	}
	t.Time = max(0, state.PositionTicks) / 10000
	t.Duration = max(0, state.DurationTicks) / 10000
	// Item paths and credentials are deliberately absent from this probe.
	if t.State != "stopped" && t.State != "error" {
		if state.Audio {
			result.Location = "fullScreenMusic"
		} else {
			result.Location = "fullScreenVideo"
		}
	}
	return result
}
func writeXML(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	_ = xml.NewEncoder(w).Encode(value)
}
func operation(path string) string {
	switch path {
	case "/resources", "/player/timeline/poll", "/player/timeline/subscribe", "/player/timeline/unsubscribe", "/player/playback/playMedia", "/player/playback/play", "/player/playback/pause", "/player/playback/stop", "/player/playback/seekTo":
		return path
	default:
		return "unsupported"
	}
}

// parseQuery rejects malformed or repeated parameters before protocol dispatch.
func parseQuery(r *http.Request) (map[string]string, error) {
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(values))
	for key, values := range values {
		if len(values) != 1 {
			return nil, errors.New("repeated parameter")
		}
		result[key] = values[0]
	}
	return result, nil
}
