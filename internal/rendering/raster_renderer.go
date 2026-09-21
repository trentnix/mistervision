package rendering

import (
	"mistervision/internal/caption"
	"mistervision/internal/musicviz"
	"mistervision/internal/ui"
	"mistervision/internal/videoout"
)

// RasterRenderer draws shared UI and video overlays in memory. The zero value is
// ready to use. Calls must be serial, and source artwork must remain immutable.
// Returned pixels are borrowed until the next Render call.
type RasterRenderer struct {
	rasterWidth, rasterHeight int
	wide                      *carouselBackdrop
	music                     musicviz.Renderer
	captions                  caption.Renderer
	canvas                    *ui.Canvas
	cache                     sceneCache
	overlay                   *ui.Canvas
	animation                 animationState
}

// NewRenderer constructs the shared software renderer used by all destinations.
func NewRenderer() *RasterRenderer { return &RasterRenderer{} }

// Render draws one scene at positive logical dimensions. It reuses frame storage,
// advances animation from Scene.Now, and invalidates visual caches on size changes.
// The caller must present or copy both pixel slices before calling Render again.
func (r *RasterRenderer) Render(w, h int, s Scene) videoout.Frame {
	rw, rh := w, h
	if !s.Video && r.rasterWidth > 0 && r.rasterHeight > 0 {
		rw, rh = r.rasterWidth, r.rasterHeight
	}
	r.prepare(w, h, rw, rh)
	anim := r.animation.advance(s, s.listRows(w, h))
	wide := r.wide != nil && s.Root && !s.ListMode && !s.Video && !s.Audio && !s.About.Visible && s.Setup.Kind == SetupHidden && s.Content.Detail == nil
	if wide {
		r.wide.draw(r.canvas, s, anim)
		// The backdrop is already painted. The carousel now draws only its foreground.
		s.Background = nil
		s.Artwork.Covers = nil
	}
	f := videoout.Frame{UIWidth: rw, UIHeight: rh, UI: renderSceneWithMusic(r.canvas, &r.cache, s, anim, &r.music), Video: s.Video}
	if s.Video {
		r.overlay.Typeface = r.canvas.Typeface
		clear(r.overlay.Pixels)
		f.Overlay = renderVideoOverlayOn(r.overlay, s.Playback, s.Now, s.Controls, &r.captions)
		drawMessage(r.overlay, s.Message, s.Now)
	}
	drawMessage(r.canvas, s.Message, s.Now)
	if wide {
		f.UI = r.wide.compose(r.canvas)
		f.UIWidth, f.UIHeight = r.wide.width, r.wide.height
		f.FullScreen = true
	}
	return f
}

var _ Renderer = (*RasterRenderer)(nil)

// prepare owns geometry invalidation for both frame buffers and artwork caches.
func (r *RasterRenderer) prepare(w, h, rw, rh int) {
	oldW, oldH := 0, 0
	if r.canvas != nil {
		oldW, oldH = r.canvas.RasterSize()
	}
	if r.canvas == nil || r.canvas.Width != w || r.canvas.Height != h || oldW != rw || oldH != rh {
		r.canvas = ui.NewRaster(w, h, rw, rh)
		r.overlay = ui.NewOverlay(w, h)
		r.cache = sceneCache{}
	} else {
		clear(r.canvas.Pixels)
	}
}

// NewRendererForRaster preserves logical layout while drawing browsing text and
// artwork at the destination's raster size. Video overlays remain logical.
func NewRendererForRaster(width, height int) *RasterRenderer {
	return &RasterRenderer{rasterWidth: width, rasterHeight: height}
}
