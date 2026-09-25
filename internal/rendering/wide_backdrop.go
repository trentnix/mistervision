package rendering

import (
	"image"
	"math"

	"mistervision/internal/ui"
)

// wideBackdrop fills the display with solid backgrounds, artwork, or mosaics. The
// foreground remains centered at its original raster size and density.
type wideBackdrop struct {
	width, height int
	canvas        *ui.Canvas
	cache         sceneCache
	full          *ui.Canvas
	music         *ui.Canvas
}

// SetDisplayAspect enables full-width UI backgrounds on wide displays.
// Call before Render. Foreground layout stays in its centered 4:3 viewport.
func (r *RasterRenderer) SetDisplayAspect(aspect float64) {
	r.wide = nil
	r.cache = sceneCache{}
	if aspect <= 4.0/3 || r.rasterWidth <= 0 || r.rasterHeight <= 0 || math.IsNaN(aspect) || math.IsInf(aspect, 0) {
		return
	}
	r.wide = &wideBackdrop{width: int(math.Round(float64(r.rasterWidth) * aspect / (4.0 / 3))), height: r.rasterHeight}
}

func (b *wideBackdrop) draw(foreground *ui.Canvas, s Scene, anim Animation) {
	rw, rh := foreground.RasterSize()
	// Extend the same pixel density in both directions, then crop vertically.
	height := max(b.height, int(math.Round(float64(rh)*float64(b.width)/float64(rw))))
	if b.canvas == nil || b.canvas.Width != foreground.Width || b.canvas.Height != foreground.Height {
		b.canvas = ui.NewRaster(foreground.Width, foreground.Height, b.width, height)
		b.cache = sceneCache{}
		b.full = ui.NewRaster(foreground.Width, foreground.Height, b.width, b.height)
	}
	if s.About.Visible || s.Setup.Kind != SetupHidden {
		b.full.Rect(0, 0, b.full.Width, b.full.Height, 0x0b0d13)
	} else if s.Audio {
		clear(b.full.Pixels) // Music paints its background after calculating artwork bounds.
		return
	} else if s.Content.Detail != nil || !s.Root || s.ListMode {
		b.drawBrowsing(s, float64(b.width)/float64(rw)*(4.0/3))
	} else if s.Background != nil {
		b.cache.customBackgroundAspect(b.full, s.Background, float64(b.width)/float64(rw)*(4.0/3))
	} else {
		clear(b.canvas.Pixels)
		if len(s.Artwork.Covers) > 0 {
			music := s.Content.Item() != nil && s.Content.Item().CollectionType == "music"
			b.cache.mosaic(b.canvas, s.Artwork.Covers, music, anim.Seconds)
		}
		_, bh := b.canvas.RasterSize()
		top := (bh - b.height) / 2
		copy(b.full.Pixels, b.canvas.Pixels[top*b.width*4:(top+b.height)*b.width*4])
	}
	b.copyCenter(foreground)
}

// copyCenter seeds the centered foreground with its part of the full background.
func (b *wideBackdrop) copyCenter(foreground *ui.Canvas) {
	_, rh := foreground.RasterSize()
	b.copyCenterRows(foreground, 0, rh)
}

// copyCenterRows copies a raster row range without redrawing its text or artwork.
func (b *wideBackdrop) copyCenterRows(foreground *ui.Canvas, first, end int) {
	rw, rh := foreground.RasterSize()
	left, top := (b.width-rw)/2, (b.height-rh)/2
	for y := max(0, first); y < min(rh, end); y++ {
		copy(foreground.Pixels[y*rw*4:(y+1)*rw*4], b.full.Pixels[((top+y)*b.width+left)*4:((top+y)*b.width+left+rw)*4])
	}
}

// drawBrowsing caches the full-screen artwork independently of foreground posters.
func (b *wideBackdrop) drawBrowsing(s Scene, aspect float64) {
	detail := s.Content.Detail != nil
	custom := s.Background
	if detail {
		custom = nil
	}
	art := s.Artwork
	art.Primary = nil
	b.cache.backdrop(b.full, art, detail, custom, image.Rectangle{}, func(layer *ui.Canvas) {
		if custom != nil {
			b.cache.customBackgroundAspect(layer, custom, aspect)
			return
		}
		if art.Backdrop == nil {
			return
		}
		drawBackgroundAspect(layer, art.Backdrop, aspect)
		full := layer.Height * 3 / 4
		for y := 0; y < layer.Height; y++ {
			brightness := max(0, 255-y*255/max(1, full-1))
			if !detail {
				brightness = brightness * 110 / 255
			}
			layer.Shade(0, y, layer.Width, 1, 255-brightness)
		}
	})
}

func (b *wideBackdrop) compose(foreground *ui.Canvas) []byte {
	rw, rh := foreground.RasterSize()
	left, top := (b.width-rw)/2, (b.height-rh)/2
	for y := 0; y < rh; y++ {
		copy(b.full.Pixels[((top+y)*b.width+left)*4:((top+y)*b.width+left+rw)*4], foreground.Pixels[y*rw*4:(y+1)*rw*4])
	}
	return b.full.Pixels
}
