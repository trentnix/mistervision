package companion

import (
	"context"
	"encoding/xml"
	"errors"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"mistervision/internal/remote"
)

func controlled(t *testing.T, emit func(remote.Command)) *Source {
	t.Helper()
	s, err := New(Config{Identifier: "player", Name: "MiSTerVision", Version: "test", Control: &ControlConfig{ServerID: "server", Authorize: func(_ context.Context, token string) error {
		if token != "viewer-token" {
			return errors.New("wrong viewer")
		}
		return nil
	}}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.control.start(ctx, emit)
	t.Cleanup(func() { cancel(); s.control.stop() })
	return s
}
func controlRequest(s *Source, path, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", path, nil)
	r.RemoteAddr = "192.168.1.50:1234"
	r.Header.Set("X-Plex-Client-Identifier", "controller")
	if token != "" {
		r.Header.Set("X-Plex-Token", token)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}
func TestControlRequiresActiveViewerAndLAN(t *testing.T) {
	var emitted int
	s := controlled(t, func(c remote.Command) { emitted++; c.Acknowledge(true) })
	for _, token := range []string{"", "other-viewer", "transient-token"} {
		w := controlRequest(s, "/player/playback/stop?commandID=1", token)
		if w.Code != 403 {
			t.Fatalf("unauthorized status %d", w.Code)
		}
	}
	r := httptest.NewRequest("GET", "/resources", nil)
	r.RemoteAddr = "203.0.113.5:1234"
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 403 || emitted != 0 {
		t.Fatal("unauthorized access accepted")
	}
	s.PublishPlayback(remote.PlaybackState{Status: remote.Playing, ItemID: "private-title"})
	w = controlRequest(s, "/player/timeline/poll", "")
	if w.Code != 200 || strings.Contains(w.Body.String(), "private-title") || strings.Contains(w.Body.String(), "<Timeline") {
		t.Fatal("anonymous poll exposes playback")
	}
}
func TestVideoCommandsUseSharedSemantics(t *testing.T) {
	var commands []remote.Command
	s := controlled(t, func(c remote.Command) { commands = append(commands, c); c.Acknowledge(true) })
	paths := []string{
		"/player/playback/playMedia?commandID=1&machineIdentifier=server&key=/library/metadata/123&containerKey=/playQueues/77%3Fown%3D1&offset=42000&address=attacker.example&port=80&token=untrusted",
		"/player/playback/pause?commandID=2&type=video",
		"/player/playback/play?commandID=3&type=video",
		"/player/playback/seekTo?commandID=4&type=video&offset=125000",
		"/player/playback/stop?commandID=5&type=video",
	}
	for _, path := range paths {
		w := controlRequest(s, path, "viewer-token")
		if w.Code != 200 {
			t.Fatalf("status %d: %s", w.Code, w.Body)
		}
	}
	want := []remote.Kind{remote.Play, remote.Pause, remote.Resume, remote.Seek, remote.Stop}
	for i, c := range commands {
		if c.Kind != want[i] {
			t.Fatalf("kind %s", c.Kind)
		}
	}
	if commands[0].IDs[0] != "123" || *commands[0].Position != 420000000 || *commands[3].Position != 1250000000 {
		t.Fatal("identifier or position translation failed")
	}
}
func TestDuplicateAndOldCommandsDoNotExecuteAgain(t *testing.T) {
	var emitted int
	s := controlled(t, func(c remote.Command) { emitted++; c.Acknowledge(true) })
	for _, id := range []string{"2", "2", "1"} {
		controlRequest(s, "/player/playback/stop?commandID="+id, "viewer-token")
	}
	if emitted != 1 {
		t.Fatalf("emitted %d", emitted)
	}
}
func TestRejectedCommandsAndTimelineFacts(t *testing.T) {
	s := controlled(t, func(c remote.Command) { c.Acknowledge(false) })
	if w := controlRequest(s, "/player/playback/stop?commandID=1", "viewer-token"); w.Code != 409 {
		t.Fatalf("status %d", w.Code)
	}
	// A processed receipt never substitutes the requested time for decoder facts.
	s.PublishPlayback(remote.PlaybackState{Status: remote.Seeking, ItemID: "123", PositionTicks: 40000000, DurationTicks: 900000000})
	w := controlRequest(s, "/player/timeline/poll", "viewer-token")
	var result timelineContainer
	if xml.Unmarshal(w.Body.Bytes(), &result) != nil || len(result.Timelines) != 1 {
		t.Fatalf("timeline %s", w.Body)
	}
	if result.Timelines[0].State != "buffering" || result.Timelines[0].Time != 4000 {
		t.Fatalf("facts %+v", result)
	}
}
func TestControlRejectsMalformedMediaAndOffsets(t *testing.T) {
	s := controlled(t, func(c remote.Command) { t.Error("invalid command emitted") })
	for _, path := range []string{
		"/player/playback/playMedia?machineIdentifier=other&key=/library/metadata/123",
		"/player/playback/playMedia?machineIdentifier=server&key=https://attacker.example/movie",
		"/player/playback/playMedia?machineIdentifier=server&key=/library/metadata/../123",
		"/player/playback/playMedia?machineIdentifier=server&key=/library/metadata/123&containerKey=https://attacker.example/playQueues/77",
		"/player/playback/playMedia?machineIdentifier=server&key=/library/metadata/123&containerKey=/playQueues/77%3Ftoken%3Dsecret",
		"/player/playback/playMedia?machineIdentifier=server&key=/library/metadata/123&type=music",
		"/player/playback/seekTo?offset=-1",
		"/player/playback/seekTo?offset=9223372036854775807",
		"/player/playback/seekTo",
	} {
		w := controlRequest(s, path+"&commandID=1", "viewer-token")
		if w.Code != 400 {
			t.Fatalf("status %d", w.Code)
		}
	}
}
func TestStoppedTimelineDoesNotRepeatedlyResetWebQueue(t *testing.T) {
	s := controlled(t, func(c remote.Command) { c.Acknowledge(true) })
	s.PublishPlayback(remote.PlaybackState{Status: remote.Playing, ItemID: "123"})
	controlRequest(s, "/player/timeline/poll", "viewer-token")
	s.PublishPlayback(remote.PlaybackState{Status: remote.Stopped, ItemID: "123"})
	for i := range 2 {
		w := controlRequest(s, "/player/timeline/poll", "viewer-token")
		var result timelineContainer
		_ = xml.Unmarshal(w.Body.Bytes(), &result)
		if len(result.Timelines) != 1-i {
			t.Fatalf("poll %d: %+v", i, result)
		}
	}
}
func TestAuthorizationCacheAndSessionRestart(t *testing.T) {
	var calls atomic.Int32
	s := controlled(t, func(c remote.Command) { c.Acknowledge(true) })
	s.control.config.Authorize = func(context.Context, string) error { calls.Add(1); return nil }
	for range 2 {
		controlRequest(s, "/player/timeline/poll", "viewer-token")
	}
	if calls.Load() != 1 {
		t.Fatal("credential not cached")
	}
	s.control.stop()
	s.control.start(context.Background(), func(c remote.Command) { c.Acknowledge(true) })
	controlRequest(s, "/player/timeline/poll", "viewer-token")
	if calls.Load() != 2 {
		t.Fatal("authorization crossed sessions")
	}
}

// Browsers must be able to verify the timeline's receiver identity. A reverse
// proxy can still replace this header, so real-client checks must cover its route.
func TestTimelineExposesReceiverIdentityToBrowsers(t *testing.T) {
	s := controlled(t, func(c remote.Command) { c.Acknowledge(true) })
	w := controlRequest(s, "/player/timeline/poll", "viewer-token")
	if w.Header().Get("X-Plex-Client-Identifier") != "player" || w.Header().Get("Access-Control-Expose-Headers") != "X-Plex-Client-Identifier" {
		t.Fatal("browser cannot verify the timeline's receiver identity")
	}
}
