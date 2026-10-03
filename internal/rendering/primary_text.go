package rendering

import (
	"image"
	"image/color"
	"image/draw"
	"math"

	"mistervision/internal/caption"
	"mistervision/internal/ui"
)

// primaryTextCache bounds label images and reuses the caption font shaper.
// Entries exclude position so carousel motion does not invalidate text.
type primaryTextCache struct {
	renderer caption.Renderer
	images   map[primaryTextKey]*textRaster
	keys     []primaryTextKey
	next     int
}

type textRaster struct {
	image        *image.RGBA
	overlay      *image.NRGBA
	trimmed      *image.RGBA
	dense        *ui.RasterImage
	denseTrimmed *ui.RasterImage
	sx, sy       float64
}

type primaryTextKey struct {
	text               string
	width, size, lines int
	color              uint32
	scaleY             float32
	bold               bool
}

func (c *primaryTextCache) image(key primaryTextKey) *image.RGBA {
	if entry, ok := c.images[key]; ok {
		return entry.image
	}
	im := c.renderer.Label(key.text, key.width, key.size, key.lines, key.scaleY,
		color.RGBA{R: byte(key.color >> 16), G: byte(key.color >> 8), B: byte(key.color), A: 255})
	if key.bold && im != nil {
		// A one-pixel horizontal stroke gives the bundled regular face a
		// modest heading weight without changing its advance or line breaks.
		for y := 0; y < im.Rect.Dy(); y++ {
			for x := im.Rect.Dx() - 1; x > 0; x-- {
				i := y*im.Stride + x*4
				if im.Pix[i-1] > im.Pix[i+3] {
					copy(im.Pix[i:i+4], im.Pix[i-4:i])
				}
			}
		}
	}
	if c.images == nil {
		c.images = make(map[primaryTextKey]*textRaster)
	}
	const capacity = 128
	if len(c.keys) < capacity {
		c.keys = append(c.keys, key)
	} else {
		delete(c.images, c.keys[c.next])
		c.keys[c.next] = key
		c.next = (c.next + 1) % capacity
	}
	entry := &textRaster{image: im}
	if ink := textInk(im); !ink.Empty() {
		entry.trimmed = im.SubImage(image.Rect(0, ink.Min.Y, im.Rect.Max.X, ink.Max.Y)).(*image.RGBA)
	}
	c.images[key] = entry
	return im
}

// primaryText measures and renders using the same proportional font layout.
func (p *screenPainter) primaryText(text string, width, size, lines int, color uint32) *ui.RasterImage {
	return p.textImage(text, width, size, lines, color, false)
}

// headingText adds a modest weight to the same font used for primary text.
func (p *screenPainter) headingText(text string, width, size, lines int, color uint32) *ui.RasterImage {
	return p.textImage(text, width, size, lines, color, true)
}

func (p *screenPainter) textImage(text string, width, size, lines int, color uint32, bold bool) *ui.RasterImage {
	if p.cache == nil {
		p.cache = &sceneCache{}
	}
	key := primaryTextKey{text: text, width: width, size: size, lines: lines, color: color,
		scaleY: float32(p.height) * 4 / float32(p.width*3), bold: bold}
	sx, sy := p.canvas.Density()
	return p.cache.text.denseImage(key, sx, sy)
}

// overlay converts once per cached label for correct straight-alpha video output.
func (c *primaryTextCache) overlay(key primaryTextKey) *image.NRGBA {
	im := c.image(key)
	if im == nil {
		return nil
	}
	entry := c.images[key]
	if entry.overlay == nil {
		entry.overlay = image.NewNRGBA(im.Bounds())
		draw.Draw(entry.overlay, entry.overlay.Bounds(), im, im.Bounds().Min, draw.Src)
	}
	return entry.overlay
}

// textInk measures visible pixels once when a label enters the cache.
// Negligible alpha coverage can round to the background color when composited.
func textInk(im *image.RGBA) image.Rectangle {
	var ink image.Rectangle
	if im != nil {
		for y := range im.Rect.Dy() {
			for x := range im.Rect.Dx() {
				if im.Pix[y*im.Stride+x*4+3] >= 8 {
					ink = ink.Union(image.Rect(x, y, x+1, y+1))
				}
			}
		}
	}
	return ink
}

// listText removes vertical font padding without changing horizontal alignment.
func (p *screenPainter) listText(text string, width, size int, color uint32, bold bool) *ui.RasterImage {
	key := primaryTextKey{text: text, width: width, size: size, lines: 1, color: color,
		scaleY: float32(p.height) * 4 / float32(p.width*3), bold: bold}
	im := p.cache.text.image(key)
	if im == nil {
		return nil
	}
	trimmed := p.cache.text.images[key].trimmed
	if trimmed == nil {
		return nil
	}
	sx, sy := p.canvas.Density()
	dense := p.cache.text.denseImage(key, sx, sy)
	entry := p.cache.text.images[key]
	if entry.denseTrimmed == nil {
		// Logical ink bounds are not exact after independently rasterizing at
		// another density. Crop the source by its own ink so glyph bottoms survive.
		source := dense.Source
		ink := textInk(source)
		if !ink.Empty() {
			source = source.SubImage(image.Rect(source.Rect.Min.X, ink.Min.Y, source.Rect.Max.X, ink.Max.Y)).(*image.RGBA)
		}
		entry.denseTrimmed = &ui.RasterImage{RGBA: trimmed, Source: source}
	}
	return entry.denseTrimmed
}

// denseImage keeps logical metrics and high-resolution outlines in one cache entry.
func (c *primaryTextCache) denseImage(key primaryTextKey, sx, sy float64) *ui.RasterImage {
	im := c.image(key)
	if im == nil {
		return nil
	}
	entry := c.images[key]
	if entry.dense != nil && entry.sx == sx && entry.sy == sy {
		return entry.dense
	}
	source := im
	if sx != 1 || sy != 1 {
		source = c.renderer.LabelDensity(key.text, key.width, key.size, key.lines, key.scaleY, color.RGBA{R: byte(key.color >> 16), G: byte(key.color >> 8), B: byte(key.color), A: 255}, float32(sx), float32(sy))
		if key.bold {
			stroke := max(1, int(math.Round(sx)))
			for y := 0; y < source.Rect.Dy(); y++ {
				for x := source.Rect.Dx() - 1; x >= 0; x-- {
					at := y*source.Stride + x*4
					best := at
					for dx := 1; dx <= min(stroke, x); dx++ {
						if source.Pix[at-dx*4+3] > source.Pix[best+3] {
							best = at - dx*4
						}
					}
					copy(source.Pix[at:at+4], source.Pix[best:best+4])
				}
			}
		}
	}
	entry.denseTrimmed = nil
	entry.dense = &ui.RasterImage{RGBA: im, Source: source}
	entry.sx, entry.sy = sx, sy
	return entry.dense
}
