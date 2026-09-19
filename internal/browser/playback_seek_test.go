package browser

import (
	"errors"
	"testing"
	"time"

	"mistervision/internal/input/control"
	"mistervision/internal/playback"
)

func fillPlayerCommands(c *PlaybackController) {
	for len(c.active.controls) < cap(c.active.controls) {
		c.active.controls <- playback.Control{Kind: playback.Report}
	}
}

func drainPlayerCommands(c *PlaybackController) {
	for len(c.active.controls) > 0 {
		<-c.active.controls
	}
}

func TestSeekWaitsForPauseDelivery(t *testing.T) {
	f := newControllerFixture(t)
	c := f.c
	fillPlayerCommands(c)
	c.Key(control.SeekForward, f.now)
	c.Tick(f.now.Add(time.Second))
	if len(f.calls) != 1 || c.state.SeekInFlight {
		t.Fatal("busy decoder started seek without accepting pause")
	}
	drainPlayerCommands(c)
	c.Tick(f.now.Add(2 * time.Second))
	expectCommand(t, c.active.controls, playback.SetPaused)
	if len(f.calls) != 2 || !c.state.SeekInFlight {
		t.Fatal("seek did not retry after commands drained")
	}
	c.Handle(PlaybackEvent{Kind: PlaybackEnded, ID: c.pending.id, Err: errors.New("preparation failed")}, f.now)
	expectCommand(t, c.active.controls, playback.Resume)
}

func TestFailedSeekRetriesResumeDelivery(t *testing.T) {
	f := newControllerFixture(t)
	c := f.c
	c.Key(control.SeekForward, f.now)
	c.Tick(f.now.Add(time.Second))
	expectCommand(t, c.active.controls, playback.SetPaused)
	c.Handle(PlaybackEvent{Kind: PlaybackPaused, ID: c.active.id, Value: true}, f.now)
	fillPlayerCommands(c)
	c.Handle(PlaybackEvent{Kind: PlaybackEnded, ID: c.pending.id, Err: errors.New("preparation failed")}, f.now)
	c.Tick(f.now.Add(2 * time.Second))
	if !c.running || c.wantsPause() || c.state.SeekTarget != nil {
		t.Fatal("failure lost playback intent or retained seek")
	}
	drainPlayerCommands(c)
	c.Tick(f.now.Add(3 * time.Second))
	expectCommand(t, c.active.controls, playback.Resume)
	c.Tick(f.now.Add(4 * time.Second))
	if len(c.active.controls) != 0 {
		t.Fatal("resume was delivered more than once")
	}
}

func TestPendingSeekResumeRespectsNewInput(t *testing.T) {
	for _, action := range []control.Action{control.Open, control.Back} {
		t.Run(string(action), func(t *testing.T) {
			f := newControllerFixture(t)
			c := f.c
			c.Key(control.SeekForward, f.now)
			c.Tick(f.now.Add(time.Second))
			expectCommand(t, c.active.controls, playback.SetPaused)
			c.Handle(PlaybackEvent{Kind: PlaybackPaused, ID: c.active.id, Value: true}, f.now)
			fillPlayerCommands(c)
			c.Handle(PlaybackEvent{Kind: PlaybackEnded, ID: c.pending.id, Err: errors.New("preparation failed")}, f.now)
			c.Key(action, f.now)
			drainPlayerCommands(c)
			c.Tick(f.now.Add(2 * time.Second))
			if action == control.Open {
				expectCommand(t, c.active.controls, playback.SetPaused)
			}
			if len(c.active.controls) != 0 {
				t.Fatal("obsolete resume survived newer input")
			}
		})
	}
}

func TestReplacementPauseRetriesWithoutPositionUpdates(t *testing.T) {
	f := newControllerFixture(t)
	c := f.c
	setControllerPaused(t, c, true, f.now)
	c.Key(control.SeekForward, f.now)
	c.Tick(f.now.Add(time.Second))
	c.Handle(PlaybackEvent{Kind: PlaybackPrepared, ID: c.pending.id}, f.now)
	c.Handle(PlaybackEvent{Kind: PlaybackEnded, ID: c.active.id}, f.now)
	expectCommand(t, c.active.controls, playback.SetPaused)
	fillPlayerCommands(c)
	c.Handle(PlaybackEvent{Kind: PlaybackPosition, ID: c.active.id, Ticks: 320000000}, f.now)
	drainPlayerCommands(c)
	c.Tick(f.now.Add(2 * time.Second))
	if len(c.active.controls) != 0 {
		t.Fatal("seek startup repeated its primed pause")
	}
	if !c.wantsPause() {
		t.Fatal("replacement lost paused intent")
	}
}

func TestSeekPreservesIntentDespiteDelayedPauseFeedback(t *testing.T) {
	for _, paused := range []bool{false, true} {
		name := "resume"
		if paused {
			name = "pause"
		}
		t.Run(name, func(t *testing.T) {
			f := newControllerFixture(t)
			c := f.c
			c.SetPaused(!paused)
			drainPlayerCommands(c)
			c.SetPaused(paused)
			drainPlayerCommands(c)
			c.Handle(PlaybackEvent{Kind: PlaybackPaused, ID: c.active.id, Value: !paused}, f.now)
			c.Key(control.SeekForward, f.now)
			c.Tick(f.now.Add(time.Second))
			if !paused {
				expectCommand(t, c.active.controls, playback.SetPaused)
			}
			if len(c.active.controls) != 0 {
				t.Fatal("seek ignored user pause intent")
			}
			c.Handle(PlaybackEvent{Kind: PlaybackPrepared, ID: c.pending.id}, f.now)
			c.Handle(PlaybackEvent{Kind: PlaybackEnded, ID: c.active.id}, f.now)
			c.Handle(PlaybackEvent{Kind: PlaybackPosition, ID: c.active.id, Ticks: 320000000}, f.now)
			if paused {
				expectCommand(t, c.active.controls, playback.SetPaused)
			}
			if len(c.active.controls) != 0 || c.wantsPause() != paused {
				t.Fatal("replacement inherited delayed feedback instead of user intent")
			}
		})
	}
}

func TestRetargetedSeekRoutesCommandsToReplacement(t *testing.T) {
	f := newControllerFixture(t)
	c := f.c
	// Leave refresh and automatic pause queued when the original stops.
	c.Refresh()
	c.Key(control.SeekForward, f.now)
	c.Tick(f.now.Add(time.Second))
	c.Handle(PlaybackEvent{Kind: PlaybackPrepared, ID: c.pending.id}, f.now)
	c.Key(control.SeekForward, f.now.Add(time.Second))
	c.Handle(PlaybackEvent{Kind: PlaybackEnded, ID: c.active.id}, f.now)
	if c.Refresh() {
		t.Fatal("refresh reached a decoder during the handoff gap")
	}
	c.Tick(f.now.Add(2 * time.Second))
	c.Handle(PlaybackEvent{Kind: PlaybackPrepared, ID: c.pending.id}, f.now)
	c.Handle(PlaybackEvent{Kind: PlaybackPosition, ID: c.active.id, Ticks: 620000000}, f.now)
	if len(f.calls[2].controls) != 0 {
		t.Fatal("replacement inherited commands")
	}
	c.SetPaused(true)
	expectCommand(t, f.calls[2].controls, playback.SetPaused)
	expectCommand(t, f.calls[0].controls, playback.Refresh)
	expectCommand(t, f.calls[0].controls, playback.SetPaused)
	if len(f.calls[1].controls) != 0 {
		t.Fatal("canceled replacement received active controls")
	}
}

func TestFailedSeekKeepsNewPauseIntent(t *testing.T) {
	f := newControllerFixture(t)
	c := f.c
	c.Key(control.SeekForward, f.now)
	c.Tick(f.now.Add(time.Second))
	expectCommand(t, c.active.controls, playback.SetPaused)
	c.SetPaused(true)
	c.Handle(PlaybackEvent{Kind: PlaybackEnded, ID: c.pending.id, Err: errors.New("preparation failed")}, f.now)
	c.Tick(f.now.Add(2 * time.Second))
	if !c.wantsPause() || len(c.active.controls) != 0 {
		t.Fatal("failed seek resumed despite user pause")
	}
	c.Key(control.Open, f.now)
	expectCommand(t, c.active.controls, playback.Resume)
}

func TestPausedSeekIsPrimedBeforeFirstReplacementPosition(t *testing.T) {
	f := newControllerFixture(t)
	c := f.c
	c.SetPaused(true)
	drainPlayerCommands(c)
	c.SeekTo(300000000, f.now)
	c.Tick(f.now.Add(time.Second))
	c.Handle(PlaybackEvent{Kind: PlaybackPrepared, ID: c.pending.id}, f.now)
	c.Handle(PlaybackEvent{Kind: PlaybackEnded, ID: c.active.id}, f.now)
	expectCommand(t, c.active.controls, playback.SetPaused)
	if c.state.ProgressSeen {
		t.Fatal("fixture already supplied replacement feedback")
	}
	// Resume must reach a loading replacement instead of being lost until a
	// position report from a decoder that has already been told to pause.
	c.SetPaused(false)
	expectCommand(t, c.active.controls, playback.Resume)
	c.Handle(PlaybackEvent{Kind: PlaybackPosition, ID: c.active.id, Ticks: 300000000}, f.now)
	if len(c.active.controls) != 0 {
		t.Fatal("first position replayed obsolete pause intent")
	}
}

func TestRemoteSeekCanRetargetAReplacementBeforeItsFirstPosition(t *testing.T) {
	f := newControllerFixture(t)
	c := f.c
	c.SeekTo(300000000, f.now)
	c.Tick(f.now.Add(time.Second))
	c.Handle(PlaybackEvent{Kind: PlaybackPrepared, ID: c.pending.id}, f.now)
	c.Handle(PlaybackEvent{Kind: PlaybackEnded, ID: c.active.id}, f.now)
	c.SeekTo(600000000, f.now.Add(2*time.Second))
	if c.state.SeekTarget == nil || *c.state.SeekTarget != 600000000 {
		t.Fatal("seek was discarded while the replacement loaded")
	}
}

func TestPrimedPauseKeepsLoadingVisibleUntilFirstFrame(t *testing.T) {
	now := time.Now()
	state := playbackState{PlayingVideo: true, Paused: true, LastAdvance: now}
	if state.videoWaitLabel(now) != "Loading..." {
		t.Fatal("primed pause hid startup feedback")
	}
	state.VideoStarted = true
	if state.videoWaitLabel(now) != "" {
		t.Fatal("paused first frame kept loading feedback")
	}
}

func TestRemoteSeekAcceptsFirstFrameBeforePosition(t *testing.T) {
	f := newControllerFixture(t)
	c := f.c
	c.state.ProgressSeen, c.state.PositionKnown = false, false
	c.state.VideoStarted = true
	c.SeekTo(300000000, f.now)
	if c.state.SeekTarget == nil || *c.state.SeekTarget != 300000000 {
		t.Fatal("visible video rejected an absolute seek before its first position poll")
	}
}
