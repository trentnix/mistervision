package rendering

import (
	"fmt"

	"mistervision/internal/input/control"
	"mistervision/internal/media"
)

// PostPlayPresentation describes the completion screen. Next is absent while
// looking up the successor, after a lookup failure, or at the end of a series.
// The browser owns the choices. The renderer only displays the snapshot.
type PostPlayPresentation struct {
	Title, Message string
	Next           *media.Item
	Rows           []string
	Selected       int
}

// postPlay uses the next episode's cached artwork with a dark gradient, keeping
// text and actions legible before images arrive and within the CRT safe area.
func (p *screenPainter) postPlay(prompt *PostPlayPresentation) {
	c := p.canvas
	w, h := p.width, p.height
	p.previewBackdrop(h * 3 / 4)

	face := *p.cache.face
	face.bodyHeight, face.bold = 10, true
	body := *c
	body.Typeface = &face
	c = &body
	x, width := 32, w-64
	drawTitle := func(text string, y, size, lines int, color uint32) {
		if im := p.headingText(text, width, size, lines, color); im != nil {
			shadow := p.headingText(text, width, size, lines, 0x000000)
			c.Blit(shadow, x+1, y+1, shadow.Bounds().Dx(), shadow.Bounds().Dy())
			c.Blit(im, x, y, im.Bounds().Dx(), im.Bounds().Dy())
		}
	}
	drawTitle(prompt.Title, p.safeY+12, 24, 1, 0xffffff)

	hints := []controlHint{}
	if len(prompt.Rows) > 1 {
		hints = append(hints, pairedHint(p.scene.Controls, control.Up, control.Down, "Choose"))
	}
	hints = append(hints, hint(p.scene.Controls, control.Open, "Select"), hint(p.scene.Controls, control.Back, "Back"))
	controls := controlRows(w, hints)
	actionsTop := controlsTop(p.bottom, controls) - 12 - len(prompt.Rows)*30
	if item := prompt.Next; item != nil {
		title := p.headingText(item.Name, width, 34, 2, 0xffffff)
		titleHeight := 0
		if title != nil {
			titleHeight = title.Bounds().Dy()
		}
		// Measure wrapped titles before placing metadata so long names never
		// collide with actions at either progressive or interlaced geometry.
		titleY := actionsTop - 16 - titleHeight
		c.Text(x, titleY-35, item.SeriesName, 0xffffff, w-x)
		if item.ParentIndexNumber != nil && item.IndexNumber != nil {
			c.Text(x, titleY-18, fmt.Sprintf("Season %d · Episode %d", *item.ParentIndexNumber, *item.IndexNumber), 0xffffff, w-x)
		}
		if title != nil {
			c.Blit(title, x, titleY, title.Bounds().Dx(), titleHeight)
		}
	} else {
		c.Wrap(x, max(p.safeY+50, actionsTop-65), width, 3, prompt.Message, 0xffffff)
	}
	for i, label := range prompt.Rows {
		y := actionsTop + i*30
		if i == prompt.Selected {
			c.Rect(x-8, y, min(width+16, 400), 26, 0x0d377c)
		}
		c.Text(x, y+7, label, 0xffffff, w-x)
	}
	drawControls(c, p.bottom, controls)
}
