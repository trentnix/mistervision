package rendering

import (
	"image"
	"math"

	"mistervision/internal/input/control"
	"mistervision/internal/media"
	"mistervision/internal/ui"
)

const listSideWidth = 215

// list draws a paginated item list and returns its configured button badges.
func (p *screenPainter) list() [][]controlHint {
	c := p.canvas
	cache := p.cache
	art := p.scene.Artwork
	w, h := p.width, p.height
	sy := p.safeY
	anim := p.animation
	v := &p.scene.Content
	s := p.scene
	live := v.Item() != nil && media.IsLive(*v.Item())
	if live {
		art.Primary = nil
	}
	var hints []controlHint
	if v.Item() != nil {
		hints = append(hints, hint(s.Controls, control.Open, "Select"))
	}
	if v.CanShuffle {
		hints = append(hints, hint(s.Controls, control.Select, "Shuffle all"))
	}
	back := "Back"
	if s.Root {
		hints = append(hints, hint(s.Controls, control.Select, "Carousel"))
		back = "Exit"
	}
	if v.Error != "" || s.SelectionError != "" {
		hints = append(hints, hint(s.Controls, control.Retry, "Retry"))
	}
	hints = append(hints, hint(s.Controls, control.Back, back))
	controls := controlRows(w, hints)
	top := sy + 21
	if s.Root {
		top += 24 // Leave the shared home status and viewer row unobstructed.
	}
	artBox := image.Rect(w-24-listSideWidth, top, w-24, max(top+1, controlsTop(p.bottom, controls)-12))

	cache.backdrop(c, art, false, s.Background, artBox, func(layer *ui.Canvas) {
		if p.background != nil {
			copy(layer.Pixels, c.Pixels)
		} else if s.Background != nil {
			cache.customBackground(layer, s.Background)
		} else if art.Backdrop != nil {
			heroHeight := h * 3 / 4
			layer.Blit(art.Backdrop, 0, 0, w, heroHeight)
			for y := 0; y < heroHeight; y++ {
				brightness := 110 * (255 - y*255/max(1, heroHeight-1)) / 255
				layer.Shade(0, y, w, 1, 255-brightness)
			}
		}
		layer.Image(art.Primary, artBox.Min.X, artBox.Min.Y, artBox.Dx(), artBox.Dy())
	})
	if s.Root {
		p.homeHeader()
	} else {
		p.header(s.title(), p.safeY)
	}
	width := w - 48
	if !s.Root || live || art.Primary != nil {
		width = w - 48 - listSideWidth - 10
	}
	// Borrow a vertical slice of the frame to clip moving rows without an
	// intermediate image or a full-frame copy. Header and footer stay fixed.
	rows := s.listRows(w, h)
	// Preserve the roomier library row pitch. Home shows fewer rows to leave
	// space for its status header without shrinking text or line spacing.
	rowHeight := max(20, min(30, (controlsTop(p.bottom, controls)-16-(sy+21))/VisibleRows(w, h)))
	rowHeight = min(rowHeight, max(20, (controlsTop(p.bottom, controls)-16-top)/rows))
	list := c.Rows(top, min(rows*rowHeight, h-top))
	if item := v.Item(); item != nil {
		title, line := p.listRowText(*item, width, true, live)
		geometry := placeListText(title, line, rowHeight)
		list.Rect(20, int(math.Round(anim.Row*float64(rowHeight))), width+8, geometry.barHeight, 0x0d377c)
	}
	scroll := float64(v.Scroll) + anim.ScrollOffset
	for index := max(0, int(math.Floor(scroll))); index < min(len(v.Page.Items), int(math.Ceil(scroll))+rows); index++ {
		item := v.Page.Items[index]
		y := int(math.Round((float64(index) - scroll) * float64(rowHeight)))
		title, line := p.listRowText(item, width, index == v.Selected, live)
		geometry := placeListText(title, line, rowHeight)

		if title != nil {
			list.Blit(title, 24, y+geometry.titleY, title.Bounds().Dx(), title.Bounds().Dy())
		}
		if line != nil {
			list.Blit(line, 34, y+geometry.subtitleY, line.Bounds().Dx(), line.Bounds().Dy())
		}
	}
	if len(v.Page.Items) == 0 && !v.Loading && v.Error == "" {
		center(c, h/2, "Nothing here", dimColor, 1)
	}
	if live {
		p.channelGuide()
	}
	return controls
}

// listTextPlacement balances the selection's visible ink, not font line boxes.
type listTextPlacement struct{ titleY, subtitleY, barHeight int }

// placeListText keeps a title-only row at the same top inset. Two-line rows
// receive equal whole-pixel padding above the title and below the subtext.
func placeListText(title, subtitle *ui.RasterImage, rowHeight int) listTextPlacement {
	height := func(im *ui.RasterImage) int {
		if im == nil {
			return 0
		}
		return im.Bounds().Dy()
	}
	titleHeight, subtitleHeight := height(title), height(subtitle)
	if subtitleHeight == 0 {
		return listTextPlacement{titleY: 3, barHeight: rowHeight - 2}
	}
	const gap = 2
	content := titleHeight + gap + subtitleHeight
	padding := max(1, (rowHeight-2-content)/2)
	return listTextPlacement{titleY: padding, subtitleY: padding + titleHeight + gap, barHeight: content + 2*padding}
}

// listRowText uses the same type sizes and metadata treatment in every home
// and library list. Label rasters and their ink bounds are cached.
func (p *screenPainter) listRowText(item media.Item, width int, selected, live bool) (*ui.RasterImage, *ui.RasterImage) {
	color := uint32(dimColor)
	if selected {
		color = 0xffffff
	}
	title := p.listText(itemTitle(item), width, 18, color, true)
	s, col := subtitle(item)
	if p.scene.Root {
		s = ""
		if item.LibraryCount != nil {
			s = libraryCountText(item, *item.LibraryCount)
		}
	}
	if live {
		current, _ := media.CurrentNext(p.scene.Guide[item.ID], p.scene.Now)
		s = "No guide information"
		if current != nil {
			s = current.Title
		}
	}
	if p.scene.Content.Continue {
		s, col = continueSubtitle(item), titleColor
	}
	const indent = 10
	line := p.listText(s, width-indent, 16, col, false)
	return title, line
}
