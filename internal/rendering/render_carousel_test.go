package rendering

import (
	"strings"
	"testing"
	"unicode/utf8"

	"mistervision/internal/ui"
)

func TestCarouselNameLimit(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"Anime Movies", "Anime Movies"},
		{"  Science\n Fiction\t & Fantasy  ", "Science Fiction & Fantasy"},
		{"TV\x00 Shows", "TV Shows"},
		{strings.Repeat("界", 64), strings.Repeat("界", 64)},
		{strings.Repeat("界", 10000), strings.Repeat("界", 63) + "…"},
		{"\n\t", ""},
	} {
		got := carouselName(tc.input)
		if got != tc.want {
			t.Fatalf("bounded name = %q, want %q", got, tc.want)
		}
		if !utf8.ValidString(got) || utf8.RuneCountInString(got) > 64 {
			t.Fatal("invalid or unbounded label")
		}
	}
}

func TestCarouselLabelsWrapAndBoundLongNames(t *testing.T) {
	for _, height := range []int{240, 288} {
		p := screenPainter{canvas: ui.New(640, height), cache: &sceneCache{}, width: 640, height: height}
		short := p.centeredHeadingText("Movies", carouselNameWidth, 34, 2, titleColor)
		wrapped := p.centeredHeadingText("Science Fiction & Fantasy", carouselNameWidth, 34, 2, titleColor)
		long := p.centeredHeadingText(carouselName(strings.Repeat("An extremely long library name ", 100)), carouselNameWidth, 34, 2, titleColor)
		if wrapped.Bounds().Dy() <= short.Bounds().Dy() {
			t.Fatal("long title did not wrap")
		}
		if long.Bounds().Dy() > wrapped.Bounds().Dy()+1 {
			t.Fatal("long title exceeded two lines")
		}
		for _, im := range []*ui.RasterImage{short, wrapped, long} {
			if im.Bounds().Dx() > carouselNameWidth {
				t.Fatal("title exceeded slot width")
			}
		}
	}
}

// Wide glyphs must wrap by measured width rather than a character-count estimate.
func TestCarouselWideGlyphPressure(t *testing.T) {
	for _, size := range []struct{ height, rasterW, rasterH int }{{240, 640, 240}, {288, 640, 288}, {240, 960, 720}} {
		p := screenPainter{canvas: ui.NewRaster(640, size.height, size.rasterW, size.rasterH), cache: &sceneCache{}, width: 640, height: size.height}
		label := func(name string) *ui.RasterImage {
			return p.centeredHeadingText(carouselName(name), carouselNameWidth, 34, 2, titleColor)
		}
		wide, narrow := label(strings.Repeat("W", 16)), label(strings.Repeat("i", 16))
		if wide.Bounds().Dy() <= narrow.Bounds().Dy() {
			t.Fatal("wide letters did not wrap independently of character count")
		}
		for _, name := range []string{
			strings.Repeat("W", 12), strings.Repeat("W", 16), strings.Repeat("WM", 10000),
			"Worldwide War Movies and Military Documentaries",
			"世界の映画とテレビ番組と音楽のコレクション",
		} {
			im := label(name)
			if im.Bounds().Dx() > carouselNameWidth || im.Bounds().Dy() > size.height/3 {
				t.Fatalf("%q exceeds carousel title area: %v", carouselName(name), im.Bounds())
			}
			if im.Source.Bounds().Dx() > carouselNameWidth*size.rasterW/640 {
				t.Fatalf("%q exceeds high-density title width: %v", carouselName(name), im.Source.Bounds())
			}
		}
	}
}
