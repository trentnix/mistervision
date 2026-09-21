package rendering

import (
	"math"

	"mistervision/internal/ui"
)

// carouselBackdrop crops a cached 4:3 background to the full display. The
// foreground remains centered at its original raster size and density.
type carouselBackdrop struct {
	width, height int
	canvas        *ui.Canvas
	cache         sceneCache
	full          *ui.Canvas
}

// SetDisplayAspect enables full-width carousel backgrounds on wide displays.
// Call before Render. Other screens retain their normal browsing viewport.
func (r *RasterRenderer) SetDisplayAspect(aspect float64) {
	r.wide = nil
	if aspect <= 4.0/3 || r.rasterWidth <= 0 || r.rasterHeight <= 0 || math.IsNaN(aspect) || math.IsInf(aspect, 0) {
		return
	}
	r.wide = &carouselBackdrop{width: int(math.Round(float64(r.rasterWidth) * aspect / (4.0 / 3))), height: r.rasterHeight}
}

func (b *carouselBackdrop) draw(foreground *ui.Canvas, s Scene, anim Animation) {
	rw, rh := foreground.RasterSize()
	// Extend the same pixel density in both directions, then crop vertically.
	height := max(b.height, int(math.Round(float64(rh)*float64(b.width)/float64(rw))))
	if b.canvas == nil || b.canvas.Width != foreground.Width || b.canvas.Height != foreground.Height {
		b.canvas = ui.NewRaster(foreground.Width, foreground.Height, b.width, height)
		b.cache = sceneCache{}
		b.full = ui.NewRaster(foreground.Width, foreground.Height, b.width, b.height)
	}
	clear(b.canvas.Pixels)
	if s.Background != nil {
		b.cache.customBackgroundAspect(b.full, s.Background, float64(b.width)/float64(rw)*(4.0/3))
	} else {
		if len(s.Artwork.Covers) > 0 {
			music := s.Content.Item() != nil && s.Content.Item().CollectionType == "music"
			b.cache.mosaic(b.canvas, s.Artwork.Covers, music, anim.Seconds)
		}
		_, bh := b.canvas.RasterSize()
		top := (bh - b.height) / 2
		copy(b.full.Pixels, b.canvas.Pixels[top*b.width*4:(top+b.height)*b.width*4])
	}
	left, top := (b.width-rw)/2, (b.height-rh)/2
	for y := 0; y < rh; y++ {
		copy(foreground.Pixels[y*rw*4:(y+1)*rw*4], b.full.Pixels[((top+y)*b.width+left)*4:((top+y)*b.width+left+rw)*4])
	}
}

func (b *carouselBackdrop) compose(foreground *ui.Canvas) []byte {
	rw, rh := foreground.RasterSize()
	left, top := (b.width-rw)/2, (b.height-rh)/2
	for y := 0; y < rh; y++ {
		copy(b.full.Pixels[((top+y)*b.width+left)*4:((top+y)*b.width+left+rw)*4], foreground.Pixels[y*rw*4:(y+1)*rw*4])
	}
	return b.full.Pixels
}
