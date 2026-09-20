package rendering

import (
	"image"
	"image/color"
	"image/draw"

	"mistervision/internal/caption"
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
	image   *image.RGBA
	overlay *image.NRGBA
	trimmed *image.RGBA
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
func (p *screenPainter) primaryText(text string, width, size, lines int, color uint32) *image.RGBA {
	return p.textImage(text, width, size, lines, color, false)
}

// headingText adds a modest weight to the same font used for primary text.
func (p *screenPainter) headingText(text string, width, size, lines int, color uint32) *image.RGBA {
	return p.textImage(text, width, size, lines, color, true)
}

func (p *screenPainter) textImage(text string, width, size, lines int, color uint32, bold bool) *image.RGBA {
	if p.cache == nil {
		p.cache = &sceneCache{}
	}
	key := primaryTextKey{text: text, width: width, size: size, lines: lines, color: color,
		scaleY: float32(p.height) * 4 / float32(p.width*3), bold: bold}
	return p.cache.text.image(key)
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
func (p *screenPainter) listText(text string, width, size int, color uint32, bold bool) *image.RGBA {
	key := primaryTextKey{text: text, width: width, size: size, lines: 1, color: color,
		scaleY: float32(p.height) * 4 / float32(p.width*3), bold: bold}
	im := p.cache.text.image(key)
	if im == nil {
		return nil
	}
	return p.cache.text.images[key].trimmed
}
