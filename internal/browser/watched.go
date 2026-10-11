package browser

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"mistervision/internal/diagnostics"
	"mistervision/internal/media"
)

// watchedChange owns a preview action and its inline feedback. A pending change
// finishes before playback can resume, so progress reports cannot overwrite it.
type watchedChange struct {
	busy            bool
	generation      int
	itemID, message string
	cancel          context.CancelFunc
}

func (w *watchedChange) close() {
	if w.cancel != nil {
		w.cancel()
	}
	*w = watchedChange{generation: w.generation + 1}
}

func (s *browserSession) changeWatched() {
	client, ok := s.client.(media.WatchedState)
	item := s.model.Current().Detail
	if !ok || item == nil {
		return
	}
	id, watched := item.ID, !item.UserData.Played
	s.watched.close()
	work, cancel := context.WithTimeout(s.ctx, 15*time.Second)
	w := &s.watched
	w.busy, w.itemID, w.message, w.cancel = true, id, messageWatchedSaving, cancel
	generation, connection := w.generation, s.connection.generation
	go func() {
		err := client.SetWatched(work, id, watched)
		s.send(s.ctx, watchedResult{generation: generation, connection: connection, id: id, watched: watched, err: err})
	}()
}

type watchedResult struct {
	generation, connection int
	id                     string
	watched                bool
	err                    error
}

func (r watchedResult) apply(s *browserSession) bool {
	if !s.watched.busy || r.generation != s.watched.generation || r.connection != s.connection.generation {
		return false
	}
	s.watched.busy = false
	s.watched.cancel()
	s.config.Diagnostics.Record("browser.watched", slog.Bool("watched", r.watched), slog.Bool("failed", r.err != nil), slog.String("error_kind", diagnostics.ErrorKind(r.err)))
	if r.err != nil {
		s.watched.message = requestFailure(messageWatchedFailed, r.err)
		if errors.Is(r.err, media.ErrUnauthorized) {
			s.requireSignIn(r.err)
		}
		return true
	}
	// Invalidate older metadata/page requests before updating retained views.
	// The next visit still fetches authoritative data from the server.
	s.requests.cancel()
	s.model.Generation++
	s.selection.cancel()
	s.selection.generation++
	update := func(item *media.Item) {
		if item != nil && item.ID == r.id {
			item.UserData.Played = r.watched
			item.UserData.PlaybackPositionTicks = 0
		}
	}
	for i := range s.model.Stack {
		v := &s.model.Stack[i]
		update(v.Detail)
		for j := range v.Page.Items {
			update(&v.Page.Items[j])
		}
		v.fetching = false
	}
	// The controller's stopped-position shortcut must not restore a resume
	// position that the user explicitly cleared after cleanup.
	if s.controller.item.ID == r.id {
		s.controller.state.PositionTicks = 0
	}
	text := messageMarkedWatched
	if !r.watched {
		text = messageMarkedUnwatched
	}
	s.watched.message = text
	s.refreshHome()
	return true
}
