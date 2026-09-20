package rendering

import (
	"image"

	"mistervision/internal/ui"
)

// browsingTypeface adapts cached Noto labels to the canvas text boundary.
// Its geometry stays fixed even when a canvas borrows a clipped row region.
type browsingTypeface struct {
	cache      *primaryTextCache
	scaleY     float32
	bodyHeight int // Logical body font height. Zero uses the standard seven pixels.
}

func (f *browsingTypeface) key(text string, width, scale int, color uint32) primaryTextKey {
	// Match the existing logical line spacing at each output geometry.
	height := 7 * scale
	if scale == 1 && f.bodyHeight > 0 {
		height = f.bodyHeight
	}
	size := max(8, int(float32(height)/f.scaleY+0.5))
	return primaryTextKey{text: text, width: width, size: size, lines: 1,
		color: color, scaleY: f.scaleY, bold: scale > 1}
}

func (f *browsingTypeface) Measure(text string, scale int) int {
	return labelWidth(f.cache.image(f.key(text, 4096, scale, 0xffffff)))
}

func (f *browsingTypeface) Rasterize(text string, width, scale int, color uint32) (*image.NRGBA, int) {
	return f.cache.overlay(f.key(text, width, scale, color)), -3 * scale
}

func (s *sceneCache) typeface(w, h int) ui.Typeface {
	scale := float32(h) * 4 / float32(w*3)
	if s.face == nil || s.face.scaleY != scale {
		s.face = &browsingTypeface{cache: &s.text, scaleY: scale}
	}
	return s.face
}

var _ ui.Typeface = (*browsingTypeface)(nil)
