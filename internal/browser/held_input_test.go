package browser

import (
	"testing"
	"time"

	"mistervision/internal/input/control"
	"mistervision/internal/media"
	"mistervision/internal/playback"
	"mistervision/internal/remote"
	"mistervision/internal/rendering"
)

func TestHeldSeekAccumulatesWithoutListAcceleration(t *testing.T) {
	for _, kind := range []string{"Movie", "Audio", "TvChannel"} {
		t.Run(kind, func(t *testing.T) {
			s := testSession(t)
			s.controller.item.Type = kind
			s.model.Stack = append(s.model.Stack, View{Detail: &s.controller.item})
			s.controller.state.ProgressSeen = true
			s.controller.state.PlayingVideo = kind != "Audio"
			s.controller.state.PositionTicks = 100 * 10000000
			s.controller.item.RunTimeTicks = 3600 * 10000000
			drainPlayerCommands(s.controller)
			start := time.Unix(100, 0)
			s.dispatchKeyAt(control.Next, start)
			for _, ms := range []int{350, 400, 460, 600, 650, 850, 900} {
				s.dispatchKeyAt(control.Next.Repeat(), start.Add(time.Duration(ms)*time.Millisecond))
			}
			switch kind {
			case "Movie":
				if s.controller.state.SeekPresses != 4 || *s.controller.state.SeekTarget != 220*10000000 {
					t.Fatal("held seek did not accumulate at a steady rate", s.controller.state)
				}
				deadline := start.Add(1350 * time.Millisecond)
				if !s.controller.state.SeekDeadline.Equal(deadline) {
					t.Fatal("wrong seek settle time")
				}
				s.controller.Tick(deadline.Add(-time.Millisecond))
				if s.controller.state.SeekInFlight {
					t.Fatal("started seeking while destination was still changing")
				}
				s.controller.Tick(deadline)
				if !s.controller.state.SeekInFlight {
					t.Fatal("did not seek after the half-second delay")
				}
			case "Audio":
				count := 0
				for len(s.controller.active.controls) > 0 {
					cmd := <-s.controller.active.controls
					if cmd.Kind != playback.SeekAudioStep || cmd.Seconds != 10 {
						t.Fatal("unexpected audio command", cmd)
					}
					count++
				}
				if count != 4 {
					t.Fatal("wrong audio seek count", count)
				}
			case "TvChannel":
				if s.controller.state.SeekTarget != nil || len(s.controller.active.controls) != 0 {
					t.Fatal("live TV accepted seeking")
				}
			}
		})
	}
}

func TestSeekRepeatPreservesTapsAndDirectionChanges(t *testing.T) {
	var r seekRepeat
	start := time.Unix(100, 0)
	for _, step := range []struct {
		key    control.Action
		repeat bool
		ms     int
		want   bool
	}{
		{control.SeekForward, false, 0, true},
		{control.SeekForward, false, 20, true},
		{control.SeekForward, true, 30, false},
		{control.SeekBackward, false, 40, true},
		{control.SeekBackward, true, 289, false},
		{control.SeekBackward, true, 290, true},
		{control.SeekBackward, true, 1000, true},
		{control.SeekBackward, true, 1001, false},
	} {
		if got := r.accept(step.key, step.repeat, start.Add(time.Duration(step.ms)*time.Millisecond)); got != step.want {
			t.Fatal(step, got)
		}
	}
}

func TestGlobalExitConfirmationPreservesUnderlyingScreen(t *testing.T) {
	for _, screen := range []string{"video", "music", "photo", "about", "setup", "options", "watched", "update"} {
		t.Run(screen, func(t *testing.T) {
			s := testSession(t)
			s.model.Stack = append(s.model.Stack, View{Detail: &media.Item{Type: "Movie"}})
			switch screen {
			case "music":
				s.model.Current().Detail.Type = "Audio"
			case "photo":
				s.model.Current().Detail.Type = "Photo"
			case "about":
				s.about.Visible = true
			case "setup":
				s.setup.Kind = rendering.SetupPIN
			case "options":
				s.options.visible = true
			case "watched":
				s.watched.busy = true
			case "update":
				s.about.Visible, s.about.Updating = true, true
			}
			id, depth := s.controller.active.id, len(s.model.Stack)
			s.dispatchKey(control.ConfirmExit)
			if !s.confirmExit || s.model.Quit {
				t.Fatal("missing exit confirmation")
			}
			s.dispatchKey(control.Open.Repeat())
			if s.model.Quit {
				t.Fatal("held confirm accepted exit")
			}
			if s.handleRemote(remote.Command{Kind: remote.Play}) {
				t.Fatal("remote play dismissed confirmation")
			}
			s.dispatchKey(control.Back)
			if s.confirmExit || s.model.Quit || s.controller.active.id != id || len(s.model.Stack) != depth || s.controller.stoppedByUser {
				t.Fatal("cancel changed underlying state")
			}
			if screen == "options" && !s.options.visible {
				t.Fatal("cancel lost the options menu")
			}
			s.dispatchKey(control.ConfirmExit)
			s.dispatchKey(control.Open)
			if !s.model.Quit {
				t.Fatal("confirm did not request normal shutdown")
			}
		})
	}
}
