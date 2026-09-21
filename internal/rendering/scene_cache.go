package rendering

import (
	"image"
	"math"

	"mistervision/internal/ui"
)

// sceneCache reuses static screen backgrounds, the current item backdrop, and
// one carousel's row strips, plus a bounded cache of shaped labels. It does not
// retain a second library-wide artwork
// cache. Source identity and output geometry determine validity. Unsupported
// image implementations bypass reuse.
type sceneCache struct {
	text                           primaryTextCache
	face                           *browsingTypeface
	customAspect                   float64
	customBase                     *ui.Canvas
	customSource, backgroundCustom image.Image
	setupBase                      *ui.Canvas
	setupLogoHeight                int
	aboutBase                      *ui.Canvas
	aboutStatusY                   int
	aboutCompact                   bool
	videoBackground                *ui.Canvas
	videoSource                    image.Image
	background                     *ui.Canvas
	source, cover                  image.Image
	coverBox                       image.Rectangle
	detail                         bool
	rows                           []*ui.Canvas
	covers                         []image.Image
	musicCanvas                    *ui.Canvas
	music                          bool
	width, height, tileWidth       int
}

func sameArtwork(a, b image.Image) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	// JPEG and PNG decoders also return YCbCr and NRGBA. These immutable
	// standard image pointers have the same identity contract as RGBA.
	switch x := a.(type) {
	case *image.RGBA:
		y, ok := b.(*image.RGBA)
		return ok && x == y
	case *image.NRGBA:
		y, ok := b.(*image.NRGBA)
		return ok && x == y
	case *image.YCbCr:
		y, ok := b.(*image.YCbCr)
		return ok && x == y
	}
	return false
}

func (s *sceneCache) backdrop(c *ui.Canvas, art Artwork, detail bool, custom image.Image, coverBox image.Rectangle, draw func(*ui.Canvas)) {
	if s == nil {
		draw(c)
		return
	}
	cover := art.Primary
	if detail {
		cover = nil
	}
	if s.background == nil || s.background.Width != c.Width || s.background.Height != c.Height || s.detail != detail || s.coverBox != coverBox || !sameArtwork(s.backgroundCustom, custom) || !sameArtwork(s.source, art.Backdrop) || !sameArtwork(s.cover, cover) {
		s.background = c.NewLayer()
		draw(s.background)
		s.source, s.cover, s.detail = art.Backdrop, cover, detail
		s.backgroundCustom = custom
		s.coverBox = coverBox
	}
	copy(c.Pixels, s.background.Pixels)
}

func mosaicGeometry(w, h int, music bool) (int, int) {
	cw := max(1, w/6)
	aspect := 2.0 / 3
	if music {
		aspect = 1
	}
	ch := max(1, int(float64(cw)/(aspect*float64(w*3)/float64(h*4))))
	return cw, ch
}

func (s *sceneCache) mosaic(c *ui.Canvas, covers []image.Image, music bool, seconds float64) {
	if s == nil {
		drawMosaic(c, covers, music, seconds)
		return
	}
	valid := s.width == c.Width && s.height == c.Height && s.music == music && len(s.covers) == len(covers)
	if valid {
		for i := range covers {
			if !sameArtwork(s.covers[i], covers[i]) {
				valid = false
				break
			}
		}
	}
	if !valid {
		cw, ch := mosaicGeometry(c.Width, c.Height, music)
		s.rows = nil
		for row := 0; row*ch < c.Height; row++ {
			// Include the offscreen tiles so scrolling only changes the copy offset.
			height := min(ch, c.Height-row*ch)
			sx, sy := c.Density()
			top := int(math.Round(float64(row*ch) * sy))
			bottom := int(math.Round(float64(row*ch+height) * sy))
			strip := ui.NewRaster(9*cw, height, int(math.Round(float64(9*cw)*sx)), bottom-top)
			for col := -1; col < 8; col++ {
				idx := (row*7 + col + 13) % len(covers)
				strip.Blit(covers[idx], (col+1)*cw, 0, cw, ch)
			}
			strip.Shade(0, 0, strip.Width, strip.Height, 145)
			for y := 0; y < strip.Height; y++ {
				strip.Shade(0, y, strip.Width, 1, (row*ch+y)*180/max(1, c.Height-1))
			}
			s.rows = append(s.rows, strip)
		}
		s.covers = append(s.covers[:0], covers...)
		s.width, s.height, s.tileWidth, s.music = c.Width, c.Height, cw, music
	}
	y := 0
	for row, strip := range s.rows {
		shift := int(seconds*10) % s.tileWidth
		if row%2 == 0 {
			shift = -shift
		}
		x := s.tileWidth - shift
		if rw, rh := c.RasterSize(); rw != c.Width || rh != c.Height {
			sw, sh := strip.RasterSize()
			sourceX := min(sw-rw, int(math.Round(float64(x)*float64(sw)/float64(strip.Width))))
			_, scaleY := c.Density()
			top := int(math.Round(float64(y) * scaleY))
			for sy := 0; sy < sh; sy++ {
				src := (sy*sw + sourceX) * 4
				dst := ((top + sy) * rw) * 4
				copy(c.Pixels[dst:dst+rw*4], strip.Pixels[src:src+rw*4])
			}
			y += strip.Height
			continue
		}
		for sy := 0; sy < strip.Height; sy++ {
			src := (sy*strip.Width + x) * 4
			dst := (y * c.Width) * 4
			copy(c.Pixels[dst:dst+c.Width*4], strip.Pixels[src:src+c.Width*4])
			y++
		}
	}
}

// drawMosaic is the uncached path used by standalone renders and pixel tests.
func drawMosaic(c *ui.Canvas, covers []image.Image, music bool, seconds float64) {
	w, h := c.Width, c.Height
	cw, ch := mosaicGeometry(w, h, music)
	for row := 0; row*ch < h; row++ {
		shift := int(seconds*10) % cw
		if row%2 == 0 {
			shift = -shift
		}
		for col := -1; col < 8; col++ {
			idx := (row*7 + col + 13) % len(covers)
			c.Blit(covers[idx], col*cw+shift, row*ch, cw, ch)
		}
	}
	c.Shade(0, 0, w, h, 145)
	for y := 0; y < h; y++ {
		c.Shade(0, y, w, 1, y*180/max(1, h-1))
	}
}

// videoBackdrop prepares the static companion-player background once. All
// outputs share this frame without repeatedly scaling artwork during decoding.
func (s *sceneCache) videoBackdrop(c *ui.Canvas, source image.Image) {
	if s == nil {
		c.Image(source, 0, 0, c.Width, c.Height)
		return
	}
	if s.videoBackground == nil || s.videoBackground.Width != c.Width || s.videoBackground.Height != c.Height || !sameArtwork(s.videoSource, source) {
		s.videoBackground = c.NewLayer()
		s.videoBackground.Image(source, 0, 0, c.Width, c.Height)
		s.videoSource = source
	}
	copy(c.Pixels, s.videoBackground.Pixels)
}
