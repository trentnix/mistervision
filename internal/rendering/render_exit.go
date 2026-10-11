package rendering

import "mistervision/internal/input/control"

// exitConfirmation draws the same confirmation above every screen, including
// the video overlay. Cancel leaves the underlying screen and playback untouched.
func (p *screenPainter) exitConfirmation() {
	c, s, w, h := p.canvas, p.scene, p.width, p.height

	rows := controlRows(w, []controlHint{hint(s.Controls, control.Open, "Exit"), hint(s.Controls, control.Back, "Cancel")})
	// Use the shared dense list text path to retain sharp outlines and ink
	// bounds at both native CRT and HDMI raster densities.
	size := max(8, int(14/(float64(h)*4/float64(w*3))+0.5))
	title := p.listText("Exit?", w-48, size, titleColor, true)
	ink := title.Bounds()
	const padding, gap, badgeHeight = 8, 8, 14
	controlsHeight := badgeHeight + max(0, len(rows)-1)*controlRowHeight
	height := padding*2 + ink.Dy() + gap + controlsHeight
	top := (h - height) / 2
	c.Shade(0, top, w, height, 210)
	if p.background != nil {
		p.background.full.Shade(0, top, w, height, 210)
	}
	c.Blit(title, (w-ink.Dx())/2, top+padding, ink.Dx(), ink.Dy())
	drawControls(c, top+height-padding-badgeHeight+3+controlBottomInset, rows)
}
