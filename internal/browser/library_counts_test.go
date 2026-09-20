package browser

import (
	"context"
	"errors"
	"testing"
	"time"

	"mistervision/internal/input/control"
	"mistervision/internal/media"
	"mistervision/internal/rendering"
)

type countCall struct {
	id    string
	ctx   context.Context
	reply chan libraryCountResult
}
type countServer struct {
	media.Server
	calls chan countCall
}

func (s countServer) LibraryCount(ctx context.Context, item media.Item) (*int, error) {
	call := countCall{id: item.ID, ctx: ctx, reply: make(chan libraryCountResult, 1)}
	s.calls <- call
	select {
	case r := <-call.reply:
		return r.count, r.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
func countSession(t *testing.T, ids ...string) (*browserSession, chan countCall) {
	t.Helper()
	s := testSession(t)
	s.ctx = t.Context()
	calls := make(chan countCall, 16)
	server := countServer{calls: calls}
	s.client = server
	s.selection.loader = newSelectionLoader(server, 640, 240, selectionCaches{})
	s.selection.loader.customBackground = true
	for _, id := range ids {
		s.model.Current().Page.Items = append(s.model.Current().Page.Items, media.Item{ID: id})
	}
	t.Cleanup(s.counts.reset)
	return s, calls
}
func nextCount(t *testing.T, calls <-chan countCall) countCall {
	t.Helper()
	select {
	case c := <-calls:
		return c
	case <-time.After(time.Second):
		t.Fatal("count request did not start")
		return countCall{}
	}
}
func applyCount(t *testing.T, s *browserSession) {
	t.Helper()
	select {
	case r := <-s.events:
		r.apply(s)
	case <-time.After(time.Second):
		t.Fatal("count result did not arrive")
	}
}

func TestHomeCountsShareWorkAcrossSelectionAndViewChanges(t *testing.T) {
	s, calls := countSession(t, "a", "b", "c", "d", "e", "f")
	v := s.model.Current()
	v.Selected = 4
	v.Scroll = 3
	s.model.Rows = 2
	original := v.Page.Items
	s.loadSelection()
	active := map[string]countCall{}
	for range libraryCountConcurrency {
		c := nextCount(t, calls)
		active[c.id] = c
	}
	if _, ok := active["e"]; !ok {
		t.Fatal("selected library was not prioritized")
	}
	if _, ok := active["d"]; !ok {
		t.Fatal("visible row was not prioritized")
	}
	if len(s.counts.pending) != 3 {
		t.Fatal("wrong concurrency")
	}
	s.model.Current().Selected = 5
	s.model.Current().Scroll = 5
	s.model.Key(control.Select)
	s.loadSelection()
	select {
	case <-calls:
		t.Fatal("view change started duplicate work")
	default:
	}
	for _, c := range active {
		if c.ctx.Err() != nil {
			t.Fatal("selection canceled count work")
		}
	}
	count := 12
	active["e"].reply <- libraryCountResult{count: &count}
	applyCount(t, s)
	next := nextCount(t, calls)
	if next.id != "f" {
		t.Fatalf("next request=%s, want newly selected library f", next.id)
	}
	if original[4].LibraryCount != nil || *s.model.Current().Page.Items[4].LibraryCount != 12 {
		t.Fatal("count changed borrowed data or failed to populate home")
	}
	next.reply <- libraryCountResult{count: &count}
	applyCount(t, s)
	for _, list := range []bool{false, true} {
		s.model.ListMode = list
		scene := sceneFromModel(s.model, rendering.PlaybackPresentation{}, s.setup, s.selection.current, "", time.Now())
		if scene.LibraryCount == nil || *scene.LibraryCount != 12 {
			t.Fatal("home views do not share selected count")
		}
	}
}

func TestHomeCountsReuseCacheAndBackOffFailures(t *testing.T) {
	s, calls := countSession(t, "cached", "broken", "empty", continueID)
	count := 12
	s.selection.loader.libraries.remember("cached", func(c *cachedLibrary) { c.count = &count; c.countUntil = time.Now().Add(time.Minute) })
	s.loadLibraryCounts()
	active := map[string]countCall{}
	for range 2 {
		c := nextCount(t, calls)
		active[c.id] = c
	}
	if _, ok := active["cached"]; ok {
		t.Fatal("fresh cached count was fetched")
	}
	if *s.model.Current().Page.Items[0].LibraryCount != 12 {
		t.Fatal("cache was not published")
	}
	active["broken"].reply <- libraryCountResult{err: errors.New("offline")}
	zero := 0
	active["empty"].reply <- libraryCountResult{count: &zero}
	applyCount(t, s)
	applyCount(t, s)
	s.loadLibraryCounts()
	select {
	case c := <-calls:
		t.Fatalf("unexpected retry or synthetic request: %s", c.id)
	default:
	}
	if s.model.Current().Page.Items[1].LibraryCount != nil || *s.model.Current().Page.Items[2].LibraryCount != 0 {
		t.Fatal("unknown and empty counts were confused")
	}
	delete(s.counts.retryAfter, "broken")
	s.loadLibraryCounts()
	if nextCount(t, calls).id != "broken" {
		t.Fatal("explicit retry did not run")
	}
	s.selection.loader.libraries.remember("cached", func(c *cachedLibrary) { c.countUntil = time.Time{} })
	s.loadLibraryCounts()
	if nextCount(t, calls).id != "cached" {
		t.Fatal("expired count did not refresh")
	}
}

func TestHomeCountsCancelAndRejectResultsAfterAccountChange(t *testing.T) {
	s, calls := countSession(t, "library")
	s.loadLibraryCounts()
	call := nextCount(t, calls)
	old := s.counts.generation
	s.counts.reset()
	select {
	case <-call.ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("account change did not cancel work")
	}
	count := 99
	if (libraryCountResult{generation: old, id: "library", count: &count}).apply(s) || s.model.Current().Item().LibraryCount != nil {
		t.Fatal("old account count was accepted")
	}
}
