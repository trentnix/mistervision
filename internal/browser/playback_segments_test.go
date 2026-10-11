package browser

import (
	"testing"
	"time"

	"mistervision/internal/input/control"
	"mistervision/internal/media"
	"mistervision/internal/playback"
)

func TestSegmentPromptFollowsPosition(t *testing.T) {
	f := newControllerFixture(t)
	c := f.c
	c.segments = []media.Segment{{Kind: media.SegmentIntro, StartTicks: 100000000, EndTicks: 200000000}}
	for _, tc := range []struct {
		position int64
		label    string
	}{{0, ""}, {100000000, "Skip intro"}, {199999999, "Skip intro"}, {200000000, ""}, {150000000, "Skip intro"}} {
		c.updatePosition(tc.position, f.now)
		if got := c.Snapshot(f.now).SkipLabel; got != tc.label {
			t.Fatalf("position %d: %q", tc.position, got)
		}
	}
	c.state.RevealControls(f.now)
	if c.Snapshot(f.now).SkipLabel == "" {
		t.Fatal("controls hid skip action")
	}
	c.picker.visible = true
	if c.Snapshot(f.now).SkipLabel != "" {
		t.Fatal("skip visible in track menu")
	}
	c.picker.visible = false
	c.state.Buffering = true
	if c.Snapshot(f.now).SkipLabel != "" {
		t.Fatal("skip visible while buffering")
	}
	c.state.Buffering = false
	c.state.seekVideo(&c.item, control.SeekForward, f.now)
	if c.Snapshot(f.now).SkipLabel != "" {
		t.Fatal("skip visible while seeking")
	}
	c.clearSeek()
	c.Start(c.item, nil, false, f.now)
	if len(c.segments) != 0 {
		t.Fatal("new item inherited segments")
	}
}

func TestSkipSegmentPreservesPauseAndPostCredits(t *testing.T) {
	for _, paused := range []bool{false, true} {
		f := newControllerFixture(t)
		c := f.c
		c.setSegments([]media.Segment{{Kind: media.SegmentCredits, StartTicks: 0, EndTicks: 3000000000}}, f.now)
		if paused {
			c.SetPaused(true)
			expectCommand(t, f.calls[0].controls, playback.SetPaused)
		}
		c.Key(control.Options, f.now)
		if len(f.calls) != 2 || *f.calls[1].offset != 3000000000 || c.wantsPause() != paused {
			t.Fatal("skip did not preserve marker endpoint or pause intent")
		}
		if !paused {
			expectCommand(t, f.calls[0].controls, playback.SetPaused)
		}
		if c.Snapshot(f.now).SkipLabel != "" || c.Snapshot(f.now).WaitLabel != "Seeking..." {
			t.Fatal("skip prompt did not become Seeking")
		}
		c.Key(control.Options, f.now)
		if len(f.calls) != 2 {
			t.Fatal("repeated skip launched another decoder")
		}
		c.Handle(PlaybackEvent{Kind: PlaybackPrepared, ID: 2}, f.now)
		c.Handle(PlaybackEvent{Kind: PlaybackEnded, ID: 1}, f.now)
		c.Handle(PlaybackEvent{Kind: PlaybackPosition, ID: 2, Ticks: 3000000000}, f.now)
		if c.Snapshot(f.now).Paused != paused {
			t.Fatal("replacement did not preserve paused state")
		}
		if paused {
			expectCommand(t, f.calls[1].controls, playback.SetPaused)
		}
	}
}

func TestSegmentUnavailableOrFinalSecond(t *testing.T) {
	for _, kind := range []string{"Audio", "TvChannel", "Movie"} {
		f := newControllerFixture(t)
		c := f.c
		c.item.Type = kind
		c.state.PlayingVideo = kind != "Audio"
		c.segments = []media.Segment{{Kind: media.SegmentCredits, StartTicks: 0, EndTicks: c.item.RunTimeTicks}}
		if kind == "Movie" {
			c.updatePosition(c.item.RunTimeTicks-10000000, f.now)
		}
		c.Key(control.Options, f.now)
		if len(f.calls) != 1 || c.Snapshot(f.now).SkipLabel != "" {
			t.Fatalf("unexpected skip for %s", kind)
		}
	}
}

func TestSessionRoutesSkipAndIgnoresHeldButton(t *testing.T) {
	s := testSession(t)
	now := time.Now()
	s.controller.state.LastAdvance = now
	s.controller.setSegments([]media.Segment{{Kind: media.SegmentIntro, EndTicks: 100000000}}, now)
	s.dispatchKeyAt(control.Action("options-repeat"), now)
	if s.controller.state.SeekTarget != nil {
		t.Fatal("held contextual action skipped")
	}
	s.dispatchKeyAt(control.Options, now)
	if s.controller.pendingTarget != 100000000 {
		t.Fatal("contextual action did not skip")
	}
}

func TestSegmentPromptTimeoutAndManualReveal(t *testing.T) {
	f := newControllerFixture(t)
	c := f.c
	c.state.BufferingKnown = true
	c.setSegments([]media.Segment{{Kind: media.SegmentIntro, EndTicks: 600000000}}, f.now)
	for _, tc := range []struct {
		elapsed time.Duration
		visible bool
	}{
		{0, true}, {8*time.Second - time.Nanosecond, true}, {8 * time.Second, false}, {12 * time.Second, false},
	} {
		now := f.now.Add(tc.elapsed)
		c.updatePosition(20000000, now) // Repeated progress must not renew the timer.
		if got := c.Snapshot(now).SkipLabel != ""; got != tc.visible {
			t.Fatalf("at %s: visible=%v", tc.elapsed, got)
		}
	}
	now := f.now.Add(12 * time.Second)
	c.Key(control.Options, now)
	if len(f.calls) != 1 {
		t.Fatal("hidden skip action still accepted input")
	}
	c.Key(control.ToggleControls, now)
	if c.Snapshot(now).SkipLabel == "" {
		t.Fatal("opening controls did not restore prompt")
	}
	c.Key(control.ToggleControls, now.Add(time.Second))
	if c.Snapshot(now.Add(time.Second)).SkipLabel != "" {
		t.Fatal("closing controls left prompt visible")
	}
	c.Key(control.ToggleControls, now.Add(2*time.Second))
	if c.Snapshot(now.Add(2*time.Second)).SkipLabel == "" {
		t.Fatal("second reveal failed")
	}
	if c.Snapshot(now.Add(5*time.Second)).SkipLabel != "" {
		t.Fatal("controls timeout left prompt visible")
	}
	c.updatePosition(600000000, now.Add(6*time.Second))
	c.Key(control.ToggleControls, now.Add(6*time.Second))
	if c.Snapshot(now.Add(6*time.Second)).SkipLabel != "" {
		t.Fatal("controls revived expired segment")
	}
	c.state.HideControls()
	c.updatePosition(100000000, now.Add(7*time.Second))
	if c.Snapshot(now.Add(7*time.Second)).SkipLabel == "" {
		t.Fatal("seeking back did not show new invitation")
	}
}

func TestSegmentPromptDoesNotRestartAfterPauseOrTemporaryOverlay(t *testing.T) {
	for _, mode := range []string{"pause", "buffering", "picker"} {
		f := newControllerFixture(t)
		c := f.c
		c.state.BufferingKnown = true
		c.setSegments([]media.Segment{{Kind: media.SegmentIntro, EndTicks: 600000000}}, f.now)
		switch mode {
		case "pause":
			c.SetPaused(true)
		case "buffering":
			c.state.Buffering = true
		case "picker":
			c.picker.visible = true
		}
		now := f.now.Add(9 * time.Second)
		c.state.Buffering = false
		c.picker.visible = false
		c.updatePosition(20000000, now)
		if c.Snapshot(now).SkipLabel != "" {
			t.Fatalf("%s restarted the timer", mode)
		}
	}
}

func TestManualControlsTakeOverInitialSegmentPrompt(t *testing.T) {
	f := newControllerFixture(t)
	c := f.c
	c.setSegments([]media.Segment{{Kind: media.SegmentIntro, EndTicks: 600000000}}, f.now)
	c.Key(control.ToggleControls, f.now.Add(time.Second))
	c.Key(control.ToggleControls, f.now.Add(2*time.Second))
	if c.Snapshot(f.now.Add(2*time.Second)).SkipLabel != "" {
		t.Fatal("manual dismissal restored automatic prompt")
	}
}
