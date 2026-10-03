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

// Artwork availability changes the cycle without changing the stored preference.
func TestMusicArtworkBackgroundCycle(t *testing.T) {
	s := testSession(t)
	library, err := musicviz.Load(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatal(err)
	}
	s.music.library = library
	s.music.index = library.Index(library.Config.Default)
	s.music.loading = true // Keep asset workers out of selection tests.
	art := image.NewRGBA(image.Rect(0, 0, 16, 9))
	s.selection.current.artwork.Backdrop = art
	s.selection.current.artwork.Primary = art
	if got := s.music.backgroundIndex(true, true, false); got != musicviz.ArtworkBackground {
		t.Fatal("artwork was not the initial background")
	}
	for i := range len(library.Config.Backgrounds) {
		s.cycleMusicBackground()
		if got := s.music.backgroundIndex(true, true, false); got != i {
			t.Fatalf("cycle %d: got %d", i, got)
		}
	}
	s.cycleMusicBackground()
	if got := s.music.backgroundIndex(true, true, false); got != musicviz.ArtworkBackground {
		t.Fatal("cycle did not return to artwork")
	}
	// Without either image, cycle from the default effect through each usable
	// preset exactly once. No extra blank artwork slot or spinning disc appears.
	s.selection.current.artwork.Backdrop = nil
	s.selection.current.artwork.Primary = nil
	seen := map[int]bool{}
	for range len(library.Config.Backgrounds) - 1 {
		s.cycleMusicBackground()
		got := s.music.backgroundIndex(false, false, false)
		if got < 0 || library.Config.Backgrounds[got].Type == "spinning" || seen[got] {
			t.Fatalf("unavailable or duplicate option: %d", got)
		}
		seen[got] = true
	}
}

func TestMusicBackgroundRemembersChoiceAcrossPlayback(t *testing.T) {
	library, err := musicviz.Load(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatal(err)
	}
	for i, preset := range library.Config.Backgrounds {
		if preset.Type == "spinning" {
			continue
		}
		m := musicPresentation{library: library, index: i, manual: true}
		for _, available := range []bool{true, false, true} {
			if got := m.backgroundIndex(available, available, false); got != i {
				t.Fatalf("%s changed after stopping or selecting another song", preset.Name)
			}
		}
		if got := m.backgroundIndex(false, false, true); got != i {
			t.Fatal("pending artwork replaced a remembered effect")
		}
	}
	m := musicPresentation{library: library, manual: true, backdrop: true}
	if got := m.backgroundIndex(false, false, true); got != musicviz.ArtworkBackground {
		t.Fatal("artwork loading flashed an effect")
	}
	if got := m.backgroundIndex(false, false, false); got != library.Index(library.Config.Default) {
		t.Fatal("missing background did not use the default effect")
	}
	if got := m.backgroundIndex(true, false, false); got != musicviz.ArtworkBackground {
		t.Fatal("fallback erased the artwork preference")
	}
}

func TestMusicSpinningRequiresCoverNotBackdrop(t *testing.T) {
	library, err := musicviz.Load(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatal(err)
	}
	m := musicPresentation{library: library, index: 0, manual: true}
	if got := m.backgroundIndex(false, true, false); got != 0 {
		t.Fatal("spinning with a cover incorrectly required a backdrop")
	}
	if got := m.backgroundIndex(true, false, false); got != musicviz.ArtworkBackground {
		t.Fatal("missing cover did not fall back to available backdrop")
	}
	if got := m.backgroundIndex(false, false, false); got != library.Index(library.Config.Default) {
		t.Fatal("missing cover did not fall back to default effect")
	}
	if got := m.backgroundIndex(true, true, false); got != 0 {
		t.Fatal("fallback erased the spinning preference")
	}
	library.Config.Default = "Now Spinning"
	if got := m.backgroundIndex(false, false, false); got < 0 || library.Config.Backgrounds[got].Type == "spinning" {
		t.Fatal("unavailable configured default did not fall back safely")
	}
}
