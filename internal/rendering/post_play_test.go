package rendering

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	"os"
	"testing"
	"time"

	"mistervision/internal/media"
	"mistervision/internal/ui"
)

func TestPostPlayPromptUsesBrowsingOutput(t *testing.T) {
	item := media.Item{ID: "episode", Name: "The Night of No Tomorrow", Type: "Episode", SeriesName: "Dungeons & Dragons", RunTimeTicks: 22 * 600000000}
	art := image.NewRGBA(image.Rect(0, 0, 160, 90))
	for y := range 90 {
		for x := range 160 {
			art.SetRGBA(x, y, color.RGBA{R: uint8(60 + x), G: uint8(80 + y), B: 120, A: 255})
		}
	}
	var backdrop image.Image = art
	if path := os.Getenv("POST_PLAY_BACKDROP"); path != "" {
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		backdrop, _, err = image.Decode(file)
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, height := range []int{240, 288, 360, 480} {
		scene := Scene{Now: time.Unix(100, 0), Content: Content{Detail: &item}, Artwork: Artwork{Backdrop: backdrop}, PostPlay: &PostPlayPresentation{
			Title: "Up next", Next: &media.Item{Name: "The Eye of the Beholder", SeriesName: "Dungeons & Dragons", ParentIndexNumber: new(1), IndexNumber: new(2)},
			Rows: []string{"Play next episode", "Back to episodes"},
		}}
		r := NewRenderer()
		if height == 360 {
			r = NewRendererForRaster(960, 720)
			r.SetDisplayAspect(16.0 / 9)
		}
		logicalHeight := height
		if height == 360 {
			logicalHeight = 240
		}
		frame := r.Render(640, logicalHeight, scene)
		if frame.Video || len(frame.Overlay) != 0 {
			t.Fatal("completion prompt retained the video output")
		}
		first := bytes.Clone(frame.UI)
		scene.PostPlay.Selected = 1
		if bytes.Equal(first, r.Render(640, logicalHeight, scene).UI) {
			t.Fatal("selection did not move")
		}
		if dir := os.Getenv("POST_PLAY_PREVIEW_DIR"); dir != "" {
			c := ui.New(frame.UIWidth, frame.UIHeight)
			copy(c.Pixels, first)
			name := fmt.Sprintf("next-episode-%d.png", height)
			if height == 360 {
				name = "next-episode-widescreen.png"
			}
			if height == 240 || height == 288 {
				writeSetupPreview(t, dir, name, c)
			} else {
				writeRasterPreview(t, dir, name, c)
			}
		}
	}
}

func TestPostPlayArtworkFillsWideOutputAndClearsWhenMissing(t *testing.T) {
	art := image.NewRGBA(image.Rect(0, 0, 160, 90))
	for y := range 90 {
		for x := range 160 {
			art.SetRGBA(x, y, color.RGBA{R: 160, G: 100, B: 60, A: 255})
		}
	}
	s := Scene{Content: Content{Detail: &media.Item{Type: "Episode"}}, Artwork: Artwork{Backdrop: art}, PostPlay: &PostPlayPresentation{
		Title: "Up next", Next: &media.Item{Name: "A very long episode title that wraps onto another line without moving or covering the actions", SeriesName: "Example series", ParentIndexNumber: new(1), IndexNumber: new(2)}, Rows: []string{"Play next episode", "Back to episodes"},
	}}
	r := NewRendererForRaster(960, 720)
	r.SetDisplayAspect(16.0 / 9)
	frame := r.Render(640, 240, s)
	if !frame.FullScreen || frame.UIWidth != 1280 || frame.UIHeight != 720 {
		t.Fatal("incorrect wide prompt geometry")
	}
	at := (50 * frame.UIWidth) * 4
	if frame.UI[at+2] == 0 {
		t.Fatal("artwork did not fill widescreen edge")
	}
	s.Artwork = Artwork{}
	if r.Render(640, 240, s).UI[at+2] != 0 {
		t.Fatal("missing artwork kept the previous image")
	}
}

// Both output layouts must refresh cached backgrounds when a thumbnail arrives
// after the backdrop, and restore the backdrop when that thumbnail is absent.
func TestPostPlayPrefersPreviewImageWithBackdropFallback(t *testing.T) {
	for _, wide := range []bool{false, true} {
		t.Run(fmt.Sprintf("wide=%t", wide), func(t *testing.T) {
			r := NewRenderer()
			if wide {
				r = NewRendererForRaster(960, 720)
				r.SetDisplayAspect(16.0 / 9)
			}
			backdrop := image.NewRGBA(image.Rect(0, 0, 160, 90))
			preview := image.NewRGBA(image.Rect(0, 0, 160, 90))
			for y := range 90 {
				for x := range 160 {
					backdrop.SetRGBA(x, y, color.RGBA{R: 200, A: 255})
					preview.SetRGBA(x, y, color.RGBA{B: 200, A: 255})
				}
			}
			item := media.Item{Type: "Episode", Name: "Next episode"}
			s := Scene{Content: Content{Detail: &item}, Artwork: Artwork{Backdrop: backdrop}, PostPlay: &PostPlayPresentation{Next: &item}}
			fallback := bytes.Clone(r.Render(640, 240, s).UI)
			s.Artwork.Primary = preview
			withPreview := bytes.Clone(r.Render(640, 240, s).UI)
			if bytes.Equal(fallback, withPreview) {
				t.Fatal("episode preview did not replace the backdrop")
			}
			if s.Artwork.Backdrop != backdrop {
				t.Fatal("rendering changed the caller's backdrop")
			}
			s.Artwork.Primary = nil
			if !bytes.Equal(fallback, r.Render(640, 240, s).UI) {
				t.Fatal("missing preview did not restore the backdrop")
			}
			s.PostPlay = nil
			ordinary := bytes.Clone(r.Render(640, 240, s).UI)
			s.Artwork.Primary = preview
			if !bytes.Equal(ordinary, r.Render(640, 240, s).UI) {
				t.Fatal("episode preview changed the ordinary detail background")
			}
		})
	}
}
