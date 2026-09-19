package companion

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"mistervision/internal/remote"
)

func probe(t *testing.T) *Source {
	t.Helper()
	s, err := New(Config{Identifier: "probe", Name: "MiSTerVision Probe", Version: "test", Listen: "127.0.0.1:0", AllowedPeers: []netip.Addr{netip.MustParseAddr("127.0.0.1")}})
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func request(s *Source, method, path string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, nil)
	r.RemoteAddr = "127.0.0.1:54321"
	r.Header.Set("X-Plex-Client-Identifier", "controller")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}

func TestResourcesMatchDiagnosticDiscovery(t *testing.T) {
	s := probe(t)
	w := request(s, "GET", "/resources")
	var body resources
	if w.Code != 200 || xml.Unmarshal(w.Body.Bytes(), &body) != nil || len(body.Players) != 1 {
		t.Fatalf("resources: %d %s", w.Code, w.Body)
	}
	p := body.Players[0]
	if p.ID != "probe" || p.Capabilities != "timeline,playback" || w.Header().Get("X-Plex-Client-Identifier") != p.ID {
		t.Fatalf("identity: %+v", p)
	}
	advertisement := s.advertisement(32433)
	for _, v := range []string{"Resource-Identifier: probe\r\n", "Port: 32433\r\n", "Protocol-Capabilities: timeline,playback\r\n"} {
		if !strings.Contains(advertisement, v) {
			t.Fatalf("missing %q", v)
		}
	}
}

func TestProbeRejectsPlaybackWithoutLeakingCredentials(t *testing.T) {
	s := probe(t)
	var event Event
	s.config.Observe = func(e Event) { event = e }
	w := request(s, "GET", "/player/playback/playMedia?commandID=7&token=private-secret&address=evil.example&key=/library/metadata/123")
	if w.Code != 501 || event.Status != 501 || event.CommandID != 7 || !event.HasToken {
		t.Fatalf("rejection: %d %+v", w.Code, event)
	}
	encoded, _ := json.Marshal(event)
	if strings.Contains(string(encoded)+w.Body.String(), "private-secret") || strings.Contains(string(encoded), "evil.example") {
		t.Fatal("private request reached output")
	}
	w = request(s, "GET", "/player/timeline/poll?commandID=7")
	if !strings.Contains(w.Body.String(), `commandID="0"`) {
		t.Fatal("rejected command reported as completed")
	}
}

func TestProbeRequestBoundaries(t *testing.T) {
	for _, tc := range []struct {
		path   string
		status int
	}{
		{"/resources?X-Plex-Target-Client-Identifier=other", 404},
		{"/player/timeline/poll?wait=bad", 400},
		{"/player/timeline/poll?commandID=-1", 400},
		{"/player/timeline/poll?commandID=1&commandID=2", 400},
		{"/resources?token=%xx", 400},
		{"/player/timeline/subscribe?port=80&protocol=http", 501},
		{"/unknown", 404},
	} {
		t.Run(tc.path, func(t *testing.T) {
			if w := request(probe(t), "GET", tc.path); w.Code != tc.status {
				t.Fatalf("status %d, want %d", w.Code, tc.status)
			}
		})
	}
	s := probe(t)
	r := httptest.NewRequest("GET", "/resources", nil)
	r.RemoteAddr = "192.168.1.77:1111"
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("unapproved peer accepted")
	}
	if w = request(s, "POST", "/resources"); w.Code != 405 {
		t.Fatal("unsupported method accepted")
	}
	if w = request(s, "OPTIONS", "/player/timeline/poll"); w.Code != 204 || w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatal("missing preflight")
	}
	for range cap(s.requests) {
		s.requests <- struct{}{}
	}
	if w = request(s, "GET", "/resources"); w.Code != 503 {
		t.Fatal("concurrency limit ignored")
	}
}

func TestTimelinePublishesStateAndConvertsTicks(t *testing.T) {
	s := probe(t)
	for _, tc := range []struct {
		status remote.PlaybackStatus
		want   string
	}{{remote.Loading, "buffering"}, {remote.Seeking, "buffering"}, {remote.Paused, "paused"}, {remote.Playing, "playing"}, {remote.Failed, "error"}, {remote.Stopped, "stopped"}} {
		s.PublishPlayback(remote.PlaybackState{Status: tc.status, ItemID: "private-item", PositionTicks: 123450000, DurationTicks: 900000000})
		w := request(s, "GET", "/player/timeline/poll")
		var body timelineContainer
		if xml.Unmarshal(w.Body.Bytes(), &body) != nil || len(body.Timelines) != 3 {
			t.Fatalf("invalid timeline: %s", w.Body)
		}
		if got := body.Timelines[0]; got.State != tc.want || got.Time != 12345 || got.Duration != 90000 || got.Controllable != "" {
			t.Fatalf("timeline: %+v", got)
		}
		if strings.Contains(w.Body.String(), "private-item") {
			t.Fatal("probe exposed media identity")
		}
	}
}

func TestPollCancellationAndConcurrentPublication(t *testing.T) {
	s := probe(t)
	ctx, cancel := context.WithCancel(t.Context())
	r := httptest.NewRequest("GET", "/player/timeline/poll?wait=1", nil).WithContext(ctx)
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("X-Plex-Client-Identifier", "controller")
	done := make(chan struct{})
	go func() { defer close(done); s.ServeHTTP(httptest.NewRecorder(), r) }()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("poll ignored cancellation")
	}
	var wg sync.WaitGroup
	for i := range 4 {
		wg.Go(func() {
			for j := range 100 {
				s.PublishPlayback(remote.PlaybackState{ItemID: "item", PositionTicks: int64(i + j)})
				request(s, "GET", "/player/timeline/poll")
			}
		})
	}
	wg.Wait()
}

func TestDiscoveryAnswersAllowedSearchAndStops(t *testing.T) {
	s := probe(t)
	listener, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan struct{})
	go func() { defer close(done); s.discover(ctx, listener, 32433) }()
	client, err := net.DialUDP("udp4", nil, listener.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	client.SetDeadline(time.Now().Add(time.Second))
	client.Write([]byte("M-SEARCH * HTTP/1.0\r\n\r\n"))
	buffer := make([]byte, 4096)
	n, err := client.Read(buffer)
	if err != nil || !strings.Contains(string(buffer[:n]), "MiSTerVision Probe") {
		t.Fatalf("discovery: %v %s", err, buffer[:n])
	}
	cancel()
	listener.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("discovery did not stop")
	}
}

func TestConfigRequiresExplicitPrivatePeers(t *testing.T) {
	s := probe(t)
	for _, peers := range [][]netip.Addr{nil, {netip.MustParseAddr("8.8.8.8")}, {netip.MustParseAddr("::1")}} {
		c := s.config
		c.AllowedPeers = peers
		if _, err := New(c); err == nil {
			t.Fatal("invalid peers accepted")
		}
	}
	c := s.config
	c.Name = "header\r\nInjected: true"
	if _, err := New(c); err == nil {
		t.Fatal("header injection accepted")
	}
}

func TestRunReportsPortConflict(t *testing.T) {
	s := probe(t)
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	s.config.Listen = listener.Addr().String()
	var event Event
	s.config.Observe = func(e Event) { event = e }
	s.Run(t.Context(), func(remote.Command) { t.Error("probe emitted playback") })
	if event.Operation != "listen-failed" {
		t.Fatalf("port conflict: %+v", event)
	}
}

var _ http.Handler = (*Source)(nil)

func TestRejectedMethodsStillProduceProtocolEvidence(t *testing.T) {
	s := probe(t)
	var event Event
	s.config.Observe = func(e Event) { event = e }
	w := request(s, "POST", "/player/playback/playMedia?token=private-secret")
	if w.Code != 405 || event.Status != 405 || event.Method != "POST" || event.Operation != "/player/playback/playMedia" {
		t.Fatalf("missing rejection evidence: %+v", event)
	}
	w = request(s, "OPTIONS", "/player/playback/playMedia")
	if w.Code != 204 || event.Status != 204 || event.Method != "OPTIONS" {
		t.Fatalf("missing preflight evidence: %+v", event)
	}
}

func TestIdlePollDoesNotCancelControllersPendingPlayback(t *testing.T) {
	s := probe(t)
	// An already connected controller can be choosing Resume or Start Over.
	// Repeating stopped timelines here makes Plex Web discard that choice.
	for range 3 {
		w := request(s, "GET", "/player/timeline/poll?commandID=0")
		var body timelineContainer
		if err := xml.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if len(body.Timelines) != 0 || body.CommandID != 0 || body.Location != "navigation" {
			t.Fatalf("idle poll synthesized a stop: %s", w.Body)
		}
	}
	// Finishing real playback still communicates its final state and position.
	s.PublishPlayback(remote.PlaybackState{ItemID: "movie", Status: remote.Stopped, PositionTicks: 12340000})
	w := request(s, "GET", "/player/timeline/poll?commandID=8")
	var body timelineContainer
	if err := xml.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Timelines) != 3 || body.Timelines[0].State != "stopped" || body.Timelines[0].Time != 1234 || body.CommandID != 0 {
		t.Fatalf("actual stop lost: %s", w.Body)
	}
}
