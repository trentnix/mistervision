package rendering

import (
	"bytes"
	"os"
	"testing"
	"time"

	"mistervision/internal/media"
	"mistervision/internal/ui"
)

func TestMusicOptionsPreserveUnderlyingScreen(t *testing.T) {
	item := media.Item{Type: "Audio", Name: "Example song", Artists: []string{"Example artist"}, Album: "Example album", RunTimeTicks: 6000000000}
	for _, height := range []int{240, 288} {
		s := Scene{Content: Content{Detail: &item}, Audio: true, Playback: PlaybackPresentation{Active: true, Audio: true}, Now: time.Unix(100, 0)}
		r := NewRenderer()
		base := bytes.Clone(r.Render(640, height, s).UI)
		s.Options = &OptionsMenu{Title: "Music options", Rows: []string{"Repeat: All", "Shuffle: Off"}}
		opened := bytes.Clone(r.Render(640, height, s).UI)
		if bytes.Equal(base, opened) {
			t.Fatal("options were not rendered")
		}
		s.Options = nil
		if !bytes.Equal(base, r.Render(640, height, s).UI) {
			t.Fatal("dismissing options changed underlying screen")
		}
		if dir := os.Getenv("MEDIA_OPTIONS_PREVIEW_DIR"); dir != "" && height == 240 {
			c := ui.New(640, height)
			copy(c.Pixels, opened)
			writeSetupPreview(t, dir, "music-options.png", c)
		}
	}
}

func TestWatchedPreviewFeedbackStaysInline(t *testing.T) {
	for _, height := range []int{240, 288} {
		for _, overview := range []string{"", "A group of young hackers uncover a conspiracy and work together to expose it."} {
			item := media.Item{Type: "Movie", Name: "Hackers", ProductionYear: 1995, Overview: overview, RunTimeTicks: 63000000000}
			s := Scene{Content: Content{Detail: &item}, OptionsAvailable: true, Now: time.Unix(100, 0)}
			c := ui.New(640, height)
			cache := &sceneCache{}
			c.Typeface = cache.typeface(c.Width, c.Height)
			p := screenPainter{canvas: c, cache: cache, scene: s, width: 640, height: height, safeY: safeY(640, height), bottom: height - 8 - safeY(640, height)}
			for _, watched := range []bool{false, true} {
				item.UserData.Played = watched
				want := "Mark watched"
				if watched {
					want = "Mark unwatched"
				}
				found := false
				for _, row := range p.details() {
					for _, h := range row {
						if h.key == "X" && h.description == want {
							found = true
						}
					}
				}
				if !found {
					t.Fatalf("missing direct action %q", want)
				}
			}
			item.UserData.Played = false
			r := NewRenderer()
			base := bytes.Clone(r.Render(640, height, s).UI)
			s.WatchedMessage = "Marked watched."
			feedback := bytes.Clone(r.Render(640, height, s).UI)
			if bytes.Equal(base, feedback) {
				t.Fatal("missing inline feedback")
			}
			if !bytes.Equal(base[:len(base)/2], feedback[:len(feedback)/2]) {
				t.Fatal("feedback obscured or moved the preview")
			}
			if dir := os.Getenv("MEDIA_OPTIONS_PREVIEW_DIR"); dir != "" && height == 240 && overview != "" {
				item.UserData.Played = true
				copy(c.Pixels, r.Render(640, height, s).UI)
				writeSetupPreview(t, dir, "watched-preview.png", c)
			}
		}
	}
}
