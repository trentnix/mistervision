package browser

import (
	"context"
	"errors"
	"testing"
	"time"

	"mistervision/internal/media"
	"mistervision/internal/remote"
)

func TestRemotePlaybackFacts(t *testing.T) {
	f := newControllerFixture(t)
	if got := f.c.RemoteState(f.now); got.Status != remote.Playing || got.PositionTicks != 20000000 || got.DurationTicks != 6000000000 {
		t.Fatalf("playing: %+v", got)
	}
	f.c.Handle(PlaybackEvent{Kind: PlaybackPaused, ID: 1, Value: true}, f.now)
	if got := f.c.RemoteState(f.now.Add(10 * time.Second)); got.Status != remote.Paused {
		t.Fatalf("paused: %+v", got)
	}
	f.c.Handle(PlaybackEvent{Kind: PlaybackPaused, ID: 1}, f.now)
	f.c.Handle(PlaybackEvent{Kind: PlaybackBuffering, ID: 1, Value: true}, f.now)
	if got := f.c.RemoteState(f.now); got.Status != remote.Buffering {
		t.Fatalf("buffering: %+v", got)
	}
	f.c.Handle(PlaybackEvent{Kind: PlaybackEnded, ID: 1, Err: errors.New("private decoder details")}, f.now)
	if got := f.c.RemoteState(f.now); got.Status != remote.Failed {
		t.Fatalf("failed: %+v", got)
	}
	f.c.Start(media.Item{ID: "new", Type: "Audio"}, nil, false, f.now)
	if got := f.c.RemoteState(f.now); got.Status != remote.Loading || got.PositionTicks != 0 || !got.Audio {
		t.Fatalf("next item: %+v", got)
	}
	f.c.stopByUser()
	if got := f.c.RemoteState(f.now); got.Status != remote.Stopped {
		t.Fatalf("stop: %+v", got)
	}
}

func TestRemoteSeekDoesNotPublishRequestedPositionAsCompleted(t *testing.T) {
	f := newControllerFixture(t)
	const target = 900000000
	f.c.SeekTo(target, f.now)
	if got := f.c.RemoteState(f.now); got.PositionTicks != 20000000 {
		t.Fatalf("preview moved position: %+v", got)
	}
	f.c.Tick(f.now.Add(time.Second))
	if got := f.c.RemoteState(f.now); got.Status != remote.Seeking || got.PositionTicks != 20000000 {
		t.Fatalf("preparing: %+v", got)
	}
	f.c.Handle(PlaybackEvent{Kind: PlaybackPrepared, ID: 2}, f.now)
	f.c.Handle(PlaybackEvent{Kind: PlaybackEnded, ID: 1}, f.now)
	if got := f.c.RemoteState(f.now); got.Status == remote.Playing || got.PositionTicks != 20000000 {
		t.Fatalf("handoff prematurely completed: %+v", got)
	}
	f.c.Handle(PlaybackEvent{Kind: PlaybackPosition, ID: 1, Ticks: 10000000}, f.now)
	if got := f.c.RemoteState(f.now); got.PositionTicks != 20000000 {
		t.Fatalf("stale decoder: %+v", got)
	}
	f.c.Handle(PlaybackEvent{Kind: PlaybackPosition, ID: 2, Ticks: target}, f.now)
	if got := f.c.RemoteState(f.now); got.Status != remote.Playing || got.PositionTicks != target {
		t.Fatalf("first replacement position: %+v", got)
	}
}

type playbackObserver struct{ states []remote.PlaybackState }

func (o *playbackObserver) PublishPlayback(s remote.PlaybackState) { o.states = append(o.states, s) }

func TestRemotePlaybackPublicationOnlyChangesAndStopsWithSession(t *testing.T) {
	s := testSession(t)
	o := &playbackObserver{}
	s.remote.observer = o
	now := time.Now()
	s.publishRemotePlayback(now)
	for range 100 {
		s.publishRemotePlayback(now)
	}
	if len(o.states) != 1 {
		t.Fatalf("unchanged frames published %d times", len(o.states))
	}
	s.controller.Handle(PlaybackEvent{Kind: PlaybackPosition, ID: s.controller.active.id, Ticks: 12345678}, now)
	s.publishRemotePlayback(now)
	if len(o.states) != 2 || o.states[1].PositionTicks != 12345678 {
		t.Fatalf("position not published: %+v", o.states)
	}
	s.stopRemote()
	s.publishRemotePlayback(now.Add(time.Second))
	if len(o.states) != 2 {
		t.Fatal("old account received state")
	}
}

func TestRemotePauseWaitsForDecoderFeedback(t *testing.T) {
	f := newControllerFixture(t)
	f.c.SetPaused(true)
	if got := f.c.RemoteState(f.now); got.Status != remote.Playing {
		t.Fatalf("intent reported as applied: %+v", got)
	}
	f.c.Handle(PlaybackEvent{Kind: PlaybackPaused, ID: 1, Value: true}, f.now)
	if got := f.c.RemoteState(f.now); got.Status != remote.Paused {
		t.Fatalf("feedback not published: %+v", got)
	}
}

func BenchmarkRemotePlaybackUnchanged(b *testing.B) {
	s := &browserSession{controller: &PlaybackController{}}
	s.remote.observer = &playbackObserver{}
	now := time.Now()
	s.publishRemotePlayback(now)
	b.ReportAllocs()
	for b.Loop() {
		s.publishRemotePlayback(now)
	}
}

func TestNewRemoteObserverDoesNotReceivePreviousAccountsStoppedItem(t *testing.T) {
	s := testSession(t)
	s.controller.running = false
	s.controller.failed = true
	s.controller.item = media.Item{ID: "private-previous-account", RunTimeTicks: 12345678}
	s.controller.state.ConfirmedPositionTicks = 9000
	o := &playbackObserver{}
	s.remote.observer = o
	s.publishRemotePlayback(time.Now())
	if len(o.states) != 1 || o.states[0] != (remote.PlaybackState{Status: remote.Stopped}) {
		t.Fatalf("old account leaked: %+v", o.states)
	}
}

func TestRemoteHandoffReportsIncomingItemLoading(t *testing.T) {
	s := testSession(t)
	o := &playbackObserver{}
	s.remote.observer = o
	s.publishRemotePlayback(time.Now())
	s.applyRemoteItems(remote.Command{PlayMode: remote.PlayNow}, []media.Item{{ID: "incoming", Type: "Movie", RunTimeTicks: 120000000}})
	s.publishRemotePlayback(time.Now())
	got := o.states[len(o.states)-1]
	if got.ItemID != "incoming" || got.Status != remote.Loading || got.PositionTicks != 0 {
		t.Fatalf("handoff exposed old playback: %+v", got)
	}
}

// playbackOnlySource deliberately omits queue observation.
type playbackOnlySource struct{}

func (playbackOnlySource) Run(context.Context, func(remote.Command)) {}

func TestPlaybackOnlySourceDoesNotAdoptLocalQueue(t *testing.T) {
	s := testSession(t)
	s.remote.source = playbackOnlySource{}
	s.publishLocalQueue()
	if s.playbackQueue.active {
		t.Fatal("playback-only receiver changed local queue ownership")
	}
}
