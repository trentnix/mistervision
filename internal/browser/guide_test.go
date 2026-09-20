package browser

import (
	"context"
	"errors"
	"testing"
	"time"

	"mistervision/internal/media"
)

type testGuideServer struct {
	media.Server
	calls   chan context.Context
	release chan struct{}
}

func (g testGuideServer) Programs(ctx context.Context, ids []string, start, end time.Time) ([]media.Program, error) {
	g.calls <- ctx
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-g.release:
	}
	return []media.Program{{ChannelID: ids[0], Title: "Now", Start: start.Add(-time.Hour), End: end}}, nil
}
func TestGuideDoesNotBlockNavigationAndRejectsLateResults(t *testing.T) {
	s := testSession(t)
	s.controller.running = false
	g := testGuideServer{calls: make(chan context.Context, 2), release: make(chan struct{})}
	s.client = g
	s.model.Stack = append(s.model.Stack, View{Location: media.Location{Kind: "livetv"}, Page: media.Page{Items: []media.Item{{ID: "a", Type: "TvChannel"}}}})
	now := time.Now()
	s.refreshGuide(now)
	t.Cleanup(s.guide.reset)
	generation := s.guide.generation
	var ctx context.Context
	select {
	case ctx = <-g.calls:
	case <-time.After(time.Second):
		t.Fatal("guide not requested")
	}
	s.refreshGuide(now.Add(time.Second))
	if generation != s.guide.generation {
		t.Fatal("same page restarted guide")
	}
	s.model.Stack = s.model.Stack[:1]
	s.refreshGuide(now.Add(2 * time.Second))
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("navigation did not cancel guide")
	}
	if (guideResult{generation: generation, programs: []media.Program{{ChannelID: "a"}}}).apply(s) {
		t.Fatal("late guide changed another page")
	}
	if s.guide.programs != nil {
		t.Fatal("guide leaked into next page")
	}
}
func TestGuideResultsRejectOtherAccountAndRetainDataOnFailure(t *testing.T) {
	s := testSession(t)
	now := time.Now()
	result := guideResult{programs: []media.Program{{ChannelID: "a", Title: "Now", Start: now, End: now.Add(time.Hour)}}}
	if !result.apply(s) {
		t.Fatal("current result rejected")
	}
	if (guideResult{err: errors.New("offline")}).apply(s) || len(s.guide.programs["a"]) != 1 {
		t.Fatal("optional failure discarded usable data")
	}
	s.connection.generation++
	if result.apply(s) {
		t.Fatal("old account guide accepted")
	}
}

func TestGuideReturnShowsCacheAndAlwaysRefreshes(t *testing.T) {
	for _, playback := range []bool{false, true} {
		name := "leave list"
		if playback {
			name = "play channel"
		}
		t.Run(name, func(t *testing.T) {
			s := testSession(t)
			s.controller.running = false
			g := testGuideServer{calls: make(chan context.Context, 2), release: make(chan struct{})}
			s.client = g
			live := View{Location: media.Location{Kind: "livetv"}, Page: media.Page{Items: []media.Item{{ID: "a", Type: "TvChannel"}}}}
			s.model.Stack = append(s.model.Stack, live)
			now := time.Now()
			s.guide = guideState{
				active: true, channels: []string{"a"}, refresh: now.Add(time.Minute),
				programs: map[string][]media.Program{"a": {{ChannelID: "a", Title: "Old listing", Start: now.Add(-time.Minute), End: now.Add(time.Minute)}}},
			}
			t.Cleanup(s.guide.reset)
			if playback {
				s.controller.running = true
			} else {
				s.model.Stack = s.model.Stack[:1]
			}
			s.refreshGuide(now)
			if s.guide.active || s.guide.programs["a"][0].Title != "Old listing" {
				t.Fatal("leaving did not retain an inactive cache")
			}
			if playback {
				s.controller.running = false
			} else {
				s.model.Stack = append(s.model.Stack, live)
			}
			s.refreshGuide(now.Add(time.Second))
			if !s.guide.active || s.guide.programs["a"][0].Title != "Old listing" {
				t.Fatal("return did not immediately expose cached listings")
			}
			select {
			case <-g.calls:
			case <-time.After(time.Second):
				t.Fatal("return did not request fresh listings within the normal refresh interval")
			}
			close(g.release)
			select {
			case result := <-s.events:
				if !result.apply(s) || s.guide.programs["a"][0].Title != "Now" {
					t.Fatal("fresh listing did not replace cached listing")
				}
			case <-time.After(time.Second):
				t.Fatal("guide result missing")
			}
			if !(guideResult{generation: s.guide.generation}).apply(s) || len(s.guide.programs) != 0 {
				t.Fatal("successful empty response did not remove cached listings")
			}
		})
	}
}

func TestGuideConnectionChangeDiscardsCacheWhileHidden(t *testing.T) {
	s := testSession(t)
	s.guide.programs = map[string][]media.Program{"a": {{Title: "Previous account"}}}
	s.connection.generation++
	s.refreshGuide(time.Now())
	if s.guide.programs != nil {
		t.Fatal("connection change retained the previous account's guide")
	}
}

func TestGuidePageChangeRetainsOverlapAndRejectsOldRequest(t *testing.T) {
	s := testSession(t)
	s.controller.running = false
	s.client = testGuideServer{calls: make(chan context.Context, 2), release: make(chan struct{})}
	s.model.Stack = append(s.model.Stack, View{Location: media.Location{Kind: "livetv"}, Page: media.Page{Items: []media.Item{{ID: "a", Type: "TvChannel"}}}})
	now := time.Now()
	s.refreshGuide(now)
	t.Cleanup(s.guide.reset)
	oldChannels, generation := s.guide.channels, s.guide.generation
	s.guide.programs = map[string][]media.Program{"a": {{Title: "Cached"}}}
	s.model.Current().Page.Items = append(s.model.Current().Page.Items, media.Item{ID: "b", Type: "TvChannel"})
	s.refreshGuide(now.Add(time.Second))
	if len(oldChannels) != 1 || oldChannels[0] != "a" {
		t.Fatal("page change mutated the worker's channel snapshot")
	}
	if len(s.guide.channels) != 2 || s.guide.programs["a"][0].Title != "Cached" {
		t.Fatal("prefetch discarded listings for channels still on the page")
	}
	if (guideResult{generation: generation}).apply(s) {
		t.Fatal("previous page's response replaced the new guide")
	}
	s.model.Current().Page.Items = s.model.Current().Page.Items[1:]
	s.refreshGuide(now.Add(2 * time.Second))
	if len(s.guide.programs) != 0 {
		t.Fatal("guide retained channels outside the current page")
	}
}

func TestGuideUnchangedFrameDoesNotAllocate(t *testing.T) {
	s := testSession(t)
	s.controller.running = false
	s.client = testGuideServer{}
	s.model.Stack = append(s.model.Stack, View{Location: media.Location{Kind: "livetv"}, Page: media.Page{Items: []media.Item{{ID: "a", Type: "TvChannel"}}}})
	now := time.Now()
	s.guide = guideState{active: true, channels: []string{"a"}, refresh: now.Add(time.Minute)}
	if allocations := testing.AllocsPerRun(100, func() { s.refreshGuide(now) }); allocations != 0 {
		t.Fatalf("unchanged frame allocated %g times", allocations)
	}
}
