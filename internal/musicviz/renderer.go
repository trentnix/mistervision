package musicviz

import (
	"image"
	"time"

	"mistervision/internal/ui"
)

// ArtworkBackground identifies the server backdrop option supplied by the browser.
// It updates audio levels without drawing a generated effect.
const ArtworkBackground = -1

// Frame supplies output-independent animation input. Levels are linear stereo
// amplitudes in [0,1]. Artwork is immutable and borrowed during Draw.
type Frame struct {
	Now           time.Time
	Levels        [2]float64
	Paused        bool
	Stopped       bool // Clears audio levels and freezes the effect between playback sessions.
	Artwork       image.Image
	ArtworkBounds image.Rectangle
}

// Effect draws a background into a borrowed canvas. Each implementation owns
// its animation state. dt is elapsed seconds, already scaled by preset speed.
type Effect interface {
	Draw(*ui.Canvas, Frame, float64)
}

// Renderer owns the selected effect and latest audio levels on the render loop.
// Loading files and acquiring audio samples belong to other components.
type Renderer struct {
	library *Library
	index   int
	effect  Effect
	last    time.Time
	levels  [2]float64
}

// Draw advances the selected background. Configuration changes replace effect
// state. A long pause between frames never causes a large animation jump.
func (r *Renderer) Draw(c *ui.Canvas, l *Library, index int, f Frame) {
	if l == nil || index < ArtworkBackground || index >= len(l.Config.Backgrounds) {
		return
	}
	p := Preset{Type: "none", Speed: 1}
	if index != ArtworkBackground {
		p = l.Config.Backgrounds[index]
	}
	if r.library != l || r.index != index || r.effect == nil {
		r.library = l
		r.index = index
		r.effect = newEffect(p)
		r.last = f.Now
	}
	dt := min(.1, max(0, f.Now.Sub(r.last).Seconds()))
	r.last = f.Now
	if f.Stopped {
		dt = 0
	}
	for i, v := range f.Levels {
		if f.Paused || f.Stopped {
			r.levels[i] = 0
			continue
		}
		v = min(1, max(0, v))
		// The decoder already supplies RMS levels. Additional smoothing makes
		// audio-reactive effects trail the sound, particularly when a note ends.
		r.levels[i] = v
	}
	f.Levels = r.levels
	r.effect.Draw(c, f, dt*p.Speed)
}

func newEffect(p Preset) Effect {
	switch p.Type {
	case "starfield":
		return &starfield{preset: p}
	case "rain":
		return &rain{preset: p}
	case "nebula":
		return &nebula{preset: p}
	case "tunnel":
		return &tunnel{preset: p}
	case "spinning":
		return &spinning{preset: p}
	case "sprites":
		return &sprites{preset: p}
	case "image":
		return &imageEffect{preset: p}
	default:
		return emptyEffect{}
	}
}

type emptyEffect struct{}

func (emptyEffect) Draw(*ui.Canvas, Frame, float64) {}

func tint(color uint32, amount float64) uint32 {
	amount = min(1, max(0, amount))
	return uint32(float64((color>>16)&255)*amount)<<16 | uint32(float64((color>>8)&255)*amount)<<8 | uint32(float64(color&255)*amount)
}
func line(c *ui.Canvas, x0, y0, x1, y1 int, col uint32) {
	dx, dy := abs(x1-x0), -abs(y1-y0)
	sx, sy := -1, -1
	if x0 < x1 {
		sx = 1
	}
	if y0 < y1 {
		sy = 1
	}
	e := dx + dy
	for n := 0; n < 4096; n++ {
		c.Rect(x0, y0, 1, 1, col)
		if x0 == x1 && y0 == y1 {
			return
		}
		e2 := e * 2
		if e2 >= dy {
			e += dy
			x0 += sx
		}
		if e2 <= dx {
			e += dx
			y0 += sy
		}
	}
}
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
func energy(f Frame) float64 { return (f.Levels[0] + f.Levels[1]) * .5 }
