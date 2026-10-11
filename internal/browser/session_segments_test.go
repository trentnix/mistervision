package browser

import (
	"context"
	"errors"
	"testing"
	"time"

	"mistervision/internal/media"
)

type segmentServer struct {
	media.Server
	fetch func(context.Context, string) ([]media.Segment, error)
}

func (s segmentServer) Segments(ctx context.Context, id string) ([]media.Segment, error) {
	return s.fetch(ctx, id)
}

func TestSegmentsLoadIndependentlyAndCancel(t *testing.T) {
	s := testSession(t)
	started, stopped := make(chan struct{}), make(chan struct{})
	s.client = segmentServer{fetch: func(ctx context.Context, id string) ([]media.Segment, error) {
		close(started)
		<-ctx.Done()
		close(stopped)
		return nil, ctx.Err()
	}}
	s.loadSegments(s.controller.item)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("no segment request")
	}
	// Playback remains usable while the optional metadata request is blocked.
	if !s.controller.running || !s.controller.state.ProgressSeen {
		t.Fatal("metadata blocked playback")
	}
	s.segments.close()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("metadata request not canceled")
	}
	select {
	case r := <-s.events:
		if r.apply(s) {
			t.Fatal("canceled request applied")
		}
	case <-time.After(time.Second):
		t.Fatal("no completed worker")
	}
}

func TestSegmentResultRejectsOldItemAccountAndErrors(t *testing.T) {
	for _, mode := range []string{"success", "item", "account", "stopped", "error"} {
		s := testSession(t)
		r := segmentsResult{segments: []media.Segment{{Kind: media.SegmentIntro, EndTicks: 100000000}}}
		switch mode {
		case "item":
			s.segments.close()
		case "account":
			s.connection.generation++
		case "stopped":
			s.controller.stoppedByUser = true
		case "error":
			r.err = errors.New("unavailable")
		}
		if got := r.apply(s); got != (mode == "success") {
			t.Fatalf("%s: applied=%v", mode, got)
		}
		if (len(s.controller.segments) > 0) != (mode == "success") {
			t.Fatalf("%s changed segments", mode)
		}
	}
}
