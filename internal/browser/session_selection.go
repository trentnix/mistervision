package browser

import (
	"context"
	"errors"
	"log/slog"

	"mistervision/internal/media"
)

// selectionState owns the selected metadata, images, and request lifetime.
type selectionState struct {
	cancel          context.CancelFunc
	generation      int
	key, err        string
	current         selectionData
	loader          *selectionLoader
	backdropPending bool // Audio detail backdrop has not returned, including a missing-image result.
}

// loadSelection follows the selected item and view type. An unchanged selection
// reuses current work. A new selection publishes cached metadata and images immediately
// and rejects later updates from the previous selection by generation.
func (s *browserSession) loadSelection() {
	s.loadLibraryCounts()
	item := s.model.Current().Item()
	key := ""
	root := len(s.model.Stack) == 1
	detail := s.model.Current().Detail != nil
	if item != nil {
		key = item.ID
		if root {
			key = "root:" + key
		}
		if detail {
			key = "detail:" + key
		}
	}
	if key == s.selection.key {
		return
	}
	s.selection.key = key
	s.selection.cancel()
	s.selection.generation++
	s.selection.current = selectionData{}
	s.selection.backdropPending = false
	s.selection.err = ""
	if item == nil || s.client == nil {
		return
	}
	if item.ID == continueID {
		s.seedHomeArtwork()
	}
	s.selection.current = s.selection.loader.snapshot(*item, root)
	s.selection.backdropPending = !root && detail && item.Type == "Audio" && s.selection.current.artwork.Backdrop == nil
	if item.ID == continueID && s.home.err != nil {
		s.selection.err = messageContinueIncomplete
	}
	selected := *item
	generation := s.selection.generation
	work, stop := context.WithCancel(s.ctx)
	s.selection.cancel = stop
	loader := s.selection.loader
	go loader.load(work, selected, root, detail, func(update selectionUpdate) {
		s.send(work, selectionResult{generation: generation, update: update})
	})
}

func (s *browserSession) handleSelection(r selectionResult) bool {
	if r.generation != s.selection.generation {
		return false
	}
	if errors.Is(r.update.err, media.ErrUnauthorized) {
		s.requireSignIn(r.update.err)
		return true
	}
	backdropResolved := r.update.kind == selectionArtwork && r.update.art.kind == "Backdrop" && s.selection.backdropPending
	if backdropResolved {
		s.selection.backdropPending = false
	}
	if r.update.err != nil && r.update.kind == selectionArtwork {
		// Covers and backdrops are optional. A carousel can request the same
		// missing image as a child screen, so surfacing every failure makes the
		// error appear to follow navigation. Keep diagnostics without a banner.
		s.config.Diagnostics.Record("browser.artwork", slog.String("kind", r.update.art.kind), slog.Bool("failed", true))
		if r.update.art.kind != "Photo" {
			return backdropResolved
		}
	}
	if r.update.err != nil {
		switch r.update.kind {
		case selectionDetails:
			s.selection.err = messageDetailsFailed
		default:
			s.selection.err = messagePhotoFailed
		}
	} else {
		switch r.update.kind {
		case selectionArtwork:
			applyArtwork(&s.selection.current.artwork, r.update.art)
		case selectionDetails:
			if s.model.Current().Detail != nil {
				s.model.Current().Detail = r.update.detail
			}
		}
	}

	return true
}
