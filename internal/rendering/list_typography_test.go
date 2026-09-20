package rendering

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"mistervision/internal/media"
	"mistervision/internal/ui"
)

func TestListSelectionBalancesVisibleText(t *testing.T) {
	count := 67
	for _, height := range []int{240, 288} {
		for _, tc := range []struct {
			name string
			root bool
			item media.Item
		}{
			{"home", true, media.Item{Name: "Movies", CollectionType: "movies", LibraryCount: &count}},
			{"movie", false, media.Item{Name: "Akira", Type: "Movie", RunTimeTicks: 67 * 60 * 10000000}},
			{"season", false, media.Item{Name: "Season 1", Type: "Season", ChildCount: 12}},
			{"unicode", false, media.Item{Name: "日本語 — Café", Type: "Movie", RunTimeTicks: 67 * 60 * 10000000}},
		} {
			t.Run(fmt.Sprintf("%d/%s", height, tc.name), func(t *testing.T) {
				rows := VisibleRows(640, height)
				if tc.root {
					rows = HomeVisibleRows(640, height)
				}
				items := make([]media.Item, rows)
				for i := range items {
					items[i] = tc.item
				}
				selected := len(items) - 1
				scene := Scene{Root: tc.root, ListMode: true, Now: time.Unix(0, 0), Content: Content{Selected: selected, Page: media.Page{Items: items}}}
				pixels := renderScene(ui.New(640, height), nil, scene, Animation{Row: float64(selected)})
				blue := []byte{0x7c, 0x37, 0x0d}
				first, last := height, -1
				for y := 0; y < height; y++ {
					off := (y*640 + 20) * 4
					if bytes.Equal(pixels[off:off+3], blue) {
						first = min(first, y)
						last = y
					}
				}
				if last < first {
					t.Fatal("missing selection")
				}
				top, bottom := height, -1
				for y := first; y <= last; y++ {
					for x := 24; x < 200; x++ {
						off := (y*640 + x) * 4
						if !bytes.Equal(pixels[off:off+3], blue) {
							top = min(top, y)
							bottom = max(bottom, y)
						}
					}
				}
				if bottom < top || top-first != last-bottom || top-first < 1 {
					t.Fatalf("selection padding top=%d bottom=%d", top-first, last-bottom)
				}
			})
		}
	}
}

func TestHomeAndLibraryRowsUseSameTypography(t *testing.T) {
	for _, height := range []int{240, 288} {
		p := screenPainter{width: 640, height: height, cache: &sceneCache{}}
		item := media.Item{Name: "Movies", Type: "Movie", RunTimeTicks: 67 * 60 * 10000000}
		title, _ := p.listRowText(item, 300, true, false)
		p.scene.Root = true
		count := 67
		item.LibraryCount = &count
		home, _ := p.listRowText(item, 300, true, false)
		if title != home {
			t.Fatal("home and library title use different font settings")
		}
	}
}
