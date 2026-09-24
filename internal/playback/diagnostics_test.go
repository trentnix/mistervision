package playback

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mistervision/internal/player"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mistervision/internal/diagnostics"
	"mistervision/internal/jellyfin"
	"mistervision/internal/media"
	nativeplayer "mistervision/internal/player/mplayer"
)

func TestPlaybackDiagnosticsRecordMilestonesWithoutMediaSecrets(t *testing.T) {
	dir := t.TempDir()
	player := filepath.Join(dir, "player")
	// The raw decoder output deliberately contains a private URL. It must remain discarded.
	script := "#!/bin/sh\nprintf 'https://private-player/?ApiKey=decoder-secret\nID_VIDEO_WIDTH=640\nID_VIDEO_HEIGHT=360\nID_VIDEO_FPS=23.976\nID_VIDEO_CODEC=ffh264\nANS_VIDEO_STARTED=true\nANS_BUFFERING=false\nANS_TIME_POSITION=3\n'\nwhile IFS= read -r command; do :; done\n"
	if err := os.WriteFile(player, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/Items/item":
			fmt.Fprint(w, `{"Id":"item","Name":"title-secret","Type":"Movie","RunTimeTicks":1000000000}`)
		case "/Videos/item/stream":
			fmt.Fprint(w, "media")
		default:
			fmt.Fprint(w, `{}`)
		}
	}))
	defer server.Close()
	path := filepath.Join(dir, "debug.log")
	log, err := diagnostics.Open(diagnostics.Config{Enabled: true, Path: path, MaxBytes: 32768})
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	client := jellyfin.NewClient(jellyfin.Config{Server: server.URL}, jellyfin.Session{Token: "token-secret"})
	client.Diagnostics = log
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	first, position, buffer := make(chan struct{}, 1), make(chan struct{}, 1), make(chan struct{}, 1)
	done := make(chan error, 1)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		done <- Run(ctx, client, Config{Diagnostics: log, VideoDecoder: nativeplayer.Decoder{Player: player, Width: 640, Height: 480}, AudioDecoder: nativeplayer.Decoder{Width: 640, Height: 480}}, Request{
			Item: media.Item{ID: "item", Type: "Movie"},
			Callbacks: Callbacks{VideoStarted: func() { first <- struct{}{} }, Position: func(int64) {
				select {
				case position <- struct{}{}:
				default:
				}
			}, Buffering: func(bool) { buffer <- struct{}{} }},
		})
	}()
	defer func() {
		cancel()
		select {
		case <-finished:
		case <-time.After(5 * time.Second):
			t.Error("playback did not exit")
		}
	}()
	for _, ch := range []chan struct{}{first, position, buffer} {
		select {
		case <-ch:
		case <-time.After(5 * time.Second):
			t.Fatal("missing playback milestone")
		}
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		data, _ := os.ReadFile(path)
		if strings.Contains(string(data), `"video_codec":"h264"`) && strings.Contains(string(data), `"frame_rate":23.976`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("decoder metadata never reached diagnostics")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	// Wait for cleanup before flushing the borrowed logger.
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("playback did not stop")
	}
	if err := log.Close(); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	for _, event := range []string{"playback.start", "playback.prepared", "playback.first-frame", "playback.first-position", "playback.buffering", "playback.end"} {
		if !strings.Contains(string(data), event) {
			t.Fatalf("missing %s: %s", event, data)
		}
	}
	for _, secret := range []string{"title-secret", "token-secret", "decoder-secret", "private-player", server.URL} {
		if strings.Contains(string(data), secret) {
			t.Fatalf("leaked %s", secret)
		}
	}
}

func TestTranscodeDiagnosticsUsePreparedLimitsWithoutStreamSecrets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "debug.log")
	log, err := diagnostics.Open(diagnostics.Config{Enabled: true, Path: path, MaxBytes: 8192})
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	trace := newPlaybackTrace(log, Config{VideoDecoder: nativeplayer.Decoder{}, AudioDecoder: nativeplayer.Decoder{}}, Request{})
	trace.prepared(&playbackSession{liveTV: true, stream: media.PreparedStream{
		URL:    "http://private-host/stream?ApiKey=private-token",
		Limits: media.StreamLimits{MaxWidth: 640, MaxHeight: 480, VideoBitrate: 8000000, MaxFrameRate: 29.97},
	}})
	if err := log.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	var event map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &event); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]float64{"maxWidth": 640, "maxHeight": 480, "videoBitRate": 8000000, "maxFramerate": 29.97} {
		if event[key] != want {
			t.Errorf("%s: got %v", key, event[key])
		}
	}
	if strings.Contains(string(data), "private-") {
		t.Fatal("logged stream credentials or origin")
	}
}

func TestUnsupportedDisplayStopsBeforeServerPreparation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "debug.log")
	log, err := diagnostics.Open(diagnostics.Config{Enabled: true, Path: path, MaxBytes: 32768})
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	// A nil provider and nonexistent player would fail if validation did not run first.
	err = Run(context.Background(), nil, Config{Diagnostics: log, VideoDecoder: nativeplayer.Decoder{Player: "/missing/player", Width: 3840, Height: 2160}}, Request{Item: media.Item{Type: "Movie"}})
	var display *player.UnsupportedDisplayError
	if !errors.As(err, &display) {
		t.Fatalf("wrong error: %v", err)
	}
	if err := log.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatal(err)
		}
		if event["msg"] == "playback.end" {
			found = true
			if event["stage"] != "decoder-configuration" || event["error_kind"] != "unsupported-display" || event["failed"] != true {
				t.Fatalf("wrong diagnosis: %v", event)
			}
		}
	}
	if !found {
		t.Fatal("missing playback result")
	}
}

func TestStreamDiagnosticsSeparateSourceRequestAndDecoder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "debug.log")
	log, err := diagnostics.Open(diagnostics.Config{Enabled: true, Path: path, MaxBytes: 32768})
	if err != nil {
		t.Fatal(err)
	}
	trace := newPlaybackTrace(log, Config{}, Request{})
	trace.prepared(&playbackSession{
		item: media.Item{MediaSources: []media.MediaSource{
			{ID: "wrong", MediaStreams: []media.MediaStream{{Type: "Video", Width: 1, Height: 1}}},
			{ID: "selected", MediaStreams: []media.MediaStream{{Type: "Video", Codec: "hevc", Width: 3840, Height: 2160, RealFrameRate: 23.976, BitRate: 40000000}}},
		}},
		stream: media.PreparedStream{SourceID: "selected", Delivery: media.Delivery("plex", "transcode", "https://private-host/?token=private-secret"), Limits: media.StreamLimits{MaxWidth: 720, MaxHeight: 576, MaxFrameRate: 30}},
	})
	trace.videoFormat("playback.decoder-input", media.VideoFormat{Codec: "h264", Width: 640, Height: 360, FrameRate: 23.976})
	trace.videoFormat("playback.decoder-input", media.VideoFormat{Codec: "https://private-secret", FrameRate: -1})
	log.Close()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var source, request, delivery map[string]any
	var inputs []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var e map[string]any
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatal(err)
		}
		switch e["msg"] {
		case "playback.source":
			source = e
		case "playback.prepared":
			request = e
		case "playback.delivery":
			delivery = e
		case "playback.decoder-input":
			inputs = append(inputs, e)
		}
	}
	if source["width"] != float64(3840) || source["video_codec"] != "hevc" || request["maxWidth"] != float64(720) {
		t.Fatalf("source/request mixed: %v %v", source, request)
	}
	if delivery["server_video_decision"] != "unknown" || delivery["address_class"] != "unknown" {
		t.Fatal(delivery)
	}
	if len(inputs) != 3 || inputs[0]["width"] != nil || inputs[1]["width"] != float64(640) || inputs[2]["frame_rate"] != nil {
		t.Fatal(inputs)
	}
	if strings.Contains(string(data), "private-") || strings.Contains(string(data), "selected") {
		t.Fatal("private data in logs")
	}
}
