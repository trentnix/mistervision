package rendering

import (
	"mistervision/internal/caption"
	"mistervision/internal/input/control"
	"mistervision/internal/ui"
)

// drawTrackMenu uses the shared overlay canvas on every video output.
func drawTrackMenu(c *ui.Canvas, menu *TrackMenu, labels control.Labels) {
	// Keep this menu's larger type local so captions and playback controls retain
	// their own sizing when the menu closes.
	body := *c
	if face, ok := c.Typeface.(*browsingTypeface); ok {
		larger := *face
		larger.bodyHeight = 10
		body.Typeface = &larger
	}
	c = &body
	heading := body
	if face, ok := body.Typeface.(*browsingTypeface); ok {
		bold := *face
		bold.bodyHeight = 11
		bold.bold = true
		heading.Typeface = &bold
	}
	w, h := c.Width, c.Height
	sy := safeY(w, h)
	bottom := h - 8 - sy
	hints := []controlHint{pairedHint(labels, control.Previous, control.Next, "Tabs")}
	if len(menu.Rows) > 0 {
		hints = append(hints, hint(labels, control.Open, "Apply"))
	}
	hints = append(hints, hint(labels, control.Back, "Back"))
	if menu.Tab == 0 && menu.Delay != "" {
		hints = append(hints, hint(labels, control.SeekBackward, "Earlier"), hint(labels, control.SeekForward, "Later"))
	}
	controls := controlRows(w, hints)
	c.Shade(12, sy-4, w-24, h-2*sy+12, 225)
	tabY := sy + 4
	for i, title := range []string{"Subtitles", "Audio", "Picture"} {
		x := w*(2*i+1)/6 - heading.MeasureText(title, 1)/2
		color := uint32(dimColor)
		if menu.Tab == i {
			color = 0xffffff
			c.Rect(x-6, tabY+17, heading.MeasureText(title, 1)+12, 2, titleColor)
		}
		heading.Text(x, tabY, title, color, w-24)
	}
	top := tabY + 30
	footer := controlsTop(bottom, controls) - 6
	if menu.Delay != "" {
		footer -= 16
		center(c, footer, menu.Delay, dimColor, 1)
	}
	if menu.Message != "" {
		lines := messageLines(menu.Message, w-48, 4)
		footer -= len(lines)*14 + 4
		for i, line := range lines {
			center(c, footer+i*14, line, titleColor, 1)
		}
	}
	const rowHeight = 24
	rows := max(1, (footer-top)/rowHeight)
	start := max(0, min(menu.Selected-rows/2, len(menu.Rows)-rows))
	for i := start; i < min(len(menu.Rows), start+rows); i++ {
		y := top + (i-start)*rowHeight
		row := menu.Rows[i]
		if i == menu.Selected {
			c.Rect(20, y, w-40, rowHeight-2, 0x0d377c)
		}
		// An applied choice keeps its marker when focus moves to another row.
		// Reserve one column so every label starts at the same position.
		if row.Active {
			c.Rect(29, y+8, 6, 5, 0xffffff)
		}
		c.Text(46, y+5, row.Label, 0xffffff, w-44)
	}
	if start > 0 {
		c.Text(w-32, top, "^", titleColor, w-16)
	}
	if start+rows < len(menu.Rows) {
		c.Text(w-32, top+(rows-1)*rowHeight, "v", titleColor, w-16)
	}
	drawControls(c, bottom, controls)
}

// drawSubtitle positions a cached Unicode cue above playback controls.
func drawSubtitle(c *ui.Canvas, captions *caption.Renderer, text string, bottom int) {
	image := captions.Image(text, c.Width, c.Height)
	if image == nil {
		return
	}
	c.Overlay(image, (c.Width-image.Bounds().Dx())/2, bottom-image.Bounds().Dy())
}
