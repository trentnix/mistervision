package browser

import (
	"context"
	"log/slog"
	"time"

	"mistervision/internal/diagnostics"
	"mistervision/internal/input/control"
	"mistervision/internal/media"
	"mistervision/internal/rendering"
)

// postPlay owns the optional next-episode lookup and completion prompt. Decoder
// replacements keep the lookup. A new item, account, or dismissal invalidates it.
type postPlay struct {
	cancel           context.CancelFunc
	generation       int
	itemID           string
	pending, visible bool
	selected         int
	sequence         episodeSequence
	err              error
}

func (p *postPlay) close() {
	if p.cancel != nil {
		p.cancel()
	}
	*p = postPlay{generation: p.generation + 1}
}

// preparePostPlay resolves the next episode while playback proceeds. Explicit
// playlists and remote queues retain ownership of their existing advancement.
func (s *browserSession) preparePostPlay(item media.Item) {
	s.postPlay.close()
	if item.Type != "Episode" || s.client == nil || s.playbackQueue.active || s.playlistPlayback() {
		return
	}
	parent, ok := s.model.Parent()
	if !ok {
		return
	}
	p := &s.postPlay
	p.itemID, p.pending = item.ID, true
	if parent.Location.Kind == "episodes" {
		p.sequence.episodes = parent
	}
	work, cancel := context.WithTimeout(s.ctx, 10*time.Second)
	p.cancel = cancel
	generation, account, client, rows := p.generation, s.connection.generation, s.client, s.model.Rows
	go func() {
		defer cancel()
		sequence, err := findEpisodeSequence(work, client, item, parent, rows)
		s.send(s.ctx, postPlayResult{generation: generation, account: account, sequence: sequence, err: err})
	}()
}

type postPlayResult struct {
	generation, account int
	sequence            episodeSequence
	err                 error
}

func (r postPlayResult) apply(s *browserSession) bool {
	p := &s.postPlay
	if r.generation != p.generation || r.account != s.connection.generation || p.itemID == "" {
		return false
	}
	p.pending, p.err = false, r.err
	p.sequence = r.sequence
	// Keep Back selected if it was already visible during a slow lookup.
	if p.visible && r.sequence.next != nil {
		p.selected = 1
	}
	s.config.Diagnostics.Record("browser.next-episode", slog.Bool("available", r.sequence.next != nil), slog.String("error_kind", diagnostics.ErrorKind(r.err)))
	if p.visible {
		s.loadSelection()
	}
	return p.visible
}

// showPostPlay is called only after the controller accepts real item completion.
// Cancellation, playback failure, seeks, and explicit queues never open a prompt.
func (s *browserSession) showPostPlay(event PlaybackEvent) bool {
	p := &s.postPlay
	if p.itemID == "" || p.itemID != s.controller.item.ID || s.controller.stoppedByUser || event.Err != nil || s.playbackQueue.active || s.playlistPlayback() {
		p.close()
		return false
	}
	p.visible = true
	s.config.Diagnostics.Record("browser.post-play", slog.Bool("pending", p.pending), slog.Bool("available", p.sequence.next != nil))
	s.selection.key = ""
	s.loadSelection()
	return true
}

// postPlayPresentation supplies a preview of the next episode. It offers no countdown
// and keeps the next episode's identity visible before the user chooses to play.
func (s *browserSession) postPlayPresentation() *rendering.PostPlayPresentation {
	p := &s.postPlay
	if !p.visible {
		return nil
	}
	back := "Back"
	if p.sequence.episodes.Location.Kind == "episodes" {
		back = "Back to episodes"
	}
	menu := &rendering.PostPlayPresentation{Title: "Episode finished", Rows: []string{back}, Selected: p.selected}
	switch {
	case p.pending:
		menu.Message = messageNextEpisodeLoading
	case p.err != nil:
		menu.Message = messageNextEpisodeFailed
	case p.sequence.next == nil:
		menu.Message = messageNoNextEpisode
	default:
		item := p.sequence.next.item
		menu.Title = "Up next"
		menu.Rows = []string{"Play next episode", back}
		menu.Next = &item
	}
	return menu
}

func (s *browserSession) handlePostPlayKey(key control.Action) bool {
	if key.IsRepeat() && key.Base() != control.Up && key.Base() != control.Down {
		return false
	}
	p := &s.postPlay
	switch key.Base() {
	case control.Up:
		p.selected = 0
	case control.Down:
		if p.sequence.next != nil && !p.pending && p.err == nil {
			p.selected = 1
		}
	case control.Back:
		s.leavePostPlay(false)
	case control.Open:
		s.leavePostPlay(p.sequence.next != nil && !p.pending && p.err == nil && p.selected == 0)
	default:
		return false
	}
	return true
}

// leavePostPlay restores an episode list with the completed item selected, or
// starts its successor at zero. Continue Watching keeps its own parent screen.
func (s *browserSession) leavePostPlay(play bool) {
	sequence := s.postPlay.sequence
	s.postPlay.close()
	if play {
		s.selectEpisode(sequence.next.parent, &sequence.next.item)
		s.selection.key = ""
		s.loadSelection()
		zero := int64(0)
		s.startPlayback(&zero, false)
	} else {
		s.selectEpisode(sequence.episodes, nil)
		s.selection.key = ""
		s.loadSelection()
	}
}

// selectEpisode installs a resolved season page without accumulating detail
// screens. From Continue Watching it retains that feed beneath the episode list.
func (s *browserSession) selectEpisode(episodes View, item *media.Item) {
	s.requests.cancel()
	s.model.Generation++
	if episodes.Location.Kind != "episodes" {
		s.model.ReturnToParent()
		return
	}
	parent, ok := s.model.Parent()
	if ok && parent.Location.Kind == "episodes" {
		s.model.ReturnToParent()
	}
	for i := range s.model.Stack {
		view := &s.model.Stack[i]
		if view.Location.Kind != "seasons" || view.Location.SeriesID != episodes.Location.SeriesID {
			continue
		}
		for row, season := range view.Page.Items {
			if season.ID == episodes.Location.ParentID {
				view.Selected, view.Target = row, view.Start+row
				view.centerSelection(s.model.Rows)
				break
			}
		}
	}
	episodes.Loading, episodes.fetching = false, false
	episodes.Error = ""
	*s.model.Current() = episodes
	if item != nil {
		copy := *item
		copy.ContinueAction = ""
		s.model.Stack = append(s.model.Stack, View{Title: item.Name, Detail: &copy, Location: media.Location{Kind: "items", ParentID: item.ID, SeriesID: episodes.Location.SeriesID, Collection: episodes.Location.Collection}})
	}
	s.model.Notice = ""
}
