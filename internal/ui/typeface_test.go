package ui

import (
	"image"
	"image/color"
	"testing"
)

type testTypeface struct{ calls, width, scale int }

func (f *testTypeface) Measure(_ string, scale int) int { return 7 * scale }
func (f *testTypeface) Rasterize(_ string, width, scale int, _ uint32) (*image.NRGBA, int) {
	f.calls++
	f.width, f.scale = width, scale
	im := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	im.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 128})
	return im, -1
}

func TestTypefaceUsesClippingMeasurementAndOverlayAlpha(t *testing.T) {
	face := &testTypeface{}
	c := NewOverlay(10, 10)
	c.Typeface = face
	c.TextScaled(2, 3, "text", 0xffffff, 8, 2)
	if face.width != 6 || face.scale != 2 || c.MeasureText("text", 2) != 14 {
		t.Fatal("canvas did not delegate text layout correctly")
	}
	i := (2*10 + 2) * 4
	if c.Pixels[i+2] != 255 || c.Pixels[i+3] != 128 {
		t.Fatal("typeface text lost its offset or straight alpha")
	}
	c.BitmapText(0, 0, "A", 0xffffff, 10)
	if face.calls != 1 {
		t.Fatal("bitmap navigation text used the configured typeface")
	}
}
