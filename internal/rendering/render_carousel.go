package rendering

import (
	"image"

	"mistervision/internal/input/control"
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

	if p.scene.Background != nil {
		cache.customBackground(c, p.scene.Background)
	} else if len(art.Covers) > 0 {
		music := v.Item() != nil && v.Item().CollectionType == "music"
		cache.mosaic(c, art.Covers, music, anim.Seconds)
	}
	p.homeHeader()
	centers := make([]float64, len(v.Page.Items))
	names := make([]*image.RGBA, len(centers))
	for i, item := range v.Page.Items {
		color := uint32(0xffffff)
		if i == v.Selected {
			color = titleColor
		}
		names[i] = p.headingText(item.Name, 160, 30, 1, color)
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
		for i, name := range names {
			x := w/2 + int(centers[i]-origin) - labelWidth(name)/2
			if name != nil {
				c.Blit(name, x, cy-14, name.Bounds().Dx(), name.Bounds().Dy())
			}
			if i == v.Selected && p.scene.LibraryLoading {
				c.Text(w/2-c.MeasureText("Loading...", 1)/2, cy+12, "Loading...", dimColor, w)
			} else if i == v.Selected && p.scene.LibraryCount != nil {
				count := libraryCountText(v.Page.Items[i], *p.scene.LibraryCount)
				c.Text(w/2-c.MeasureText(count, 1)/2, cy+12, count, dimColor, w)
			}
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
func labelWidth(im *image.RGBA) int {
	if im == nil {
		return 0
	}
	return im.Bounds().Dx()
}
