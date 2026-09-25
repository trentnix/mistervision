package rendering

import (
	"bytes"
	"image"
	"image/color"
	"mistervision/internal/media"
	"mistervision/internal/ui"
	"os"
	"testing"
	"time"
)

func TestWideCarouselPreservesForeground(t *testing.T) {
	artwork := image.NewRGBA(image.Rect(0, 0, 160, 90))
	for y := 0; y < 90; y++ {
		for x := 0; x < 160; x++ {
			artwork.SetRGBA(x, y, color.RGBA{80, 120, 160, 255})
		}
	}
	scene := Scene{Root: true, Background: artwork, Now: time.Unix(100, 0), Content: Content{Page: media.Page{Items: []media.Item{{Name: "Movies"}, {Name: "TV Shows"}}}}}
	normal := NewRendererForRaster(480, 360).Render(640, 288, scene)
	renderer := NewRendererForRaster(480, 360)
	renderer.SetDisplayAspect(16.0 / 9)
	wide := renderer.Render(640, 288, scene)
	if !wide.FullScreen || wide.UIWidth != 640 || wide.UIHeight != 360 {
		t.Fatalf("wrong full-screen geometry: %dx%d", wide.UIWidth, wide.UIHeight)
	}
	for y := 0; y < 360; y++ {
		if !bytes.Equal(normal.UI[y*480*4:(y+1)*480*4], wide.UI[(y*640+80)*4:(y*640+560)*4]) {
			t.Fatalf("foreground moved or changed at row %d", y)
		}
	}
	for _, x := range []int{0, 639} {
		if wide.UI[(180*640+x)*4] == 0 {
			t.Fatal("background left a pillarbox")
		}
	}
	if dir := os.Getenv("RASTER_PREVIEW_DIR"); dir != "" {
		c := ui.New(wide.UIWidth, wide.UIHeight)
		copy(c.Pixels, wide.UI)
		writeRasterPreview(t, dir, "widescreen-carousel.png", c)
	}
	scene.ListMode = true
	list := renderer.Render(640, 288, scene)
	if !list.FullScreen || list.UIWidth != 640 || list.UIHeight != 360 {
		t.Fatal("list background did not fill the display")
	}
	scene.ListMode = false
	scene.Background = nil
	scene.Artwork.Covers = []image.Image{artwork}
	wide = renderer.Render(640, 288, scene)
	for _, x := range []int{0, 639} {
		if wide.UI[(180*640+x)*4] == 0 {
			t.Fatal("mosaic left a pillarbox")
		}
	}
}

func TestCarouselCRTUnchanged(t *testing.T) {
	for _, h := range []int{240, 288} {
		scene := Scene{Root: true, Now: time.Unix(100, 0)}
		plain := NewRendererForRaster(640, h).Render(640, h, scene)
		renderer := NewRendererForRaster(640, h)
		renderer.SetDisplayAspect(4.0 / 3)
		frame := renderer.Render(640, h, scene)
		if frame.FullScreen || !bytes.Equal(frame.UI, plain.UI) {
			t.Fatal("CRT frame changed")
		}
	}
}

func TestWideCustomBackgroundKeepsMatchingImageEdges(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 160, 90))
	for y := 0; y < 90; y++ {
		for x := 0; x < 160; x++ {
			col := color.RGBA{0, 180, 0, 255}
			if x < 10 || x >= 150 {
				col = color.RGBA{200, 0, 0, 255}
			}
			source.SetRGBA(x, y, col)
		}
	}
	renderer := NewRendererForRaster(480, 360)
	renderer.SetDisplayAspect(16.0 / 9)
	scene := Scene{Root: true, Background: source, Now: time.Unix(100, 0)}
	frame := renderer.Render(640, 288, scene)
	for _, x := range []int{0, 639} {
		at := (180*640 + x) * 4
		if frame.UI[at+2] == 0 || frame.UI[at+1] != 0 {
			t.Fatal("matching widescreen image lost its edge")
		}
	}
	// Changing screens and returning must rebuild the complete background.
	scene.ListMode = true
	renderer.Render(640, 288, scene)
	scene.ListMode = false
	frame = renderer.Render(640, 288, scene)
	if frame.UI[(180*640)*4+2] == 0 {
		t.Fatal("background did not survive a screen transition")
	}
}
