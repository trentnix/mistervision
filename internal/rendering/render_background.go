package rendering

import (
	"image"
	"math"

	"mistervision/internal/ui"
)

// customBackground caches a dimmed, centered crop in physical 4:3 screen space.
// Carousel and list frames share the prepared pixels. Source images are immutable.
func (s *sceneCache) customBackground(c *ui.Canvas, source image.Image) {
	s.customBackgroundAspect(c, source, 4.0/3)
}

// customBackgroundAspect fits artwork to the physical screen without stretching.
func (s *sceneCache) customBackgroundAspect(c *ui.Canvas, source image.Image, aspect float64) {
	if s == nil {
		drawCustomBackgroundAspect(c, source, aspect)
		return
	}
	if s.customAspect != aspect || s.customBase == nil || s.customBase.Width != c.Width || s.customBase.Height != c.Height || !sameArtwork(s.customSource, source) {
		s.customBase = c.NewLayer()
		drawCustomBackgroundAspect(s.customBase, source, aspect)
		s.customAspect = aspect
		s.customSource = source
	}
	copy(c.Pixels, s.customBase.Pixels)
}

func drawCustomBackground(c *ui.Canvas, source image.Image) {
	drawCustomBackgroundAspect(c, source, 4.0/3)
}

func drawCustomBackgroundAspect(c *ui.Canvas, source image.Image, aspect float64) {
	b := source.Bounds()
	if b.Empty() {
		return
	}
	par := float64(c.Width) / (float64(c.Height) * aspect)
	scale := max(float64(c.Width)/float64(b.Dx()), float64(c.Height)*par/float64(b.Dy()))
	w := int(math.Ceil(float64(b.Dx()) * scale))
	h := int(math.Ceil(float64(b.Dy()) * scale / par))
	c.Blit(source, (c.Width-w)/2, (c.Height-h)/2, w, h)
	c.Shade(0, 0, c.Width, c.Height, 145)
}
