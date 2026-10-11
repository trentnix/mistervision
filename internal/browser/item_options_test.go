package browser

import (
	"context"
	"errors"
	"testing"
	"time"

	"mistervision/internal/input/control"
	"mistervision/internal/media"
	"mistervision/internal/musicviz"
	"mistervision/internal/remote"
)

type optionsServer struct {
	media.Server
	set    func(context.Context, string, bool) error
	tracks []media.Item
}

func (c optionsServer) SetWatched(ctx context.Context, id string, watched bool) error {
	return c.set(ctx, id, watched)
}
func (c optionsServer) AudioQueue(context.Context, media.Location) ([]media.Item, error) {
	return c.tracks, nil
}
func (c optionsServer) ContinueWatching(context.Context) (media.Page, error) {
	return media.Page{}, nil
}

func receiveOptionsResult(t *testing.T, s *browserSession) workerResult {
	t.Helper()
	select {
	case r := <-s.events:
		return r
	case <-time.After(time.Second):
		t.Fatal("options request did not complete")
		return nil
	}
}

func TestMusicOptionsPreservePlayingTrack(t *testing.T) {
	s := setupMusicSession(t)
	id := s.controller.active.id
	s.dispatchKey(control.Select)
	if !s.options.visible {
		t.Fatal("options did not open")
	}
	for _, want := range []remote.RepeatMode{remote.RepeatAll, remote.RepeatOne, remote.RepeatNone} {
		s.dispatchKey(control.Open)
		if s.playbackQueue.queue.Repeat() != want {
			t.Fatal("wrong repeat mode", s.playbackQueue.queue.Repeat())
		}
	}
	s.dispatchKey(control.Down)
	s.dispatchKey(control.Open)
	if !s.playbackQueue.queue.Shuffled() || s.playbackQueue.queue.Current().ID != "first" || s.controller.active.id != id {
		t.Fatal("shuffle disturbed playing item")
	}
	s.dispatchKey(control.Open)
	if s.playbackQueue.queue.Shuffled() || s.controller.active.id != id {
		t.Fatal("unshuffle disturbed playback")
	}
	s.dispatchKey(control.Back)
	if s.options.visible || !s.controller.running {
		t.Fatal("Back stopped music instead of dismissing menu")
	}
}

func TestMusicOptionsPagedQueueKeepsDuplicateOccurrence(t *testing.T) {
	s := setupMusicSession(t)
	tracks := []media.Item{{ID: "first", Type: "Audio"}, {ID: "other", Type: "Audio"}, {ID: "first", Type: "Audio"}, {ID: "last", Type: "Audio"}}
	s.client = optionsServer{tracks: tracks}
	parent := &s.model.Stack[len(s.model.Stack)-2]
	total := 4
	parent.Start = 2
	parent.Selected = 0
	parent.Page = media.Page{Items: tracks[2:], TotalRecordCount: &total}
	s.dispatchKey(control.Select)
	s.dispatchKey(control.Open)
	if !s.options.busy {
		t.Fatal("paged queue did not load asynchronously")
	}
	receiveOptionsResult(t, s).apply(s)
	if s.options.busy || !s.playbackQueue.active || s.playbackQueue.queue.Len() != 4 || s.playbackQueue.queue.Repeat() != remote.RepeatAll {
		t.Fatal("full queue not adopted")
	}
	if got := s.playbackQueue.localRows[s.playbackQueue.queue.Current().Key]; got != 2 {
		t.Fatalf("selected duplicate occurrence %d instead of 2", got)
	}
}

func TestMusicOptionsIgnoreCanceledQueueResult(t *testing.T) {
	s := setupMusicSession(t)
	s.dispatchKey(control.Select)
	r := musicOptionsResult{generation: s.options.generation, connection: s.connection.generation, items: []media.Item{{ID: "first", Type: "Audio"}}}
	s.dispatchKey(control.Back)
	if r.apply(s) || s.playbackQueue.active {
		t.Fatal("closed menu adopted stale queue")
	}
}

func TestMusicOptionsHeldActionsDoNotToggle(t *testing.T) {
	s := setupMusicSession(t)
	s.dispatchKey(control.Select)
	for _, key := range []control.Action{control.Select, control.Open, control.Back} {
		s.dispatchKey(key.Repeat())
	}
	if !s.options.visible || s.playbackQueue.active {
		t.Fatal("held buttons changed menu state")
	}
}

func watchedSession(t *testing.T, err error) *browserSession {
	t.Helper()
	s := testSession(t)
	s.controller.running = false
	s.controller.active.id = 0
	item := media.Item{ID: "movie", Type: "Movie"}
	item.UserData.PlaybackPositionTicks = 123
	s.model.Current().Page.Items = []media.Item{item}
	s.model.Stack = append(s.model.Stack, View{Detail: &item})
	s.client = optionsServer{set: func(ctx context.Context, id string, watched bool) error {
		if id != "movie" || !watched {
			t.Errorf("wrong watched change: %s %v", id, watched)
		}
		return err
	}}
	return s
}

func TestWatchedChangeUpdatesRetainedViews(t *testing.T) {
	s := watchedSession(t, nil)
	s.dispatchKey(control.Options)
	s.dispatchKey(control.Open)
	if s.model.Current().Detail.UserData.Played {
		t.Fatal("optimistically marked watched")
	}
	oldGeneration := s.selection.generation
	receiveOptionsResult(t, s).apply(s)
	for _, item := range []media.Item{*s.model.Current().Detail, s.model.Stack[0].Page.Items[0]} {
		if !item.UserData.Played || item.UserData.PlaybackPositionTicks != 0 {
			t.Fatal("stale retained metadata", item.UserData)
		}
	}
	if s.options.visible || s.selection.generation == oldGeneration || s.watched.message != messageMarkedWatched || s.message.Text != "" || !s.home.loading {
		t.Fatal("missing completion feedback, invalidation, or Continue refresh")
	}
}

func TestWatchedFailureAndStaleResults(t *testing.T) {
	s := watchedSession(t, errors.New("private server error"))
	s.dispatchKey(control.Options)
	s.dispatchKey(control.Open)
	receiveOptionsResult(t, s).apply(s)
	if s.options.visible || s.watched.busy || s.watched.message != messageWatchedFailed+" "+messageRetry || s.model.Current().Detail.UserData.Played {
		t.Fatal("failed action changed state or leaked error", s.options)
	}
	s.dispatchKey(control.Options) // Retry before the account changes.
	r := watchedResult{generation: s.watched.generation, connection: s.connection.generation, id: "movie", watched: true}
	s.connection.generation++
	if r.apply(s) || s.model.Current().Detail.UserData.Played {
		t.Fatal("accepted previous user's result")
	}
}

func TestWatchedOptionsWaitForPlaybackCleanup(t *testing.T) {
	s := watchedSession(t, nil)
	s.controller.item = *s.model.Current().Detail
	s.controller.active.id = 1
	s.controller.cleanupComplete = false
	if s.optionsAvailable() {
		t.Fatal("offered mutation while final progress report is pending")
	}
	s.controller.cleanupComplete = true
	if !s.optionsAvailable() {
		t.Fatal("options unavailable after cleanup")
	}
}

func TestWatchedOptionsDoNotLeakOntoOtherScreens(t *testing.T) {
	for _, kind := range []string{"Audio", "Photo", "TvChannel", "Series", "Season"} {
		s := watchedSession(t, nil)
		s.model.Current().Detail.Type = kind
		if s.optionsAvailable() {
			t.Fatalf("watched action offered for %s", kind)
		}
	}
}

func TestPendingMusicOptionsCanBeDismissed(t *testing.T) {
	s := setupMusicSession(t)
	s.dispatchKey(control.Select)
	canceled := false
	s.options.busy = true
	s.options.cancel = func() { canceled = true }
	s.dispatchKey(control.Back)
	if !canceled || s.options.visible || !s.controller.running {
		t.Fatal("cancel stopped playback or left the lookup active")
	}
}

func TestWatchedMutationBlocksPlaybackUntilComplete(t *testing.T) {
	s := watchedSession(t, nil)
	s.dispatchKey(control.Options)
	id := s.controller.active.id
	s.dispatchKey(control.Open)
	if s.handleRemote(remote.Command{Kind: remote.Play}) || s.controller.active.id != id {
		t.Fatal("playback raced watched mutation")
	}
}

func TestOptionsCloseOnLostAuthorization(t *testing.T) {
	s := watchedSession(t, nil)
	s.dispatchKey(control.Options)
	s.requireSignIn(media.ErrUnauthorized)
	if s.watched.busy || s.options.visible {
		t.Fatal("pending action survived sign-in recovery")
	}
}

func TestAccountChangeClearsRepeatMode(t *testing.T) {
	s := setupMusicSession(t)
	s.adoptLocalQueue()
	s.playbackQueue.queue.SetRepeat(remote.RepeatAll)
	s.stopRemote()
	if s.playbackQueue.queue.Repeat() != remote.RepeatNone {
		t.Fatal("repeat leaked across accounts")
	}
}

func TestMusicRepeatOneAdvancesOnlyOnExplicitNext(t *testing.T) {
	s := setupMusicSession(t)
	s.dispatchKey(control.Select)
	s.dispatchKey(control.Open)
	s.dispatchKey(control.Open)
	old := s.controller.active.id
	s.handlePlayback(PlaybackEvent{Kind: PlaybackEnded, ID: old})
	if s.controller.active.id == old || s.controller.item.ID != "first" || s.playbackQueue.queue.Repeat() != remote.RepeatOne {
		t.Fatal("natural completion did not repeat current track")
	}
	s.dispatchKey(control.TrackNext)
	s.handlePlayback(PlaybackEvent{Kind: PlaybackEnded, ID: s.controller.active.id})
	if s.controller.item.ID != "second" {
		t.Fatal("Repeat One prevented explicit Next")
	}
}

func TestMarkUnwatchedClearsResume(t *testing.T) {
	s := watchedSession(t, nil)
	s.model.Current().Detail.UserData.Played = true
	s.client = optionsServer{set: func(_ context.Context, id string, watched bool) error {
		if watched {
			t.Error("sent watched instead of unwatched")
		}
		return nil
	}}
	s.dispatchKey(control.Options)
	if s.optionsPresentation() != nil || !s.watched.busy {
		t.Fatal("watched action opened a menu instead of saving directly")
	}
	s.dispatchKey(control.Open)
	receiveOptionsResult(t, s).apply(s)
	if s.model.Current().Detail.UserData.Played || s.model.Current().Detail.UserData.PlaybackPositionTicks != 0 || s.watched.message != messageMarkedUnwatched {
		t.Fatal("unwatched result not applied")
	}
}

func TestMusicSelectOpensOptionsAndXChangesBackground(t *testing.T) {
	s := setupMusicSession(t)
	library, err := musicviz.Load(t.TempDir() + "/missing.json")
	if err != nil {
		t.Fatal(err)
	}
	s.music.library = library
	s.dispatchKey(control.Options)
	if s.options.visible || !s.music.manual {
		t.Fatal("X did not change the background directly")
	}
	index := s.music.index
	s.dispatchKey(control.Options.Repeat())
	if s.music.index != index {
		t.Fatal("held X cycled backgrounds")
	}
	s.dispatchKey(control.Select)
	if !s.options.visible || s.music.index != index {
		t.Fatal("Select changed the background instead of opening options")
	}
	s.dispatchKey(control.Select)
	if s.options.visible || !s.controller.running {
		t.Fatal("Select did not dismiss options while preserving playback")
	}
}

func TestWatchedHeldAndBusyInputsDoNotRepeat(t *testing.T) {
	s := watchedSession(t, nil)
	s.dispatchKey(control.Options.Repeat())
	if s.watched.busy {
		t.Fatal("held X started a watched change")
	}
	s.dispatchKey(control.Options)
	generation := s.watched.generation
	for _, key := range []control.Action{control.Options, control.Open, control.Select, control.Back} {
		s.dispatchKey(key)
	}
	if s.watched.generation != generation || s.model.Current().Detail == nil {
		t.Fatal("busy watched change was repeated or navigated away")
	}
	receiveOptionsResult(t, s).apply(s)
	s.about.Checked = true
	s.dispatchKey(control.About)
	if s.watched.message != "" || s.options.visible {
		t.Fatal("watched feedback followed navigation")
	}
}
