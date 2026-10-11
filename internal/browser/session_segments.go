package browser

import (
	"context"
	"log/slog"
	"time"

	"mistervision/internal/diagnostics"
	"mistervision/internal/media"
)

// segmentRequest lives for one item, independently of its decoder replacements.
// Canceling invalidates even a result already queued on the event loop.
type segmentRequest struct {
	cancel     context.CancelFunc
	generation int
}

func (r *segmentRequest) close() {
	if r.cancel != nil {
		r.cancel()
		r.cancel = nil
	}
	r.generation++
}

// loadSegments runs alongside playback preparation. Unavailable metadata never
// delays or interrupts playback. Seeks and audio/subtitle changes reuse the result.
func (s *browserSession) loadSegments(item media.Item) {
	s.segments.close()
	source, ok := s.client.(media.SegmentSource)
	if !ok || item.Type == "Audio" || media.IsLive(item) {
		return
	}
	work, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	s.segments.cancel = cancel
	generation, connection := s.segments.generation, s.connection.generation
	go func() {
		defer cancel()
		segments, err := source.Segments(work, item.ID)
		s.send(s.ctx, segmentsResult{generation: generation, connection: connection, segments: segments, err: err})
	}()
}

type segmentsResult struct {
	generation, connection int
	segments               []media.Segment
	err                    error
}

func (r segmentsResult) apply(s *browserSession) bool {
	if r.generation != s.segments.generation || r.connection != s.connection.generation || !s.controller.running || s.controller.stoppedByUser {
		return false
	}
	s.config.Diagnostics.Record("browser.segments", slog.Int("count", len(r.segments)), slog.String("error_kind", diagnostics.ErrorKind(r.err)))
	if r.err != nil {
		return false
	}
	s.controller.setSegments(r.segments, time.Now())
	return true
}
