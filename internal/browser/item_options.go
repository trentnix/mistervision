package browser

import (
	"context"
	"time"

	"mistervision/internal/input/control"
	"mistervision/internal/media"
	"mistervision/internal/remote"
	"mistervision/internal/rendering"
)

// itemOptions owns the music options menu and its request lifetime. Work captures
// inputs before leaving the event loop. Closing the menu invalidates its results.
type itemOptions struct {
	visible, busy        bool
	selected, generation int
	itemID, message      string
	cancel               context.CancelFunc
}

func (o *itemOptions) close() {
	if o.cancel != nil {
		o.cancel()
	}
	*o = itemOptions{generation: o.generation + 1}
}

// optionsAvailable excludes setup, active video, and incomplete track handoffs.
// Whole-library shuffle keeps its streaming batch behavior rather than silently
// turning the current batch into a finite album queue.
func (s *browserSession) optionsAvailable() bool {
	if s.about.Visible || s.setup.Kind != rendering.SetupHidden || s.model.ExitConfirm || s.media.pending || s.media.queued != nil || s.playbackQueue.switching || s.remoteRequests.resolving {
		return false
	}
	item := s.model.Current().Detail
	if item == nil {
		return false
	}
	if s.controller.running {
		return item.Type == "Audio" && !s.controller.stoppedByUser && s.shuffle.library == ""
	}
	if s.model.MusicQueueActive() || item.Type != "Movie" && item.Type != "Episode" {
		return false
	}
	// A final playback report must finish before a manual change can supersede it.
	if s.controller.active.id != 0 && s.controller.item.ID == item.ID && !s.controller.cleanupComplete {
		return false
	}
	_, ok := s.client.(media.WatchedState)
	return ok
}

func (s *browserSession) openOptions() {
	s.options.close()
	s.options.visible = true
	s.options.itemID = s.model.Current().Detail.ID
	s.controller.state.HideControls()
}

func (s *browserSession) optionsPresentation() *rendering.OptionsMenu {
	o := &s.options
	if !o.visible {
		return nil
	}
	repeat, shuffle := "Off", "Off"
	if s.playbackQueue.active {
		switch s.playbackQueue.queue.Repeat() {
		case remote.RepeatAll:
			repeat = "All"
		case remote.RepeatOne:
			repeat = "One"
		}
		if s.playbackQueue.queue.Shuffled() {
			shuffle = "On"
		}
	}
	return &rendering.OptionsMenu{
		Title: "Music options", Selected: o.selected, Busy: o.busy, Message: o.message,
		Rows: []string{"Repeat: " + repeat, "Shuffle: " + shuffle},
	}
}

func (s *browserSession) handleOptionsKey(key control.Action) bool {
	o := &s.options
	if o.busy {
		if key == control.Back || key == control.Select {
			o.close()
			return true
		}
		return false
	}
	switch key {
	case control.Back, control.Select:
		o.close()
	case control.Up:
		o.selected = 0
	case control.Down:
		o.selected = 1
	case control.Open:
		s.changeMusicOption()
	}
	return true
}

// optionsRequest captures menu and account generations for a bounded worker.
func (s *browserSession) optionsRequest(message string) (context.Context, int, int) {
	s.options.busy = true
	s.options.message = message
	ctx, cancel := context.WithTimeout(s.ctx, 15*time.Second)
	s.options.cancel = cancel
	return ctx, s.options.generation, s.connection.generation
}

func (s *browserSession) applyMusicOption() {
	q := &s.playbackQueue.queue
	if s.options.selected == 0 {
		mode := remote.RepeatAll
		switch q.Repeat() {
		case remote.RepeatAll:
			mode = remote.RepeatOne
		case remote.RepeatOne:
			mode = remote.RepeatNone
		}
		q.SetRepeat(mode)
	} else {
		q.SetShuffle(!q.Shuffled())
	}
	s.options.message = ""
	s.publishRemoteQueue()
}
