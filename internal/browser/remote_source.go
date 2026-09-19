package browser

import (
	"context"
	"sync"
	"time"

	"mistervision/internal/playback"
	"mistervision/internal/remote"
	"mistervision/internal/rendering"
)

// remoteSession owns the control source for one authenticated account. Commands
// enter the same event loop as physical input without changing button labels.
type remoteSession struct {
	source            remote.Source
	observer          remote.PlaybackObserver
	lastPlayback      remote.PlaybackState
	playbackPublished bool
	playbackSeen      bool // A new source must not receive the previous account's stopped item.
	cancel            context.CancelFunc
	workers           sync.WaitGroup
	done              <-chan struct{} // Serializes listener teardown without blocking the UI.
	generation        int
}

func (s *browserSession) startRemote() {
	s.stopRemote()
	source := s.controlSource
	if source == nil || s.about.Updating || !s.update.exitAt.IsZero() {
		return
	}
	ctx, cancel := context.WithCancel(s.ctx)
	s.remote.source, s.remote.cancel = source, cancel
	s.remote.observer, _ = source.(remote.PlaybackObserver)
	s.remote.playbackPublished = false
	s.remote.playbackSeen = false
	s.publishRemotePlayback(time.Now())
	generation := s.remote.generation
	previous := s.remote.done
	done := make(chan struct{})
	s.remote.done = done
	s.remote.workers.Add(1)
	go func() {
		defer s.remote.workers.Done()
		defer close(done)
		if previous != nil {
			<-previous
		}
		source.Run(ctx, func(command remote.Command) { s.send(ctx, remoteCommandResult{generation, command}) })
	}()
}

func (s *browserSession) stopRemote() {
	if s.remote.cancel != nil {
		s.remote.cancel()
	}
	s.remote.generation++
	s.remote.source = nil
	s.remote.observer = nil
	s.remote.playbackPublished = false
	s.remote.playbackSeen = false
	s.remoteRequests.cancelAll()
	s.playbackQueue.active = false
	s.playbackQueue.switching = false
	s.playbackQueue.queue.Replace(nil, 0)
	s.playbackQueue.items = nil
	s.playbackQueue.localRows = nil
}

type remoteCommandResult struct {
	generation int
	command    remote.Command
}

func (r remoteCommandResult) apply(s *browserSession) bool {
	if r.command.Canceled() || r.generation != s.remote.generation || s.setup.Kind != rendering.SetupHidden || s.connection.forgetting {
		r.command.Acknowledge(false)
		return false
	}
	accepted := s.handleRemote(r.command)
	if r.command.Kind != remote.Play || !accepted {
		s.publishRemotePlayback(time.Now())
		r.command.Acknowledge(accepted)
	}
	return accepted
}

func (s *browserSession) publishRemoteQueue() {
	observer, ok := s.remote.source.(remote.QueueObserver)
	if !ok {
		return
	}
	state := s.playbackQueue.queue.Snapshot()
	observer.Publish(state)
	if s.controller.running {
		s.controller.sendCommand(playback.Report)
	}
}

// publishRemotePlayback sends changed facts only. Sources own any network pacing.
// Keeping playback separate avoids rebuilding large queues on position updates.
func (s *browserSession) publishRemotePlayback(now time.Time) {
	if s.remote.observer == nil {
		return
	}
	if s.controller.running && !s.controller.stoppedByUser {
		s.remote.playbackSeen = true
	}
	state := remote.PlaybackState{Status: remote.Stopped}
	if s.remote.playbackSeen {
		state = s.controller.RemoteState(now)
	}
	if s.playbackQueue.switching {
		if item, ok := s.playbackQueue.items[s.playbackQueue.queue.Current().ID]; ok {
			state = remote.PlaybackState{Status: remote.Loading, ItemID: item.ID, DurationTicks: item.RunTimeTicks, Audio: item.Type == "Audio"}
		}
	}
	if s.remote.playbackPublished && state == s.remote.lastPlayback {
		return
	}
	s.remote.lastPlayback, s.remote.playbackPublished = state, true
	s.remote.observer.PublishPlayback(state)
}
