package rendering

import (
	"image"
	"strings"

	"mistervision/internal/input/control"
	"mistervision/internal/musicviz"
	"mistervision/internal/ui"
)

// music draws the current track, elapsed time, and optional controls.
func (p *screenPainter) music() {
	c := p.canvas
	art := p.scene.Artwork
	w, h := p.width, p.height
	sy := p.safeY
	bottom := p.bottom
	v := &p.scene.Content
	s := p.scene

	actions := playbackHints(s.Controls, s.Playback.Paused)
	if s.Music != nil {
		actions = append(actions, hint(s.Controls, control.Select, "Background"))
	}
	rows := controlRows(w, []controlHint{
		hint(s.Controls, control.TrackPrevious, "Previous"), hint(s.Controls, control.TrackNext, "Next"),
		hint(s.Controls, control.SeekBackward, "-10s"), hint(s.Controls, control.SeekForward, "+10s"),
	}, actions)
	// Use the bottom safe area when controls are hidden. Reserve their space
	// only while the listener has opened the controls overlay.
	progressY := bottom - 3
	if s.Playback.ControlsVisible {
		progressY = controlsTop(bottom, rows) - 10
	}
	info := p.musicInfo()
	titleY := progressY - 10
	for _, line := range info {
		titleY -= line.Bounds().Dy() + musicInfoGap
	}
	artworkBottom := titleY - musicArtworkGap
	titleY += 6
	// Only transient background feedback needs a header strip.
	labelY := sy + 10
	headerBottom := 0
	if s.MusicLabel || s.MusicMessage != "" {
		headerBottom = labelY + 12
		if s.MusicMessage != "" {
			headerBottom += 12
		}
	}
	spinning := false
	if s.Music != nil && p.visualizer != nil {
		s.MusicFrame.ArtworkBounds = image.Rect(24, sy+28, w-24, max(sy+29, artworkBottom))
		rw, rh := c.RasterSize()
		if p.background != nil {
			p.background.drawMusic(c, p.visualizer, s.Music, s.MusicIndex, s.MusicFrame, art.Backdrop, headerBottom, titleY-8, progressY+15)
		} else if rw != w || rh != h {
			if p.cache.musicCanvas == nil {
				p.cache.musicCanvas = ui.New(w, h)
			}
			clear(p.cache.musicCanvas.Pixels)
			p.visualizer.Draw(p.cache.musicCanvas, s.Music, s.MusicIndex, s.MusicFrame)
			c.CopyRows(p.cache.musicCanvas, 0, 0)
		} else {
			p.visualizer.Draw(c, s.Music, s.MusicIndex, s.MusicFrame)
		}
		spinning = s.MusicIndex >= 0 && s.Music.Config.Backgrounds[s.MusicIndex].Type == "spinning"
		if s.MusicIndex == musicviz.ArtworkBackground && art.Backdrop != nil && p.background == nil {
			p.cache.customBackground(c, art.Backdrop)
		}
		if p.background == nil {
			c.Shade(0, 0, w, headerBottom, 155)
			c.Shade(0, titleY-8, w, progressY+15-(titleY-8), 175)
		}
	}
	if s.MusicLabel && s.Music != nil {
		title := musicBackdropLabel(v.Detail.Artists, v.Detail.Album)
		if s.MusicIndex >= 0 {
			title = s.Music.Config.Backgrounds[s.MusicIndex].Name
		}
		c.Text(24, labelY, truncate(title, w-48, 1), titleColor, w-24)
	}
	if s.MusicMessage != "" {
		c.Text(24, labelY+12, truncate(s.MusicMessage, w-48, 1), dimColor, w-24)
	}
	if !spinning {
		c.Image(art.Primary, 24, sy+28, w-48, max(1, artworkBottom-(sy+28)))
	}
	y := titleY
	for _, line := range info {
		box := line.Bounds()
		c.Blit(line, (w-box.Dx())/2, y, box.Dx(), box.Dy())
		y += box.Dy() + musicInfoGap
	}
	c.Rect(24, progressY, w-48, 3, 0x303030)
	if v.Detail.RunTimeTicks > 0 {
		c.Rect(24, progressY, int(min(s.Playback.PositionTicks, v.Detail.RunTimeTicks)*int64(w-48)/v.Detail.RunTimeTicks), 3, titleColor)
	}
	if s.Playback.ControlsVisible {
		top := controlsTop(bottom, rows) - 3
		c.Shade(0, top, w, h-top, 210)
		drawControls(c, bottom, rows)
	}
	if s.Notice != "" {
		drawNotice(c, "", s.Notice, -1, 6)
	}
}

// musicBackdropLabel identifies the server artwork without exposing a preset name.
func musicBackdropLabel(artists []string, album string) string {
	if name := strings.TrimSpace(strings.Join(artists, ", ")); name != "" {
		return name
	}
	if name := strings.TrimSpace(album); name != "" {
		return name
	}
	return "Background"
}

const (
	musicInfoGap    = 4
	musicArtworkGap = 34
)

// musicInfo lays out each metadata field separately using the shared list font.
// Empty artist and album fields do not reserve rows. Timing always remains visible.
func (p *screenPainter) musicInfo() []*ui.RasterImage {
	item := p.scene.Content.Detail
	var lines []*ui.RasterImage
	for _, field := range []struct {
		text  string
		size  int
		color uint32
		bold  bool
	}{
		{item.Name, 20, titleColor, true},
		{strings.Join(item.Artists, ", "), 16, 0xffffff, false},
		{item.Album, 16, 0xffffff, false},
		{runtime(p.scene.Playback.PositionTicks) + " / " + runtime(item.RunTimeTicks), 16, 0xffffff, false},
	} {
		if line := p.listText(field.text, p.width-48, field.size, field.color, field.bold); line != nil {
			lines = append(lines, line)
		}
	}
	return lines
}
