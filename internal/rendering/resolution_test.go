package rendering

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mistervision/internal/media"
	"mistervision/internal/ui"
)

func resolutionScene() Scene {
	s := Scene{Root: true, Now: time.Unix(0, 0)}
	count := 67
	s.Content.Page.Items = []media.Item{{Name: "Movies", CollectionType: "movies", LibraryCount: &count}, {Name: "Television", CollectionType: "tvshows", LibraryCount: &count}, {Name: "Music", CollectionType: "music", LibraryCount: &count}}
	return s
}

func TestBrowsingRasterPreservesLayoutAndPlayback(t *testing.T) {
	for _, size := range [][2]int{{640, 240}, {640, 288}, {720, 540}, {960, 720}} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			r := NewRendererForRaster(size[0], size[1])
			s := resolutionScene()
			for _, list := range []bool{false, true} {
				s.ListMode = list
				f := r.Render(640, 288, s)
				if len(f.UI) != size[0]*size[1]*4 || f.UIWidth != size[0] || f.UIHeight != size[1] {
					t.Fatal("incorrect browsing raster")
				}
				if r.canvas.Width != 640 || r.canvas.Height != 288 {
					t.Fatal("raster size changed layout")
				}
				if dir := os.Getenv("RASTER_PREVIEW_DIR"); dir != "" {
					c := ui.New(size[0], size[1])
					copy(c.Pixels, f.UI)
					writeRasterPreview(t, dir, fmt.Sprintf("%dx%d-list-%t.png", size[0], size[1], list), c)
				}
			}
			s.Video = true
			got := r.Render(640, 288, s)
			want := NewRenderer().Render(640, 288, s)
			if !bytes.Equal(got.UI, want.UI) || !bytes.Equal(got.Overlay, want.Overlay) {
				t.Fatal("browsing density changed playback rendering")
			}
			s.Video = false
			if len(r.Render(640, 288, s).UI) != size[0]*size[1]*4 {
				t.Fatal("browsing raster not restored")
			}
		})
	}
}

func TestDenseTextRetainsMetricsAndAddsOutlineDetail(t *testing.T) {
	var cache primaryTextCache
	key := primaryTextKey{text: "MiSTerVision", width: 200, size: 28, lines: 1, color: 0xffffff, scaleY: .6, bold: true}
	low := cache.denseImage(key, 1, 1)
	high := cache.denseImage(key, 1.5, 2)
	if low.Bounds() != high.Bounds() {
		t.Fatal("density changed layout metrics")
	}
	w, h := high.Source.Bounds().Dx(), high.Source.Bounds().Dy()
	enlarged, sharp := ui.New(w, h), ui.New(w, h)
	enlarged.Blit(low.RGBA, 0, 0, w, h)
	sharp.Blit(high.Source, 0, 0, w, h)
	if bytes.Equal(enlarged.Pixels, sharp.Pixels) {
		t.Fatal("text was enlarged instead of rasterized at higher density")
	}
	if cache.denseImage(key, 1.5, 2) != high {
		t.Fatal("dense text cache not reused")
	}
}

func BenchmarkBrowsingRaster(b *testing.B) {
	for _, size := range [][2]int{{640, 288}, {720, 540}, {960, 720}} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			r := NewRendererForRaster(size[0], size[1])
			s := resolutionScene()
			cover := image.NewRGBA(image.Rect(0, 0, 200, 300))
			for y := 0; y < 300; y++ {
				for x := 0; x < 200; x++ {
					cover.SetRGBA(x, y, color.RGBA{R: byte(x), G: byte(y), B: 80, A: 255})
				}
			}
			s.Artwork.Covers = []image.Image{cover, cover, cover, cover, cover, cover}
			r.Render(640, 288, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				s.Now = s.Now.Add(time.Second / 60)
				r.Render(640, 288, s)
			}
		})
	}
}

func writeRasterPreview(t *testing.T, dir, name string, c *ui.Canvas) {
	t.Helper()
	im := image.NewRGBA(image.Rect(0, 0, c.Width, c.Height))
	for i := 0; i < len(c.Pixels); i += 4 {
		im.Pix[i], im.Pix[i+1], im.Pix[i+2], im.Pix[i+3] = c.Pixels[i+2], c.Pixels[i+1], c.Pixels[i], 255
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, im); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}
