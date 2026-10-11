package browser

import (
	"context"
	"errors"
	"testing"
	"time"

	"mistervision/internal/input/control"
	"mistervision/internal/media"
	"mistervision/internal/playback"
	"mistervision/internal/remote"
)

func postPlaySession(t *testing.T, fromContinue bool) *browserSession {
	t.Helper()
	s := testSession(t)
	first, next := episodeItem("first", 1, 1), episodeItem("next", 1, 2)
	first.RunTimeTicks = 6000000000
	parent := episodeView("one", []media.Item{first, next}, 0)
	nextParent := parent
	nextParent.Selected, nextParent.Target = 1, 1
	list := parent
	if fromContinue {
		list = View{Title: "Continue Watching", Location: media.Location{Kind: "continue"}, Page: parent.Page}
	}
	s.model.Stack = append(s.model.Stack, list, View{Title: first.Name, Detail: &first})
	s.controller.item = first
	s.postPlay = postPlay{itemID: first.ID, sequence: episodeSequence{episodes: parent, next: &mediaSelection{parent: nextParent, item: next}}}
	return s
}

func TestPostPlayOffersExplicitChoiceAndStartsUnpausedAtZero(t *testing.T) {
	s := postPlaySession(t, false)
	s.controller.pauseRequested = true
	s.handlePlayback(PlaybackEvent{Kind: PlaybackEnded, ID: s.controller.active.id})
	if s.controller.running || !s.postPlay.visible || len(s.postPlayPresentation().Rows) != 2 {
		t.Fatal("completion did not wait for a choice")
	}
	s.controller.Tick(time.Now().Add(time.Hour))
	if s.controller.running {
		t.Fatal("prompt started playback without a choice")
	}
	s.handleKey(control.Action("open-repeat"))
	if s.controller.running {
		t.Fatal("held button accepted the prompt")
	}
	s.handleKey(control.Open)
	if !s.controller.running || s.controller.item.ID != "next" || s.controller.pauseRequested || s.controller.state.PositionTicks != 0 || s.postPlay.visible {
		t.Fatal("next episode did not start cleanly")
	}
	parent, _ := s.model.Parent()
	if parent.Item().ID != "next" {
		t.Fatal("next episode missing from selected list")
	}
}

func TestPostPlayBackRestoresEpisodesWithoutNavigationLoop(t *testing.T) {
	for _, fromContinue := range []bool{false, true} {
		for _, button := range []control.Action{control.Back, control.Open} {
			s := postPlaySession(t, fromContinue)
			s.handlePlayback(PlaybackEvent{Kind: PlaybackEnded, ID: s.controller.active.id})
			s.handleKey(control.Down)
			s.handleKey(button)
			v := s.model.Current()
			if s.postPlay.visible || v.Detail != nil || v.Location.Kind != "episodes" || v.Item().ID != "first" {
				t.Fatalf("bad return: %+v", v)
			}
			s.handleKey(control.Back)
			want := "views"
			if fromContinue {
				want = "continue"
			}
			if s.model.Current().Location.Kind != want {
				t.Fatal("Back did not unwind to the originating screen")
			}
		}
	}
}

func TestPostPlayDoesNotInterceptStopsFailuresSeeksOrQueues(t *testing.T) {
	for _, mode := range []string{"stop", "failure", "stale", "seek", "queue", "playlist"} {
		s := postPlaySession(t, false)
		event := PlaybackEvent{Kind: PlaybackEnded, ID: s.controller.active.id}
		switch mode {
		case "stop":
			s.controller.stoppedByUser = true
		case "failure":
			event.Err = errors.New("failed")
		case "stale":
			event.ID++
		case "seek":
			s.controller.SeekTo(500000000, time.Now())
			s.controller.Tick(time.Now().Add(time.Second))
		case "queue":
			s.playbackQueue.active = true
		case "playlist":
			s.model.Stack[1].Location.Kind = "playlist"
			s.controller.stoppedByUser = true
		}
		s.handlePlayback(event)
		if s.postPlay.visible {
			t.Fatalf("%s opened prompt", mode)
		}
	}
}

func TestPostPlayReportingFailureStillAllowsNextEpisode(t *testing.T) {
	s := postPlaySession(t, false)
	s.handlePlayback(PlaybackEvent{Kind: PlaybackEnded, ID: s.controller.active.id, Err: playback.ErrProgress})
	if !s.postPlay.visible || s.postPlay.sequence.next == nil {
		t.Fatal("reporting warning blocked natural completion")
	}
}

func TestPostPlaySlowLookupKeepsBackSelectedAndRejectsLateResult(t *testing.T) {
	s := postPlaySession(t, false)
	resolved := s.postPlay.sequence
	s.postPlay.pending = true
	s.postPlay.sequence.next = nil
	s.handlePlayback(PlaybackEvent{Kind: PlaybackEnded, ID: s.controller.active.id})
	r := postPlayResult{generation: s.postPlay.generation, account: s.connection.generation, sequence: resolved}
	if !r.apply(s) || s.postPlay.selected != 1 {
		t.Fatal("late lookup changed Back into Play")
	}
	s.handleKey(control.Back)
	if r.apply(s) || s.postPlay.visible {
		t.Fatal("dismissed prompt reopened")
	}
}

func TestPostPlayUnavailableFinaleAndAccountChange(t *testing.T) {
	for _, mode := range []string{"finale", "error", "account", "remote stop"} {
		s := postPlaySession(t, false)
		s.postPlay.sequence.next = nil
		s.handlePlayback(PlaybackEvent{Kind: PlaybackEnded, ID: s.controller.active.id})
		switch mode {
		case "error":
			s.postPlay.err = errors.New("unavailable")
		case "account":
			s.connection.generation++
			if (postPlayResult{generation: s.postPlay.generation, account: s.connection.generation - 1}).apply(s) {
				t.Fatal("old account applied")
			}
		case "remote stop":
			s.handleRemote(remote.Command{Kind: remote.Stop})
			if s.postPlay.visible || s.model.Current().Location.Kind != "episodes" {
				t.Fatal("remote stop left prompt open")
			}
			continue
		}
		menu := s.postPlayPresentation()
		if len(menu.Rows) != 1 || menu.Rows[0] != "Back to episodes" || menu.Message == "" {
			t.Fatalf("bad terminal prompt: %+v", menu)
		}
	}
}

type blockingEpisodeServer struct {
	media.Server
	started, canceled chan struct{}
}

func (s blockingEpisodeServer) List(ctx context.Context, _ media.Location, _, _ int) (media.Page, error) {
	close(s.started)
	<-ctx.Done()
	close(s.canceled)
	return media.Page{}, ctx.Err()
}

func TestPostPlayLookupDoesNotBlockPlaybackAndCancels(t *testing.T) {
	s := postPlaySession(t, true)
	server := blockingEpisodeServer{started: make(chan struct{}), canceled: make(chan struct{})}
	s.client = server
	s.preparePostPlay(s.controller.item)
	select {
	case <-server.started:
	case <-time.After(time.Second):
		t.Fatal("lookup did not start")
	}
	if !s.controller.running || s.postPlay.visible {
		t.Fatal("lookup interrupted playback")
	}
	s.postPlay.close()
	select {
	case <-server.canceled:
	case <-time.After(time.Second):
		t.Fatal("lookup did not cancel")
	}
	select {
	case r := <-s.events:
		if r.apply(s) {
			t.Fatal("canceled lookup applied")
		}
	case <-time.After(time.Second):
		t.Fatal("worker did not exit")
	}
}

func TestPostPlayExplicitRepeatRetainsQueueOwnership(t *testing.T) {
	s := postPlaySession(t, false)
	s.handleRemote(remote.Command{Kind: remote.Repeat, Repeat: remote.RepeatOne})
	if !s.playbackQueue.active {
		t.Fatal("explicit repeat did not adopt the episode")
	}
	s.handlePlayback(PlaybackEvent{Kind: PlaybackEnded, ID: s.controller.active.id})
	if s.postPlay.visible || !s.controller.running || s.controller.item.ID != "first" {
		t.Fatal("completion prompt intercepted repeat")
	}
}

func TestPostPlayCrossSeasonBackSelectsTheNewSeason(t *testing.T) {
	s := postPlaySession(t, false)
	seasons := View{Location: media.Location{Kind: "seasons", SeriesID: "series"}, Page: media.Page{Items: []media.Item{seasonItem("one", 1), seasonItem("two", 2)}}}
	s.model.Stack = append(s.model.Stack[:1], append([]View{seasons}, s.model.Stack[1:]...)...)
	next := episodeItem("next-season", 2, 1)
	s.postPlay.sequence.next = &mediaSelection{parent: episodeView("two", []media.Item{next}, 0), item: next}
	s.handlePlayback(PlaybackEvent{Kind: PlaybackEnded, ID: s.controller.active.id})
	s.handleKey(control.Open)
	if len(s.model.Stack) != 4 || s.model.Stack[1].Item().ID != "two" || s.controller.item.ID != next.ID {
		t.Fatal("next season did not preserve the browsing path")
	}
}

func TestPostPlaySignInRecoveryDismissesPrompt(t *testing.T) {
	s := postPlaySession(t, false)
	s.handlePlayback(PlaybackEvent{Kind: PlaybackEnded, ID: s.controller.active.id})
	s.requireSignIn(media.ErrUnauthorized)
	if s.postPlay.visible || s.postPlay.itemID != "" {
		t.Fatal("completion prompt retained input ownership during sign-in recovery")
	}
}

func TestPostPlayMetadataAndLateArtworkCannotChangeCompletedItem(t *testing.T) {
	s := postPlaySession(t, false)
	s.handlePlayback(PlaybackEvent{Kind: PlaybackEnded, ID: s.controller.active.id})
	generation := s.selection.generation
	next := s.postPlay.sequence.next.item
	next.Name = "Fresh next episode metadata"
	if !s.handleSelection(selectionResult{generation: generation, update: selectionUpdate{kind: selectionDetails, detail: &next}}) {
		t.Fatal("next episode metadata was rejected")
	}
	if s.model.Current().Detail.ID != "first" || s.postPlayPresentation().Next.Name != next.Name {
		t.Fatal("preview metadata replaced the completed episode")
	}
	s.handleKey(control.Back)
	if s.handleSelection(selectionResult{generation: generation, update: selectionUpdate{kind: selectionDetails, detail: &next}}) {
		t.Fatal("dismissed preview accepted late metadata")
	}
	if s.model.Current().Item().ID != "first" {
		t.Fatal("Back no longer selects the completed episode")
	}
}
