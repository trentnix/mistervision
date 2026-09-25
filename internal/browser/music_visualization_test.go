package browser

import (
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mistervision/internal/diagnostics"
	"mistervision/internal/input/control"
	"mistervision/internal/musicviz"
	"mistervision/internal/playback"
	"mistervision/internal/settings"
)

func TestMissingMusicAssetIsLoggedBeforeNotice(t *testing.T) {
	s := testSession(t)
	path := filepath.Join(t.TempDir(), "log")
	log, err := diagnostics.Open(diagnostics.Config{Enabled: true, Path: path, MaxBytes: 4096})
	if err != nil {
		t.Fatal(err)
	}
	s.config.Diagnostics = log
	s.handleMusicAssets(musicAssetsResult{index: 0, err: fmt.Errorf("private asset path: %w", os.ErrNotExist)})
	if err := log.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var event map[string]any
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatal(err)
	}
	if event["msg"] != "configuration.fallback" || event["configuration"] != "music_visuals" || event["fallback"] != "selected-background-unavailable" || event["error_kind"] != "not-found" {
		t.Fatal(event)
	}
	if strings.Contains(string(data), "private") {
		t.Fatal("asset path exposed")
	}
	if s.music.error == "" {
		t.Fatal("logging removed the visible notice")
	}
}

// TestMusicFallbacksPreservePlayback runs real asset failures
// through the session, then exercises pause and preset selection after recovery.
func TestMusicFallbacksPreservePlayback(t *testing.T) {
	for _, tc := range []struct {
		name, data, kind, fallback string
	}{
		{"missing asset", "missing.png", "not-found", "selected-background-unavailable"},
		{"non-image asset", "private.txt", "invalid", "selected-background-unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := testSession(t)
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "private.txt"), []byte("private asset contents"), 0600); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "events.log")
			log, err := diagnostics.Open(diagnostics.Config{Enabled: true, Path: path, MaxBytes: 65536})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { log.Close() })
			s.config.Diagnostics = log
			s.controller.item.Type = "Audio"
			data := `{"default":"Custom","backgrounds":[{"name":"Custom","type":"image","files":["` + tc.data + `"]},{"name":"Off","type":"none"}]}`
			library, err := musicviz.ParsePresets(settings.Section{Path: filepath.Join(dir, "settings.json"), Data: []byte(data)})
			if err != nil {
				t.Fatal(err)
			}
			s.music.library = library
			s.music.loading = true
			loaded, assetErr := library.LoadAssets(0)
			if assetErr == nil {
				t.Fatal("expected unavailable asset")
			}
			s.handleMusicAssets(musicAssetsResult{music: loaded, index: 0, err: assetErr})
			if s.music.loading || s.music.error == "" || s.music.library != library {
				t.Fatal("asset recovery lost presets or left loading active")
			}
			s.cycleMusicBackground()
			if s.music.index != 1 || s.music.error != "" || s.music.loading {
				t.Fatal("cannot select a working preset after asset failure")
			}
			if !s.controller.running {
				t.Fatal("background failure stopped playback")
			}
			s.controller.Key(control.Open, time.Now())
			expectCommand(t, s.controller.active.controls, playback.SetPaused)
			if err := log.Close(); err != nil {
				t.Fatal(err)
			}
			events, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var event map[string]any
			if err := json.Unmarshal(events, &event); err != nil {
				t.Fatal(err)
			}
			if event["msg"] != "configuration.fallback" || event["configuration"] != "music_visuals" || event["fallback"] != tc.fallback || event["error_kind"] != tc.kind {
				t.Fatal(event)
			}
			if strings.Contains(string(events), "private") || strings.Contains(string(events), dir) {
				t.Fatal("music fallback exposed private data")
			}
		})
	}
}

// Artwork can arrive asynchronously. A manual choice must not be overwritten.
func TestMusicArtworkBackgroundCycle(t *testing.T) {
	s := testSession(t)
	library, err := musicviz.Load(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatal(err)
	}
	s.music.library = library
	s.music.index = library.Index(library.Config.Default)
	if got := s.music.backgroundIndex(true, false); got != s.music.index {
		t.Fatal("missing artwork did not use preset")
	}
	s.selection.current.artwork.Backdrop = image.NewRGBA(image.Rect(0, 0, 16, 9))
	if got := s.music.backgroundIndex(true, true); got != musicviz.ArtworkBackground {
		t.Fatal("available artwork not preferred")
	}
	// Avoid asynchronous preset asset work. Selection is independent of loading.
	s.music.loading = true
	for i := range len(library.Config.Backgrounds) {
		s.cycleMusicBackground()
		if got := s.music.backgroundIndex(true, true); got != i {
			t.Fatalf("cycle %d: got %d", i, got)
		}
	}
	s.cycleMusicBackground()
	if got := s.music.backgroundIndex(true, true); got != musicviz.ArtworkBackground {
		t.Fatal("artwork missing from cycle")
	}
	s.selection.current.artwork.Backdrop = nil
	if got := s.music.backgroundIndex(true, false); got < 0 {
		t.Fatal("unavailable artwork selected")
	}
	for range len(library.Config.Backgrounds) + 1 {
		s.cycleMusicBackground()
		if got := s.music.backgroundIndex(true, false); got < 0 {
			t.Fatal("missing artwork added to cycle")
		}
	}
	if got := s.music.backgroundIndex(true, true); got < 0 {
		t.Fatal("late artwork replaced manual choice")
	}
	s.music.backgroundIndex(false, false)
	if got := s.music.backgroundIndex(true, true); got != musicviz.ArtworkBackground {
		t.Fatal("new playback did not prefer artwork")
	}
}
