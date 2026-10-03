package rendering

import (
	"image"
	"math"

	"mistervision/internal/musicviz"
	"mistervision/internal/ui"
)

// drawMusic extends the effect using the same proportional crop as the carousel.
// Effects keep their 4:3 drawing coordinates and run at logical resolution, so
// HDMI raster density does not multiply animation work. Artwork bounds compensate
// for the crop, keeping the rotating album in the centered foreground viewport.
func (b *wideBackdrop) drawMusic(foreground *ui.Canvas, renderer *musicviz.Renderer, library *musicviz.Library, index int, frame musicviz.Frame, backdrop image.Image, headerBottom int) {
	w, h := foreground.Width, foreground.Height
	if b.music == nil || b.music.Width != w || b.music.Height != h {
		b.music = ui.New(w, h)
	}
	clear(b.music.Pixels)
	rw, _ := foreground.RasterSize()
	scale := float64(rw) / float64(b.width)
	mapX := func(x int) int { return int(math.Round(float64(w)/2 + (float64(x)-float64(w)/2)*scale)) }
	mapY := func(y int) int { return int(math.Round(float64(h)/2 + (float64(y)-float64(h)/2)*scale)) }
	box := frame.ArtworkBounds
	frame.ArtworkBounds = image.Rect(mapX(box.Min.X), mapY(box.Min.Y), mapX(box.Max.X), mapY(box.Max.Y))
	renderer.Draw(b.music, library, index, frame)
	b.canvas.CopyRows(b.music, 0, 0)
	_, bh := b.canvas.RasterSize()
	top := (bh - b.height) / 2
	copy(b.full.Pixels, b.canvas.Pixels[top*b.width*4:(top+b.height)*b.width*4])
	if index == musicviz.ArtworkBackground && backdrop != nil {
		b.cache.customBackgroundAspect(b.full, backdrop, float64(b.width)/float64(rw)*(4.0/3))
	}
	b.full.Shade(0, 0, w, headerBottom, 155)
	b.copyCenter(foreground)
}
