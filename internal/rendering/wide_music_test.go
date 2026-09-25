package rendering

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"math"
	"path/filepath"
	"testing"
	"time"

	"mistervision/internal/media"
	"mistervision/internal/musicviz"
	"mistervision/internal/ui"
)

func TestWideMusicEffectsAndGeometry(t *testing.T) {
	library, err := musicviz.Load(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatal(err)
	}
	cover := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := range 64 {
		for x := range 64 {
			cover.SetRGBA(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	for _, size := range []struct {
		w, h, full int
		aspect     float64
	}{{480, 360, 640, 16.0 / 9}, {720, 540, 960, 16.0 / 9}, {960, 720, 1280, 16.0 / 9}, {480, 360, 576, 16.0 / 10}} {
		t.Run(fmt.Sprintf("%dx%d", size.full, size.h), func(t *testing.T) {
			r := NewRendererForRaster(size.w, size.h)
			r.SetDisplayAspect(size.aspect)
			s := Scene{Audio: true, Music: library, Artwork: Artwork{Primary: cover}, Content: Content{Detail: &media.Item{Type: "Audio", Name: "Track", Artists: []string{"Artist"}, Album: "Album"}}}
			for index, preset := range library.Config.Backgrounds {
				s.MusicIndex = index
				s.Now = time.Unix(100, 0)
				s.MusicFrame = musicviz.Frame{Now: s.Now, Artwork: cover, Levels: [2]float64{.5, .8}}
				f := r.Render(640, 288, s)
				if !f.FullScreen || f.UIWidth != size.full || f.UIHeight != size.h {
					t.Fatalf("%s: wrong frame geometry", preset.Name)
				}
				for range 3 {
					s.Now = s.Now.Add(50 * time.Millisecond)
					s.MusicFrame.Now = s.Now
					f = r.Render(640, 288, s)
				}
				if preset.Type == "nebula" {
					for _, x := range []int{0, size.full - 1} {
						at := (size.h/3*size.full + x) * 4
						if bytes.Equal(f.UI[at:at+3], []byte{0, 0, 0}) {
							t.Fatal("animation left a pillarbox")
						}
					}
				}
				if preset.Type == "spinning" {
					normal := NewRendererForRaster(size.w, size.h).Render(640, 288, s)
					want := redArtworkBounds(normal.UI, size.w, size.h).Add(image.Pt((size.full-size.w)/2, 0))
					got := redArtworkBounds(f.UI, size.full, size.h)
					// One logical effect pixel can span several output pixels after
					// the proportional widescreen crop. Bound rounding at that density.
					tolerance := int(math.Ceil(float64(size.h) / 288 * float64(size.full) / float64(size.w)))
					for _, delta := range []int{got.Min.X - want.Min.X, got.Min.Y - want.Min.Y, got.Max.X - want.Max.X, got.Max.Y - want.Max.Y} {
						if delta < -tolerance || delta > tolerance {
							t.Fatalf("rotating artwork moved or stretched: got %v, want %v", got, want)
						}
					}
				}
				if preset.Type == "none" && f.UI[(size.h/3*size.full)*4] != 0 {
					t.Fatal("previous effect survived switching to Off")
				}
			}
			// The effect is calculated at logical size, regardless of HDMI raster size.
			if r.wide.music.Width != 640 || r.wide.music.Height != 288 {
				t.Fatal("effect work grew with output resolution")
			}
			s.MusicIndex = 0
			r.SetDisplayAspect(4.0 / 3)
			f := r.Render(640, 288, s)
			if f.FullScreen || f.UIWidth != size.w || f.UIHeight != size.h {
				t.Fatal("4:3 geometry changed")
			}
		})
	}
}

func redArtworkBounds(pixels []byte, w, h int) image.Rectangle {
	bounds := image.Rectangle{}
	for y := range h {
		for x := range w {
			at := (y*w + x) * 4
			if pixels[at+2] > 200 && pixels[at+1] < 10 && pixels[at] < 10 {
				bounds = bounds.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return bounds
}

func TestWideMusicOffPreservesForeground(t *testing.T) {
	library := &musicviz.Library{Config: musicviz.Config{Backgrounds: []musicviz.Preset{{Name: "Off", Type: "none"}}}}
	s := Scene{Audio: true, Music: library, Now: time.Unix(100, 0), Content: Content{Detail: &media.Item{Type: "Audio", Name: "Track", Artists: []string{"Artist"}}}}
	normal := NewRendererForRaster(480, 360).Render(640, 288, s)
	r := NewRendererForRaster(480, 360)
	r.SetDisplayAspect(16.0 / 9)
	wide := r.Render(640, 288, s)
	for y := range 360 {
		if !bytes.Equal(normal.UI[y*480*4:(y+1)*480*4], wide.UI[(y*640+80)*4:(y*640+560)*4]) {
			t.Fatalf("music foreground changed at row %d", y)
		}
	}
}

func TestMusicArtworkBackground(t *testing.T) {
	library, err := musicviz.Load(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatal(err)
	}
	backdrop := image.NewRGBA(image.Rect(0, 0, 16, 9))
	for y := 0; y < 9; y++ {
		for x := 0; x < 16; x++ {
			backdrop.SetRGBA(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	for _, aspect := range []float64{4.0 / 3, 16.0 / 9} {
		r := NewRendererForRaster(480, 360)
		r.SetDisplayAspect(aspect)
		s := Scene{Audio: true, Music: library, MusicIndex: musicviz.ArtworkBackground, MusicLabel: true, Artwork: Artwork{Backdrop: backdrop}, Content: Content{Detail: &media.Item{Type: "Audio", Name: "Track"}}}
		f := r.Render(640, 288, s)
		for _, x := range []int{0, f.UIWidth - 1} {
			at := (120*f.UIWidth + x) * 4
			if f.UI[at+2] == 0 {
				t.Fatal("missing music artwork at edge")
			}
		}
		s.Artwork.Backdrop = nil
		f = r.Render(640, 288, s)
		if f.UI[(120*f.UIWidth)*4+2] != 0 {
			t.Fatal("stale music artwork")
		}
	}
}

func TestMusicHeaderOnlyAppearsWithBackgroundFeedback(t *testing.T) {
	library, err := musicviz.Load(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatal(err)
	}
	backdrop := image.NewRGBA(image.Rect(0, 0, 16, 9))
	for i := range backdrop.Pix {
		backdrop.Pix[i] = 255
	}
	for _, aspect := range []float64{4.0 / 3, 16.0 / 9} {
		r := NewRendererForRaster(480, 360)
		r.SetDisplayAspect(aspect)
		s := Scene{Audio: true, Music: library, MusicIndex: musicviz.ArtworkBackground,
			Artwork: Artwork{Backdrop: backdrop}, Content: Content{Detail: &media.Item{Type: "Audio", Name: "Song"}}}
		frame := r.Render(640, 288, s)
		baseline := bytes.Clone(frame.UI)
		// The top is plain artwork: no header, clock, or dark stripe.
		for y := 0; y < 20; y++ {
			for x := 0; x < frame.UIWidth; x++ {
				at := (y*frame.UIWidth + x) * 4
				if !bytes.Equal(frame.UI[at:at+3], frame.UI[:3]) {
					t.Fatal("persistent music header")
				}
			}
		}
		s.MusicLabel = true
		label := r.Render(640, 288, s)
		if bytes.Equal(label.UI, baseline) {
			t.Fatal("background feedback not visible")
		}
		s.MusicLabel = false
		after := r.Render(640, 288, s)
		if !bytes.Equal(after.UI, baseline) {
			t.Fatal("background strip did not disappear")
		}
	}
}

func TestMusicBackdropLabel(t *testing.T) {
	for _, tc := range []struct {
		artists     []string
		album, want string
	}{
		{[]string{"Boards of Canada"}, "Music Has the Right to Children", "Boards of Canada"},
		{[]string{"First", "Second"}, "Album", "First, Second"},
		{nil, "Album", "Album"}, {nil, "", "Background"},
	} {
		if got := musicBackdropLabel(tc.artists, tc.album); got != tc.want {
			t.Fatalf("got %q, want %q", got, tc.want)
		}
	}
}

func TestMusicInfoFitsBothCRTLayouts(t *testing.T) {
	for _, height := range []int{240, 288} {
		p := screenPainter{canvas: ui.New(640, height), cache: &sceneCache{}, width: 640, height: height}
		p.scene.Content.Detail = &media.Item{Name: "Roygbiv", Artists: []string{"Boards of Canada"}, Album: "Music Has the Right to Children", RunTimeTicks: 1800000000}
		lines := p.musicInfo()
		if len(lines) != 4 {
			t.Fatalf("expected separate title, artist, album, timing; got %d", len(lines))
		}
		total := 0
		for _, line := range lines {
			total += line.Bounds().Dy() + musicInfoGap
		}
		if total > height*2/5 {
			t.Fatalf("metadata crowds artwork: %d of %d", total, height)
		}
		if lines[0].Bounds().Dy() <= lines[1].Bounds().Dy() {
			t.Fatal("track title is not larger than artist")
		}
		p.scene.Content.Detail.Artists = nil
		p.scene.Content.Detail.Album = ""
		if got := len(p.musicInfo()); got != 2 {
			t.Fatalf("empty metadata left blank rows: %d", got)
		}
	}
}
