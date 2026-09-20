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
