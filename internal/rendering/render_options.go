package rendering

import (
	"mistervision/internal/input/control"
)

// options draws a modal over the current preview or music background. It uses
// the shared browsing typeface and leaves navigation badges in their usual font.
func (p *screenPainter) options(menu *OptionsMenu) {
	c := p.canvas
	face := *p.cache.face
	face.bodyHeight = 10
	body := *c
	body.Typeface = &face
	c = &body
	w, h := p.width, p.height
	c.Shade(12, p.safeY, w-24, h-2*p.safeY, 235)
	center(c, p.safeY+14, menu.Title, 0xffffff, 2)
	top := p.safeY + 48
	for i, text := range menu.Rows {
		y := top + i*30
		if i == menu.Selected {
			c.Rect(24, y, w-48, 26, 0x0d377c)
		}
		c.Text(36, y+7, text, 0xffffff, w-36)
	}
	if menu.Message != "" {
		c.Wrap(36, top+len(menu.Rows)*30+10, w-72, 3, menu.Message, 0xffffff)
	}
	hints := []controlHint{}
	if !menu.Busy {
		if len(menu.Rows) > 1 {
			hints = append(hints, pairedHint(p.scene.Controls, control.Up, control.Down, "Choose"))
		}
		hints = append(hints, hint(p.scene.Controls, control.Open, "Apply"))
	}
	hints = append(hints, hint(p.scene.Controls, control.Back, "Back"))
	drawControls(c, p.bottom, controlRows(w, hints))
}
