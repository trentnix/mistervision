package browser

import (
	"errors"
	"log/slog"
	"mistervision/internal/diagnostics"

	"mistervision/internal/media"
)

// changeMusicOption adopts a complete local album without restarting playback.
// Paged albums and playlists resolve in a worker before changing queue ownership.
func (s *browserSession) changeMusicOption() {
	s.remoteRequests.cancelLocal()
	if s.playbackQueue.active {
		s.applyMusicOption()
		return
	}
	parent, ok := s.model.Parent()
	if !ok || parent.Start == 0 && !parent.More() {
		s.adoptLocalQueue()
		if s.playbackQueue.active {
			s.applyMusicOption()
		}
		return
	}
	work, generation, connection := s.optionsRequest(messageQueueLoading)
	client := s.client
	itemID, row := s.options.itemID, parent.Start+parent.Selected
	go func() {
		items, err := client.AudioQueue(work, parent.Location)
		// Queue indices represent occurrences, so duplicate tracks retain the
		// selected occurrence. Do not silently choose the first matching ID.
		if err == nil && (row < 0 || row >= len(items) || items[row].ID != itemID) {
			err = errors.New("selected track no longer matches the playback queue")
		}
		s.send(s.ctx, musicOptionsResult{generation: generation, connection: connection, items: items, row: row, err: err})
	}()
}

type musicOptionsResult struct {
	generation, connection, row int
	items                       []media.Item
	err                         error
}

func (r musicOptionsResult) apply(s *browserSession) bool {
	if !s.options.visible || r.generation != s.options.generation || r.connection != s.connection.generation {
		return false
	}
	if !s.controller.running || s.controller.stoppedByUser || s.controller.item.ID != s.options.itemID || s.media.pending || s.playbackQueue.switching {
		s.options.close()
		return true
	}
	s.config.Diagnostics.Record("browser.music-options", slog.Bool("failed", r.err != nil), slog.String("error_kind", diagnostics.ErrorKind(r.err)))
	s.options.busy = false
	s.options.cancel()
	if r.err != nil {
		s.options.message = requestFailure(messageQueueOptionsFailed, r.err)
		if errors.Is(r.err, media.ErrUnauthorized) {
			s.requireSignIn(r.err)
		}
		return true
	}
	// The menu is dismissed by item completion or replacement, so adoption here
	// still refers to the same audible track and never starts another decoder.
	s.adoptLocalQueue()
	q := &s.playbackQueue
	q.replace(r.items, r.row)
	q.localRows = make(map[string]int, len(r.items))
	for i, entry := range q.queue.Snapshot().Entries {
		q.localRows[entry.Key] = i
	}
	s.applyMusicOption()
	return true
}
