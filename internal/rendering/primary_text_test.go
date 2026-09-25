package rendering

import (
	"bytes"
	"fmt"
	"os"
	"testing"
	"time"

	"mistervision/internal/connection"
	"mistervision/internal/input/control"
	"mistervision/internal/media"
	"mistervision/internal/ui"
)

func TestPrimaryTextCacheReusesAndBoundsImages(t *testing.T) {
	var cache primaryTextCache
	key := primaryTextKey{text: "News", width: 160, size: 18, lines: 1, color: 0xffffff, scaleY: 0.5}
	first := cache.image(key)
	if first == nil || cache.image(key) != first {
		t.Fatal("repeated label was reshaped")
	}
	before := bytes.Clone(first.Pix)
	key.bold = true
	bold := cache.image(key)
	if bold == nil || bold.Bounds() != first.Bounds() || bytes.Equal(bold.Pix, first.Pix) {
		t.Fatal("heading weight did not change coverage within the same layout")
	}
	if !bytes.Equal(before, first.Pix) {
		t.Fatal("heading weight mutated the regular cached label")
	}
	for i := range 140 {
		key.text = fmt.Sprint(i)
		cache.image(key)
	}
	if len(cache.images) != 128 || len(cache.keys) != 128 {
		t.Fatal("label cache exceeded its bound")
	}
}

func TestPrimaryFontPreviews(t *testing.T) {
	dir := os.Getenv("PRIMARY_FONT_PREVIEW_DIR")
	if dir == "" {
		t.Skip("set PRIMARY_FONT_PREVIEW_DIR to export font previews")
	}
	scene := Scene{Root: true, Now: time.Date(2026, 9, 20, 18, 0, 0, 0, time.UTC)}
	scene.Content.Page.Items = []media.Item{
		{ID: "movies", Name: "Movies", CollectionType: "movies"},
		{ID: "tv", Name: "Television", CollectionType: "tvshows"},
		{ID: "music", Name: "Music", CollectionType: "music"},
	}
	count := 67
	for i := range scene.Content.Page.Items {
		scene.Content.Page.Items[i].LibraryCount = &count
	}
	scene.About.Release.Available = true
	renderer := NewRenderer()
	for _, list := range []bool{false, true} {
		scene.ListMode = list
		name := "carousel"
		if list {
			name = "list"
		}
		frame := renderer.Render(640, 240, scene)
		canvas := ui.New(640, 240)
		copy(canvas.Pixels, frame.UI)
		writeSetupPreview(t, dir, name+".png", canvas)
	}
	scene.Root = false
	scene.Content.Title = "Movies"
	for i := range scene.Content.Page.Items {
		scene.Content.Page.Items[i].Type = "Movie"
		scene.Content.Page.Items[i].RunTimeTicks = 67 * 60 * 10000000
	}
	frame := renderer.Render(640, 240, scene)
	canvas := ui.New(640, 240)
	copy(canvas.Pixels, frame.UI)
	writeSetupPreview(t, dir, "library-list.png", canvas)
}

func TestListingTitleOmitsFolderPrefix(t *testing.T) {
	for _, kind := range []string{"MusicArtist", "MusicAlbum", "Series", "Season", "Folder"} {
		for _, folder := range []bool{false, true} {
			if got := itemTitle(media.Item{Name: "Title", Type: kind, IsFolder: folder}); got != "Title" {
				t.Fatalf("%s title = %q", kind, got)
			}
		}
	}
}

func TestRootHeadingDoesNotMoveBetweenListAndCarousel(t *testing.T) {
	for _, height := range []int{240, 288} {
		renderer := NewRenderer()
		scene := Scene{Root: true, Now: time.Unix(0, 0)}
		scene.About.Release.Available = true
		scene.About.Profile = &connection.Profile{Name: "Alex"}
		scene.Content.Page.Items = []media.Item{{Name: "Movies", CollectionType: "movies"}}
		carousel := bytes.Clone(renderer.Render(640, height, scene).UI)
		scene.ListMode = true
		list := renderer.Render(640, height, scene).UI
		top, bottom := safeY(640, height), safeY(640, height)+44
		if !bytes.Equal(carousel[top*640*4:bottom*640*4], list[top*640*4:bottom*640*4]) {
			t.Fatal("heading or clock moved between carousel and list")
		}
	}
}

func TestNavigationHintsKeepBitmapFont(t *testing.T) {
	plain, styled := ui.New(640, 240), ui.New(640, 240)
	cache := &sceneCache{}
	styled.Typeface = cache.typeface(640, 240)
	rows := controlRows(640, []controlHint{hint(control.KeyboardLabels(), control.Open, "Select"), hint(control.KeyboardLabels(), control.Back, "Back")})
	drawControls(plain, 210, rows)
	drawControls(styled, 210, rows)
	if !bytes.Equal(plain.Pixels, styled.Pixels) {
		t.Fatal("new typeface changed navigation hint pixels")
	}
}

func TestListTextRetainsDenseInk(t *testing.T) {
	for _, size := range []struct{ w, h int }{{480, 360}, {640, 480}, {960, 720}, {640, 240}} {
		c := ui.NewRaster(640, 288, size.w, size.h)
		p := screenPainter{canvas: c, cache: &sceneCache{}, width: 640, height: 288}
		for _, text := range []string{"Boards of Canada", "Music", "TEST", "gyp", "Aphex Twin"} {
			im := p.listText(text, 400, 18, 0xffffff, true)
			key := primaryTextKey{text: text, width: 400, size: 18, lines: 1, color: 0xffffff, scaleY: .6, bold: true}
			sx, sy := c.Density()
			full := p.cache.text.denseImage(key, sx, sy)
			ink := textInk(full.Source)
			if !ink.In(im.Source.Bounds()) {
				t.Errorf("%dx%d %q: ink %v clipped to %v", size.w, size.h, text, ink, im.Source.Bounds())
			}
		}
	}
}
