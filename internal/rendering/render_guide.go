package rendering

import (
	"time"

	"mistervision/internal/media"
)

// channelGuide uses the list's artwork column for a compact now/next schedule.
// Times follow the device's local zone. Missing listings never add an extra step
// before playback or replace the channel list with an error screen.
func (p *screenPainter) channelGuide() {
	item := p.scene.Content.Item()
	if item == nil {
		return
	}
	current, next := media.CurrentNext(p.scene.Guide[item.ID], p.scene.Now)
	x, y, width := p.width-24-listSideWidth, p.safeY+25, listSideWidth
	c := p.canvas
	if current == nil && next == nil {
		return
	}
	// Grow the logo upward, keeping its lower edge and the program rows fixed.
	c.Image(p.scene.Artwork.Primary, x, y-12, width, 36)
	y += 32
	draw := func(label string, program *media.Program) {
		if program == nil {
			return
		}
		c.Text(x, y, label, titleColor, x+width)
		title := p.primaryText(program.Title, width, 18, 2, 0xffffff)
		scheduleY := y + 13
		if title != nil {
			c.Blit(title, x, y+11, title.Bounds().Dx(), title.Bounds().Dy())
			scheduleY = y + 11 + title.Bounds().Dy() + 2
		}
		times := program.Start.In(time.Local).Format("15:04") + " - " + program.End.In(time.Local).Format("15:04")
		c.Text(x, scheduleY, times, dimColor, x+width)
		y += 64
	}
	draw("Now", current)
	draw("Next", next)
}
