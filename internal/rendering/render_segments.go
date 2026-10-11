package rendering

import (
	"mistervision/internal/input/control"
	"mistervision/internal/ui"
)

// drawSegmentPrompt keeps a single contextual action above subtitles and controls,
// inset from CRT overscan. Button hints retain the common bitmap badge design.
func drawSegmentPrompt(c *ui.Canvas, label string, labels control.Labels) {
	rows := controlRows(c.Width, []controlHint{hint(labels, control.Options, label)})
	if len(rows) == 0 {
		return
	}
	action := rows[0][0]
	width := action.width() + 24
	x, y := c.Width-24-width, safeY(c.Width, c.Height)+12
	c.Shade(x, y, width, 30, 210)
	drawControlHint(c, x+12, y+11, action)
}
