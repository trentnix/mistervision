package ui

import "image"

// Typeface measures and rasterizes styled text independently of the canvas.
// Images are borrowed and immutable. Rasterize returns a vertical offset from
// the requested text origin. Implementations own any caches and run serially.
type Typeface interface {
	Measure(text string, scale int) int
	Rasterize(text string, width, scale int, color uint32) (*image.NRGBA, int)
}

// MeasureText uses the same face as TextScaled, including its requested scale.
func (c *Canvas) MeasureText(text string, scale int) int {
	if scale <= 0 {
		return 0
	}
	if c.Typeface != nil {
		return c.Typeface.Measure(text, scale)
	}
	return TextWidth(text) * scale
}
