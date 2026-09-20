package rendering

import (
	"fmt"
	"image"

	"mistervision/internal/input/control"
	"mistervision/internal/musicviz"
	"mistervision/internal/ui"
)

const titleColor = 0xffe040
const dimColor = 0x808080

func safeY(w, h int) int { return int(24*float64(h*4)/float64(w*3) + 0.5) }

// VisibleRows returns the list capacity within the CRT safe area for positive logical dimensions.
// Navigation must use this capacity when centering a selection or retaining pages.
func VisibleRows(w, h int) int { return max(1, (h-2*safeY(w, h)-32)/30) }

// HomeVisibleRows reserves one row for the home update and profile information.
// Home lists keep library typography instead of compressing every row to fit.
func HomeVisibleRows(w, h int) int { return max(1, VisibleRows(w, h)-1) }

func (s Scene) listRows(w, h int) int {
	if s.Root {
		return HomeVisibleRows(w, h)
	}
	return VisibleRows(w, h)
}

// homeHeader keeps identity and update status in the same place in both home views.
func (p *screenPainter) homeHeader() {
	c, w, sy := p.canvas, p.width, p.safeY
	p.header(p.scene.title(), sy)
	if p.scene.About.Release.Available {
		label := p.primaryText("Update available", w/2-32, 16, 1, titleColor)
		if label != nil {
			c.Blit(label, 24, sy+15, label.Bounds().Dx(), label.Bounds().Dy())
		}
	}
	if profile := p.scene.About.Profile; profile != nil {
		name := truncate(profile.Name, w/2-32-profileLabelInset, 1)
		drawProfileLabel(c, w-24-c.MeasureText(name, 1)-profileLabelInset, sy+24, name, profile.Avatar, dimColor)
	}
}

func textWidth(s string, scale int) int { return ui.TextWidth(s) * scale }

func truncate(s string, width, scale int) string {
	return ui.TruncateText(s, width/scale)
}

func center(c *ui.Canvas, y int, s string, color uint32, scale int) {
	c.TextScaled((c.Width-c.MeasureText(s, scale))/2, y, s, color, c.Width, scale)
}

func runtime(ticks int64) string {
	seconds := max(int64(0), ticks/10000000)
	if seconds >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", seconds/3600, seconds/60%60, seconds%60)
	}
	return fmt.Sprintf("%d:%02d", seconds/60, seconds%60)
}

// screenPainter borrows one frame's canvas, scene, and renderer-owned cache.
// It computes shared CRT layout once and dispatches drawing without owning
// persistent state. Methods run synchronously during Render.
type screenPainter struct {
	visualizer                   *musicviz.Renderer
	canvas                       *ui.Canvas
	cache                        *sceneCache
	scene                        Scene
	animation                    Animation
	width, height, safeY, bottom int
}

// clock paints the shared local-time display in the top safe area.
func (p *screenPainter) clock() {
	p.canvas.BitmapText(p.width-72, p.safeY+4, p.scene.Now.Format("15:04"), dimColor, p.width-32)
}

// header truncates the root heading and scrolls longer library or item titles.
// Both stay inside the safe area reserved beside the clock.
func (p *screenPainter) header(title string, titleY int) {
	end := p.width - 84
	width := end - 24
	limit := 4096 // Bound raster storage for unusually long scrolling headings.
	if p.scene.Root {
		limit = width
	}
	titleImage := p.headingText(title, limit, 28, 1, titleColor)
	if titleImage != nil {
		x := 24
		advance := titleImage.Bounds().Dx()
		if !p.scene.Root && advance > width {
			x -= int(p.animation.TitleSeconds*15) % (advance + 40)
		}
		draw := func(x int) {
			left, right := max(24, x), min(end, x+advance)
			if left >= right {
				return
			}
			crop := titleImage.SubImage(image.Rect(left-x, 0, right-x, titleImage.Bounds().Dy()))
			p.canvas.Blit(crop, left, titleY-3, right-left, crop.Bounds().Dy())
		}
		draw(x)
		if x < 24 {
			draw(x + advance + 40)
		}
	}
	p.clock()
}

// footer draws browsing hints, request errors, and modal notices in that order.
func (p *screenPainter) footer(controls [][]controlHint) {
	c := p.canvas
	w, h := p.width, p.height
	bottom := p.bottom
	s := p.scene
	messageY := bottom - 14
	if len(controls) > 0 {
		drawControls(c, bottom, controls)
		messageY = controlsTop(bottom, controls) - 12
	}
	messageWidth := w - 48
	v := &s.Content
	if v.Detail == nil && (!s.Root || s.ListMode) && v.Page.TotalRecordCount != nil && *v.Page.TotalRecordCount > s.listRows(w, h) {
		count := v.Count()
		countWidth := c.MeasureText(count, 1)
		c.Text(w-24-countWidth, messageY, count, dimColor, w-24)
		messageWidth -= countWidth + 16
	}
	message := p.footerMessage()
	if message != "" {
		if c.MeasureText(message, 1) > messageWidth {
			// Longer explanations wrap above the controls instead of losing
			// their recovery instructions to footer truncation.
			lines := messageLines(message, w-88, 6)
			drawNotice(c, "", message, max(p.safeY, messageY-len(lines)*10-24), 6)
		} else {
			c.Text(24+(messageWidth-c.MeasureText(message, 1))/2, messageY, message, 0xff6060, w-24)
		}
	}
	if s.ExitConfirm {
		rows := controlRows(w, []controlHint{hint(s.Controls, control.Open, "Exit"), hint(s.Controls, control.Back, "Cancel")})
		// Use visible glyph bounds so font padding cannot unbalance the stripe.
		title, _ := c.Typeface.Rasterize("Exit?", w-48, 2, titleColor)
		var ink image.Rectangle
		for y := range title.Rect.Dy() {
			for x := range title.Rect.Dx() {
				if title.Pix[y*title.Stride+x*4+3] != 0 {
					ink = ink.Union(image.Rect(x, y, x+1, y+1))
				}
			}
		}
		const padding, gap, badgeHeight = 8, 8, 14
		controlsHeight := badgeHeight + max(0, len(rows)-1)*controlRowHeight
		height := padding*2 + ink.Dy() + gap + controlsHeight
		top := (h - height) / 2
		c.Shade(0, top, w, height, 210)
		c.Overlay(title.SubImage(ink).(*image.NRGBA), (w-ink.Dx())/2, top+padding)
		drawControls(c, top+height-padding-badgeHeight+3+controlBottomInset, rows)
	} else if s.Notice != "" {
		drawNotice(c, "", s.Notice, -1, 6)
	}
}

// footerMessage supplies the status text that browsing layouts reserve room for.
func (p *screenPainter) footerMessage() string {
	v := &p.scene.Content
	if v.Error != "" {
		return v.Error
	}
	if v.Loading || (v.Fetching && v.Scroll+p.scene.listRows(p.width, p.height) > len(v.Page.Items)) {
		return "Loading..."
	}
	return p.scene.SelectionError
}
