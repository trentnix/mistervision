package rendering

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"mistervision/internal/media"
	"mistervision/internal/ui"
)

func TestVideoWatchStatusSubtext(t *testing.T) {
	for _, kind := range []string{"Movie", "Episode"} {
		for _, tc := range []struct {
			name               string
			duration, position int64
			played             bool
			want               string
			color              uint32
		}{
			{"unwatched", 150 * 600000000, 0, false, "150 min", dimColor},
			{"resume", 150 * 600000000, 754 * 10000000, false, "150 min - resume 12:34", 0xffc040},
			{"watched", 150 * 600000000, 0, true, "150 min - Watched", 0x90ee90},
			{"no duration", 0, 0, true, "Watched", 0x90ee90},
		} {
			t.Run(kind+"/"+tc.name, func(t *testing.T) {
				item := media.Item{Type: kind, RunTimeTicks: tc.duration}
				item.UserData.Played = tc.played
				item.UserData.PlaybackPositionTicks = tc.position
				got, color := subtitle(item)
				if got != tc.want || color != tc.color {
					t.Fatalf("subtitle = %q (%06x), want %q (%06x)", got, color, tc.want, tc.color)
				}
			})
		}
	}
}

func TestWatchedStatePreservesListTitleWidth(t *testing.T) {
	for _, height := range []int{240, 288, 360, 480} {
		p := screenPainter{canvas: ui.New(640, height), width: 640, height: height, cache: &sceneCache{}}
		item := media.Item{Name: strings.Repeat("Long title ", 10), Type: "Movie", RunTimeTicks: 150 * 600000000}
		title, _ := p.listRowText(item, 367, true, false)
		item.UserData.Played = true
		watched, _ := p.listRowText(item, 367, true, false)
		if title != watched || title.Bounds().Dx() != 367 {
			t.Fatal("watched status changed the title or reserved a badge column")
		}
	}
}

func TestWatchedListPreview(t *testing.T) {
	dir := os.Getenv("WATCHED_PREVIEW_DIR")
	if dir == "" {
		t.Skip("set WATCHED_PREVIEW_DIR to write previews")
	}
	for _, height := range []int{240, 360} {
		items := []media.Item{{Name: "Contact", Type: "Movie", ProductionYear: 1997, RunTimeTicks: 150 * 600000000}, {Name: "Ghost in the Shell", Type: "Movie", ProductionYear: 1995, RunTimeTicks: 83 * 600000000}, {Name: "Hackers", Type: "Movie", ProductionYear: 1995, RunTimeTicks: 105 * 600000000}, {Name: "The 36th Chamber of Shaolin", Type: "Movie", ProductionYear: 1978, RunTimeTicks: 115 * 600000000}}
		items[0].UserData.Played = true
		items[1].UserData.Played = true
		items[2].UserData.PlaybackPositionTicks = 7540000000
		scene := Scene{ListMode: true, Now: time.Unix(0, 0), Content: Content{Title: "Movies"}}
		scene.Content.Page.Items = items
		c := ui.New(640, height)
		copy(c.Pixels, NewRenderer().Render(640, height, scene).UI)
		name := fmt.Sprintf("watched-list-%d.png", height)
		if height == 240 {
			writeSetupPreview(t, dir, name, c)
		} else {
			writeRasterPreview(t, dir, name, c)
		}
	}
}
