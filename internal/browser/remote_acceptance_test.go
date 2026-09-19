package browser

import (
	"context"
	"testing"
	"time"

	"mistervision/internal/media"
	"mistervision/internal/remote"
)

func TestRemoteAcceptanceRejectsStaleOrCanceledCommands(t *testing.T) {
	s := testSession(t)
	for _, stale := range []bool{false, true} {
		done := make(chan struct{})
		accepted := make(chan bool, 1)
		generation := s.remote.generation
		if stale {
			generation--
		} else {
			close(done)
		}
		result := remoteCommandResult{generation: generation, command: remote.Command{Kind: remote.Stop, Accepted: accepted, Done: done}}
		if result.apply(s) {
			t.Fatal("invalid command applied")
		}
		select {
		case ok := <-accepted:
			if ok {
				t.Fatal("invalid command accepted")
			}
		default:
			t.Fatal("missing rejection")
		}
		if s.controller.stoppedByUser {
			t.Fatal("invalid command stopped local playback")
		}
	}
}

func TestCanceledRemoteCatalogResultCannotStartPlayback(t *testing.T) {
	s := testSession(t)
	oldItem := s.controller.item.ID
	done := make(chan struct{})
	close(done)
	accepted := make(chan bool, 1)
	s.remoteRequests.resolving = true
	r := remoteItemsResult{generation: s.remoteRequests.generation, command: remote.Command{Kind: remote.Play, PlayMode: remote.PlayNow, Done: done, Accepted: accepted}, items: []media.Item{{ID: "replacement", Type: "Movie"}}}
	r.apply(s)
	if s.remoteRequests.resolving || s.controller.item.ID != oldItem {
		t.Fatal("canceled lookup changed playback or stayed pending")
	}
	select {
	case ok := <-accepted:
		if ok {
			t.Fatal("canceled catalog result accepted")
		}
	default:
		t.Fatal("missing rejection")
	}
}

// delayedRemote holds teardown open to test rapid session replacement.
type delayedRemote struct {
	started chan struct{}
	release <-chan struct{}
}

func (r delayedRemote) Run(ctx context.Context, _ func(remote.Command)) {
	if ctx.Err() != nil {
		return
	}
	close(r.started)
	<-ctx.Done()
	if r.release != nil {
		<-r.release
	}
}

func TestRemoteRestartWaitsForAllEarlierListeners(t *testing.T) {
	s := testSession(t)
	release := make(chan struct{})
	first := delayedRemote{started: make(chan struct{}), release: release}
	middle := delayedRemote{started: make(chan struct{})}
	last := delayedRemote{started: make(chan struct{})}
	s.controlSource = first
	s.startRemote()
	<-first.started
	s.controlSource = middle
	s.startRemote()
	s.controlSource = last
	s.startRemote()
	select {
	case <-last.started:
		t.Fatal("listener restarted before old teardown")
	case <-time.After(10 * time.Millisecond):
	}
	close(release)
	select {
	case <-last.started:
	case <-time.After(time.Second):
		t.Fatal("replacement did not start")
	}
	s.stopRemote()
	s.remote.workers.Wait()
	select {
	case <-middle.started:
		t.Fatal("canceled intermediate source started")
	default:
	}
}
