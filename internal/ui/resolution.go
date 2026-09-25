package ui

import (
	"image"
	"math"
)

// NewRaster keeps layout coordinates independent of the backing pixel size.
// Dimensions must be positive. The returned opaque canvas draws into a tightly
// packed BGRX raster.
func NewRaster(w, h, pixelWidth, pixelHeight int) *Canvas {
	if w == pixelWidth && h == pixelHeight {
		return New(w, h)
	}
	c := &Canvas{Width: w, Height: h, raster: New(pixelWidth, pixelHeight)}
	c.Pixels = c.raster.Pixels
	c.scaleX, c.scaleY = float64(pixelWidth)/float64(w), float64(pixelHeight)/float64(h)
	return c
}

// RasterSize returns the dimensions of Pixels, independently of layout size.
func (c *Canvas) RasterSize() (int, int) {
	if c.raster != nil {
		return c.raster.Width, c.raster.Height
	}
	return c.Width, c.Height
}

// Density reports the number of raster pixels per logical coordinate unit.
func (c *Canvas) Density() (float64, float64) {
	if c.raster != nil {
		return c.scaleX, c.scaleY
	}
	return 1, 1
}

// NewLayer allocates an opaque cache layer with matching layout and density.
func (c *Canvas) NewLayer() *Canvas {
	w, h := c.RasterSize()
	return NewRaster(c.Width, c.Height, w, h)
}

func (c *Canvas) rasterBox(x, y, w, h int) (int, int, int, int) {
	sx, sy := c.Density()
	left, top := int(math.Round(float64(x)*sx)), int(math.Round(float64(y)*sy))
	return left, top, int(math.Round(float64(x+w)*sx)) - left, int(math.Round(float64(y+h)*sy)) - top
}

// Rows borrows a vertical region for clipped drawing using local coordinates.
// The region shares pixels and typeface with its parent and must not outlive it.
func (c *Canvas) Rows(top, height int) *Canvas {
	top = max(0, min(c.Height, top))
	height = max(0, min(c.Height-top, height))
	out := *c
	out.Height = height
	w, _ := c.RasterSize()
	_, y, _, h := c.rasterBox(0, top, c.Width, height)
	out.Pixels = c.Pixels[y*w*4 : (y+h)*w*4]
	if c.raster != nil {
		raster := *c.raster
		raster.Height, raster.Pixels = h, out.Pixels
		out.raster = &raster
	}
	return &out
}

// RasterImage retains logical image bounds while carrying a sharper source.
// Layout measures RGBA. Blit uses Source when drawing at increased density.
// Both images must be immutable and describe the same content.
type RasterImage struct {
	*image.RGBA
	Source *image.RGBA
}

// SubImage crops both representations using the logical coordinate rectangle.
func (im *RasterImage) SubImage(r image.Rectangle) image.Image {
	r = r.Intersect(im.Bounds())
	b, s := im.Bounds(), im.Source.Bounds()
	if r.Empty() || b.Empty() {
		return &RasterImage{RGBA: &image.RGBA{}, Source: &image.RGBA{}}
	}
	mapped := image.Rect(s.Min.X+(r.Min.X-b.Min.X)*s.Dx()/b.Dx(), s.Min.Y+(r.Min.Y-b.Min.Y)*s.Dy()/b.Dy(), s.Min.X+(r.Max.X-b.Min.X)*s.Dx()/b.Dx(), s.Min.Y+(r.Max.Y-b.Min.Y)*s.Dy()/b.Dy())
	return &RasterImage{RGBA: im.RGBA.SubImage(r).(*image.RGBA), Source: im.Source.SubImage(mapped).(*image.RGBA)}
}

// DenseTypeface can rasterize outlines at canvas density without changing
// logical measurements or line breaks. Ordinary typefaces remain supported.
type DenseTypeface interface {
	RasterizeDense(text string, width, scale int, color uint32, sx, sy float64) (image.Image, int)
}

// CopyRows copies an opaque logical row strip into a canvas at its density.
// It is used for cached scrolling backgrounds, which need no font rasterization.
func (c *Canvas) CopyRows(src *Canvas, sourceX, destY int) {
	rw, rh := c.RasterSize()
	_, top, _, height := c.rasterBox(0, destY, c.Width, src.Height)
	for y := max(0, top); y < min(rh, top+height); y++ {
		sy := (y - top) * src.Height / height
		for x := 0; x < rw; x++ {
			at := (sy*src.Width + sourceX + x*c.Width/rw) * 4
			dst := (y*rw + x) * 4
			copy(c.Pixels[dst:dst+4], src.Pixels[at:at+4])
		}
	}
}

// BlitInViewport draws using a centered viewport's logical coordinates and pixel
// density, but clips at this canvas's edges. This lets content extend beyond the
// viewport without stretching its text or changing its position within it.
func (c *Canvas) BlitInViewport(viewport *Canvas, im image.Image, x, y, w, h int) {
	if im == nil {
		return
	}
	if viewport.raster != nil {
		if dense, ok := im.(*RasterImage); ok {
			im = dense.Source
		}
	}
	x, y, w, h = viewport.rasterBox(x, y, w, h)
	vw, vh := viewport.RasterSize()
	cw, ch := c.RasterSize()
	destination := c
	if c.raster != nil {
		destination = c.raster
	}
	destination.Blit(im, x+(cw-vw)/2, y+(ch-vh)/2, w, h)
}
