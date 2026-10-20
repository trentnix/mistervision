package caption

import (
	"bytes"
	"image/color"
	"testing"
)

func TestLabelWrapsAndPreservesCaptionCache(t *testing.T) {
	var r Renderer
	cue := r.Image("Caption remains unchanged", 640, 240)
	before := bytes.Clone(cue.Pix)
	ink := color.RGBA{R: 255, G: 224, B: 64, A: 255}
	short := r.Label("News", 215, 18, 2, 0.5, ink)
	long := r.Label("A long program title that wraps across multiple lines", 215, 18, 2, 0.5, ink)
	if short == nil || long == nil || short.Bounds().Dy() >= long.Bounds().Dy() || long.Bounds().Dx() > 215 {
		t.Fatal("label layout did not honor wrapping and width")
	}
	if !bytes.Equal(before, r.Image("Caption remains unchanged", 640, 240).Pix) {
		t.Fatal("interface label changed the cached caption")
	}
	for _, text := range []string{"日本語", "مرحبا بالعالم", "†††"} {
		if im := r.Label(text, 160, 18, 1, 0.5, ink); im == nil || im.Bounds().Dx() > 160 {
			t.Fatalf("invalid Unicode label for %q", text)
		}
	}
	if r.Label("", 160, 18, 1, 0.5, ink) != nil {
		t.Fatal("empty label rendered")
	}
}

func TestCenteredLabelKeepsShortLineCenteredAtEachDensity(t *testing.T) {
	var r Renderer
	for _, density := range []float32{1, 1.5, 2} {
		im := r.CenteredLabelDensity("A wide heading\nI", 240, 24, 2, 1, color.RGBA{255, 255, 255, 255}, density, density)
		left, right := im.Bounds().Dx(), -1
		for y := im.Bounds().Dy() / 2; y < im.Bounds().Dy(); y++ {
			for x := 0; x < im.Bounds().Dx(); x++ {
				if im.RGBAAt(x, y).A > 32 {
					left = min(left, x)
					right = max(right, x)
				}
			}
		}
		if right < left {
			t.Fatal("missing second line")
		}
		delta := (left + right) - im.Bounds().Dx()
		if delta < -int(4*density) || delta > int(4*density) {
			t.Fatalf("density %g: short line is off center by %d pixels", density, delta/2)
		}
	}
}
