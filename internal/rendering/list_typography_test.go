package rendering

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"mistervision/internal/media"
	"mistervision/internal/ui"
)

func TestListSelectionContainsVisibleText(t *testing.T) {
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
				for y := max(0, first-2); y < min(height, last+5); y++ {
					for x := 24; x < 200; x++ {
						off := (y*640 + x) * 4
						if pixels[off] > 50 && pixels[off+1] > 50 && pixels[off+2] > 50 {
							top = min(top, y)
							bottom = max(bottom, y)
						}
					}
				}
				// A stable baseline reserves descender space even when the text
				// does not use it. Visible-ink padding need not be identical.
				if bottom < top || top < first || bottom > last {
					t.Fatalf("selection padding top=%d bottom=%d", top-first, last-bottom)
				}
			})
		}
	}
}

func TestHomeAndLibraryRowsUseSameTypography(t *testing.T) {
	for _, height := range []int{240, 288} {
		p := screenPainter{canvas: ui.New(640, height), width: 640, height: height, cache: &sceneCache{}}
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

func TestListRowSpacingDoesNotDependOnDescenders(t *testing.T) {
	for _, geometry := range []struct{ height, rasterWidth, rasterHeight int }{
		{240, 640, 240}, {288, 640, 288}, {480, 640, 480}, {240, 960, 720},
	} {
		t.Run(fmt.Sprintf("%d/%dx%d", geometry.height, geometry.rasterWidth, geometry.rasterHeight), func(t *testing.T) {
			var wantTop, wantBottom, wantBar int
			for i, title := range []string{"Countdown", "Space Fold", "Boobytrap", "The Long Wait", "Blitzkrieg"} {
				item := media.Item{Name: title, Type: "Episode", RunTimeTicks: 24 * 600000000}
				item.UserData.Played = true // Green metadata separates it from the title.
				scene := Scene{ListMode: true, Content: Content{Page: media.Page{Items: []media.Item{item}}}}
				r := NewRendererForRaster(geometry.rasterWidth, geometry.rasterHeight)
				frame := r.Render(640, geometry.height, scene)
				top, bottom, bar := frame.UIHeight, -1, -1
				for y := 0; y < frame.UIHeight; y++ {
					for x := 0; x < frame.UIWidth; x++ {
						at := (y*frame.UIWidth + x) * 4
						b, g, red := int(frame.UI[at]), int(frame.UI[at+1]), int(frame.UI[at+2])
						if g > red+20 && g > b+20 {
							top, bottom = min(top, y), max(bottom, y)
						}
						if b == 0x7c && g == 0x37 && red == 0x0d {
							bar = max(bar, y)
						}
					}
				}
				if bottom < top || bar < 0 {
					t.Fatal("missing metadata or selection bar")
				}
				if i == 0 {
					wantTop, wantBottom, wantBar = top, bottom, bar
				}
				if top != wantTop || bottom != wantBottom || bar != wantBar {
					t.Fatalf("%q moved metadata or selection: %d,%d,%d want %d,%d,%d", title, top, bottom, bar, wantTop, wantBottom, wantBar)
				}
			}
		})
	}
}
