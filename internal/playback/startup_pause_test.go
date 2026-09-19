package playback

import (
	"bytes"
	"syscall"
	"testing"
	"time"

	"mistervision/internal/media"
	"mistervision/internal/player"
	"mistervision/internal/player/ffplay"
	"mistervision/internal/player/mplayer"
)

func TestNativeStartupPauseWaitsForFirstFrameNotPosition(t *testing.T) {
	var commands bytes.Buffer
	p := &playerProcess{decoder: mplayer.Decoder{}, control: player.Control{Stdin: &commands}}
	s := playbackSession{item: media.Item{Type: "Movie"}, start: 1200000000, trace: &playbackTrace{}}
	timer := time.NewTimer(time.Minute)
	defer timer.Stop()
	paused := false
	s.control(p, Callbacks{Paused: func(value bool) { paused = value }}, Control{Kind: SetPaused}, timer)
	if commands.Len() != 0 || paused || s.state.IsPaused || !s.pauseOnReady {
		t.Fatal("pause was sent while MPlayer can discard it during cache opening")
	}
	s.videoStarted = true
	s.control(p, Callbacks{Paused: func(value bool) { paused = value }}, Control{Kind: SetPaused}, timer)
	if commands.String() != "pause\n" || !paused || !s.state.IsPaused || s.started {
		t.Fatal("first frame did not pause before position feedback")
	}
	if !timer.Stop() {
		t.Fatal("startup pause disabled the first-frame deadline")
	}
	// Avoid server reporting in this unit test, while exercising the paused
	// position rule once the first frame has been accepted.
	s.started = true
	s.state.PositionTicks = 1200400000
	s.update(0.5, nil, timer)
	if s.state.PositionTicks != 1200400000 {
		t.Fatal("paused frames advanced the position")
	}
}

func TestProcessSuspensionDecoderDefersStartupPause(t *testing.T) {
	signals := 0
	p := &playerProcess{decoder: ffplay.Decoder{}, control: player.Control{Signal: func(syscall.Signal) error { signals++; return nil }}}
	s := playbackSession{item: media.Item{Type: "Movie"}}
	timer := time.NewTimer(time.Minute)
	defer timer.Stop()
	s.control(p, Callbacks{}, Control{Kind: SetPaused}, timer)
	if signals != 0 || !s.pauseOnReady || s.state.IsPaused {
		t.Fatal("startup pause froze an unrendered frame")
	}
	s.control(p, Callbacks{}, Control{Kind: Resume}, timer)
	if signals != 0 || s.pauseOnReady {
		t.Fatal("resume did not cancel deferred startup pause")
	}
}
