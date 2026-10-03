package browser

import (
	"time"

	"mistervision/internal/musicviz"
	"mistervision/internal/playback"
)

// musicPresentation owns effect selection and the latest disposable audio
// measurement. Effect animation itself belongs to rendering.RasterRenderer.
type musicPresentation struct {
	library    *musicviz.Library
	index      int
	manual     bool // A user-selected background persists until the app closes.
	backdrop   bool
	labelUntil time.Time
	levels     playback.AudioLevels
	levelTime  time.Time
	loading    bool
	error      string
}

func (s *browserSession) cycleMusicBackground() {
	if s.music.library == nil {
		return
	}
	art := s.selection.current.artwork
	index := s.music.backgroundIndex(art.Backdrop != nil, art.Primary != nil, false)
	// Artwork is the slot before the configured presets. Skip missing images
	// rather than presenting an empty background or a disc without its cover.
	for range len(s.music.library.Config.Backgrounds) + 1 {
		index++
		if index == len(s.music.library.Config.Backgrounds) {
			index = musicviz.ArtworkBackground
		}
		if index == musicviz.ArtworkBackground {
			if art.Backdrop == nil {
				continue
			}
		} else if s.music.library.Config.Backgrounds[index].Type == "spinning" && art.Primary == nil {
			continue
		}
		s.music.backdrop = index == musicviz.ArtworkBackground
		if index >= 0 {
			s.music.index = index
		}
		s.music.manual = true
		break
	}
	s.music.labelUntil = time.Now().Add(1500 * time.Millisecond)
	s.music.error = ""
	s.loadMusicAssets()
}

// backgroundIndex preserves the listener's choice across playback sessions.
// Artwork-dependent choices fall back only while their required image is absent.
func (m *musicPresentation) backgroundIndex(backdrop, cover, pending bool) int {
	if m.manual && !m.backdrop && m.library != nil && m.library.Config.Backgrounds[m.index].Type != "spinning" {
		return m.index
	}
	if m.manual && !m.backdrop && cover {
		return m.index
	}
	if backdrop || pending {
		return musicviz.ArtworkBackground
	}
	if m.library == nil {
		return m.index
	}
	index := m.library.Index(m.library.Config.Default)
	if m.library.Config.Backgrounds[index].Type != "spinning" || cover {
		return index
	}
	for i, preset := range m.library.Config.Backgrounds {
		if preset.Type != "spinning" {
			return i
		}
	}
	// A configuration containing only spinning effects has no usable effect.
	return musicviz.ArtworkBackground
}

func (s *browserSession) loadMusicAssets() {
	if s.music.library == nil || s.music.loading || s.music.library.Ready(s.music.index) {
		return
	}
	library, index := s.music.library, s.music.index
	s.music.loading = true
	go func() {
		loaded, err := library.LoadAssets(index)
		s.send(s.ctx, musicAssetsResult{music: loaded, index: index, err: err})
	}()
}

func (s *browserSession) handleMusicAssets(r musicAssetsResult) bool {
	s.music.loading = false
	if r.err != nil {
		s.config.Diagnostics.ConfigurationFallback("music_visuals", "selected-background-unavailable", r.err)
	}
	if r.err == nil {
		s.music.library = r.music
	} else if r.index == s.music.index {
		s.music.error = messageMusicBackgroundFailed
	}
	if r.index != s.music.index {
		s.loadMusicAssets()
	}
	return true
}
