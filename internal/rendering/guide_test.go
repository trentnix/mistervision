package rendering

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"os"
	"testing"
	"time"

	"mistervision/internal/media"
	"mistervision/internal/ui"
)

func TestChannelGuideRendersWithoutMovingControls(t *testing.T) {
	now := time.Date(2026, 9, 20, 18, 0, 0, 0, time.Local)
	scene := Scene{ListMode: true, Now: now, Content: Content{Title: "Live TV", Page: media.Page{Items: []media.Item{
		{ID: "one", Name: "ABC", Number: "2.1", Type: "TvChannel"},
		{ID: "two", Name: "PBS", Number: "8.1", Type: "TvChannel"},
	}}}}
	logo := image.NewRGBA(image.Rect(0, 0, 96, 48))
	draw.Draw(logo, logo.Bounds(), image.NewUniform(color.RGBA{R: 240, G: 190, B: 20, A: 255}), image.Point{}, draw.Src)
	programs := []media.Program{
		{ChannelID: "one", Title: "ABC Weekend Special: The Adventures Continue", Start: now.Add(-15 * time.Minute), End: now.Add(15 * time.Minute)},
		{ChannelID: "one", Title: "Evening News", Start: now.Add(15 * time.Minute), End: now.Add(45 * time.Minute)},
	}
	for _, height := range []int{240, 288} {
		renderer := NewRenderer()
		scene.Artwork.Primary = nil
		empty := append([]byte(nil), renderer.Render(640, height, scene).UI...)
		scene.Artwork.Primary = logo
		if !bytes.Equal(empty, renderer.Render(640, height, scene).UI) {
			t.Fatal("channel without guide listings did not leave the panel blank")
		}
		scene.Guide = map[string][]media.Program{"one": programs}
		filled := append([]byte(nil), renderer.Render(640, height, scene).UI...)
		scene.Artwork.Primary = nil
		if bytes.Equal(filled, renderer.Render(640, height, scene).UI) {
			t.Fatal("available channel logo did not appear")
		}
		if bytes.Equal(empty, filled) {
			t.Fatal("guide did not change presentation")
		}
		if !bytes.Equal(empty[(height-35)*640*4:], filled[(height-35)*640*4:]) {
			t.Fatal("guide moved or covered controls")
		}
		if dir := os.Getenv("GUIDE_PREVIEW_DIR"); dir != "" && height == 240 {
			canvas := ui.New(640, height)
			copy(canvas.Pixels, filled)
			writeSetupPreview(t, dir, "live-tv-guide.png", canvas)
		}
		scene.Guide = nil
	}
}

func TestChannelGuideIgnoresUnscheduledCatalogTitle(t *testing.T) {
	item := media.Item{ID: "one", Name: "ABC", Type: "TvChannel"}
	scene := Scene{ListMode: true, Now: time.Now(), Content: Content{Page: media.Page{Items: []media.Item{item}}}}
	renderer := NewRenderer()
	want := append([]byte(nil), renderer.Render(640, 240, scene).UI...)
	scene.Content.Page.Items[0].CurrentProgram.Name = "Expired catalog program"
	got := renderer.Render(640, 240, scene).UI
	if !bytes.Equal(want, got) {
		t.Fatal("channel row displayed catalog text without a valid schedule")
	}
}
