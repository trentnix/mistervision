package browser

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"mistervision/internal/input/control"
	"mistervision/internal/media"
	"mistervision/internal/release"
)

func TestAboutPreservesBrowseAndIsolatesInput(t *testing.T) {
	s := testSession(t)
	s.controller.running = false
	s.model.Current().Selected = 3
	s.model.Notice = "Existing notice"
	for _, key := range []control.Action{"about", "about-repeat", "next", "open"} {
		s.handleKey(key)
	}
	if !s.about.Visible || s.model.Current().Selected != 3 || s.model.Notice != "Existing notice" {
		t.Fatal("About leaked input to browser")
	}
	s.handleKey(control.About)
	if s.about.Visible || s.model.Quit || s.model.Notice != "Existing notice" {
		t.Fatal("toggle did not return intact")
	}
	s.handleKey(control.About)
	s.handleKey(control.Back)
	if s.about.Visible || s.model.Quit {
		t.Fatal("Back must close About, not exit")
	}
}

func TestAboutCannotInterruptPlayback(t *testing.T) {
	for _, kind := range []string{"Movie", "Audio", "Photo"} {
		s := testSession(t)
		s.model.Current().Detail = &media.Item{Type: kind}
		s.controller.running = kind != "Photo"
		s.handleKey(control.About)
		if s.about.Visible {
			t.Fatalf("About opened over %s", kind)
		}
	}
	s := testSession(t)
	s.controller.running = false
	s.media.pending = true
	s.handleKey(control.About)
	if s.about.Visible {
		t.Fatal("About opened during media handoff")
	}
}

func TestUpdateCheckIsAsyncAndSurvivesAboutClose(t *testing.T) {
	s := testSession(t)
	s.controller.running = false
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.ctx = ctx
	entered := make(chan struct{}, 2)
	gate := make(chan struct{})
	s.config.CheckUpdate = func(ctx context.Context) (release.Status, error) {
		entered <- struct{}{}
		select {
		case <-gate:
			return release.Status{Latest: "v1.0.0", Available: true}, nil
		case <-ctx.Done():
			return release.Status{}, ctx.Err()
		}
	}
	s.handleKey(control.About)
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("check not started")
	}
	s.handleKey(control.Select)
	s.handleKey(control.Back)
	if s.about.Visible || !s.about.Checking {
		t.Fatal("close canceled the release check")
	}
	select {
	case <-entered:
		t.Fatal("duplicate check")
	default:
	}
	close(gate)
	select {
	case result := <-s.events:
		s.handleResult(result)
	case <-time.After(3 * time.Second):
		t.Fatal("closed About lost the release result")
	}
	if s.about.Visible || s.about.Checking || !s.about.Release.Available {
		t.Fatal("release result did not update the hidden page")
	}
}

func TestUpdateCheckMinimumFeedbackDuration(t *testing.T) {
	for _, delay := range []time.Duration{0, 1500 * time.Millisecond} {
		for _, fails := range []bool{false, true} {
			synctest.Test(t, func(t *testing.T) {
				s := testSession(t)
				s.about.Visible = true
				s.config.CheckUpdate = func(context.Context) (release.Status, error) {
					time.Sleep(delay)
					if fails {
						return release.Status{}, errors.New("offline")
					}
					return release.Status{Latest: "v2.0.0", Available: true}, nil
				}
				started := time.Now()
				s.checkUpdate()
				synctest.Wait()
				time.Sleep(999 * time.Millisecond)
				synctest.Wait()
				select {
				case <-s.events:
					t.Fatal("update feedback ended before one second")
				default:
				}
				result := <-s.events
				if elapsed := time.Since(started); elapsed != max(time.Second, delay) {
					t.Fatalf("feedback duration = %v, want %v", elapsed, max(time.Second, delay))
				}
				s.handleResult(result)
				if s.about.Checking || !s.about.Checked {
					t.Fatal("check did not finish")
				}
			})
		}
	}
}

func TestUpdateCheckFeedbackWaitCancelsOnExit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := testSession(t)
		ctx, cancel := context.WithCancel(context.Background())
		s.ctx = ctx
		s.config.CheckUpdate = func(context.Context) (release.Status, error) { return release.Status{}, nil }
		s.checkUpdate()
		synctest.Wait()
		cancel()
		synctest.Wait()
		select {
		case <-s.events:
			t.Fatal("canceled check published a result")
		default:
		}
	})
}
