package rendering

import (
	"math"
	"unicode"

	"mistervision/internal/input/control"
	"mistervision/internal/ui"
)

const (
	carouselNameWidth = 240
	carouselNameRunes = 64
)

// carousel draws library names over cached artwork strips and returns its controls.
func (p *screenPainter) carousel() [][]controlHint {
	c := p.canvas
	cache := p.cache
	art := p.scene.Artwork
	w, h := p.width, p.height
	sy := p.safeY
	anim := p.animation
	v := &p.scene.Content

	if p.background == nil {
		if p.scene.Background != nil {
			cache.customBackground(c, p.scene.Background)
		} else if len(art.Covers) > 0 {
			music := v.Item() != nil && v.Item().CollectionType == "music"
			cache.mosaic(c, art.Covers, music, anim.Seconds)
		}
	}
	p.homeHeader()
	centers := make([]float64, len(v.Page.Items))
	names := make([]*ui.RasterImage, len(centers))
	rowHeight := 0
	for i, item := range v.Page.Items {
		color := uint32(0xffffff)
		if i == v.Selected {
			color = titleColor
		}
		names[i] = p.centeredHeadingText(carouselName(item.Name), carouselNameWidth, 34, 2, color)
		if names[i] != nil {
			rowHeight = max(rowHeight, names[i].Bounds().Dy())
		}
		if i > 0 {
			centers[i] = centers[i-1] + float64(labelWidth(names[i-1])+labelWidth(names[i]))/2 + 74
		}
	}
	if len(centers) > 0 {
		pos := max(0, min(float64(len(centers)-1), anim.Selection))
		lo := int(pos)
		hi := min(lo+1, len(centers)-1)
		origin := centers[lo] + (centers[hi]-centers[lo])*(pos-float64(lo))
		cy := (sy + 24 + h - sy - 28) / 2
		rowTop := cy - rowHeight/2 - 4
		for i, name := range names {
			x := w/2 + int(centers[i]-origin) - labelWidth(name)/2
			if name != nil {
				y := rowTop + (rowHeight-name.Bounds().Dy())/2
				if p.background != nil {
					p.background.full.BlitInViewport(c, name, x, y, name.Bounds().Dx(), name.Bounds().Dy())
				} else {
					c.Blit(name, x, y, name.Bounds().Dx(), name.Bounds().Dy())
				}
			}
		}
		if p.background != nil {
			// Draw each name once, then copy only the shared row into the
			// foreground so final composition preserves the complete carousel.
			_, sy := c.Density()
			top := int(math.Round(float64(rowTop) * sy))
			bottom := int(math.Round(float64(rowTop+rowHeight) * sy))
			p.background.copyCenterRows(c, top, bottom)
		}
		text := ""
		if p.scene.LibraryLoading {
			text = "Loading..."
		} else if p.scene.LibraryCount != nil && v.Item() != nil {
			text = libraryCountText(*v.Item(), *p.scene.LibraryCount)
		}
		if label := p.primaryText(text, w-48, 18, 1, dimColor); label != nil {
			c.Blit(label, (w-label.Bounds().Dx())/2, rowTop+rowHeight+6, label.Bounds().Dx(), label.Bounds().Dy())
		}
	}
	labels := p.scene.Controls
	hints := []controlHint{pairedHint(labels, control.Previous, control.Next, "Browse")}
	if v.Item() != nil {
		hints = append(hints, hint(labels, control.Open, "Select"))
	}
	hints = append(hints, hint(labels, control.Select, "List"))
	if v.Error != "" || p.scene.SelectionError != "" {
		hints = append(hints, hint(labels, control.Retry, "Retry"))
	}
	hints = append(hints, hint(labels, control.About, "About"), hint(labels, control.Back, "Exit"))
	return controlRows(w, hints)
}

// labelWidth treats empty library names as an empty carousel slot.
func labelWidth(im *ui.RasterImage) int {
	if im == nil {
		return 0
	}
	return im.Bounds().Dx()
}

// carouselName bounds shaping and cache storage without changing the server name.
// Collapse whitespace so embedded line breaks cannot consume the two visible lines.
func carouselName(name string) string {
	runes := make([]rune, 0, carouselNameRunes+1)
	space := false
	for _, r := range name {
		if unicode.IsSpace(r) {
			space = len(runes) > 0
			continue
		}
		if unicode.IsControl(r) {
			continue
		}
		if space {
			runes = append(runes, ' ')
			space = false
		}
		runes = append(runes, r)
		if len(runes) > carouselNameRunes {
			return string(runes[:carouselNameRunes-1]) + "…"
		}
	}
	return string(runes)
}
