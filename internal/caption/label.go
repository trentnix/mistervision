package caption

import (
	"image"
	"image/color"
	"image/draw"
	"strings"
)

// Label returns owned, antialiased text using the caption fonts without their
// video outline. Layout uses physical pixels, then scaleY accounts for the
// display's pixel aspect ratio. Text is left aligned and truncated to limit
// lines. Callers must cache repeated labels and serialize use of the renderer.
func (r *Renderer) Label(text string, width, size, limit int, scaleY float32, ink color.RGBA) *image.RGBA {
	if width <= 6 || size <= 0 || limit <= 0 || scaleY <= 0 {
		return nil
	}
	runes := []rune(text)
	if len(runes) > 2048 {
		text = string(runes[:2048])
	}
	if strings.TrimSpace(text) == "" {
		return nil
	}
	r.fonts.init()
	lines := r.layoutLines(text, width-6, size, min(limit, 3))
	used := 0
	for _, line := range lines {
		advance := 0
		for _, run := range line {
			advance += int(run.Advance)
		}
		used = max(used, (advance+63)/64)
	}
	mask := rasterMask(lines, min(width, used+6), size, scaleY, false)
	if mask == nil {
		return nil
	}
	out := image.NewRGBA(mask.Bounds())
	draw.DrawMask(out, out.Bounds(), image.NewUniform(ink), image.Point{}, mask, image.Point{}, draw.Src)
	return out
}
