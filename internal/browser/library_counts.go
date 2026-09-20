package browser

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"mistervision/internal/diagnostics"
	"mistervision/internal/media"
	"mistervision/internal/rendering"
)

const libraryCountConcurrency = 3

// libraryCountState owns account-scoped work shared by both home views.
// Only the browser loop accesses these fields. Selection changes never cancel it.
type libraryCountState struct {
	ctx        context.Context
	cancel     context.CancelFunc
	generation int
	pending    map[string]bool
	retryAfter map[string]time.Time
}

func (c *libraryCountState) reset() {
	if c.cancel != nil {
		c.cancel()
	}
	*c = libraryCountState{generation: c.generation + 1}
}

// loadLibraryCounts fills free request slots, prioritizing the selected library
// and visible rows. Cache entries and failed attempts have a one-minute lifetime.
func (s *browserSession) loadLibraryCounts() {
	if s.client == nil || s.selection.loader == nil || s.connection.forgetting || s.setup.Kind != rendering.SetupHidden {
		return
	}
	root := &s.model.Stack[0]
	if root.Location.Kind != "views" {
		return
	}
	c := &s.counts
	if c.ctx == nil {
		c.ctx, c.cancel = context.WithCancel(s.ctx)
		c.pending = make(map[string]bool)
		c.retryAfter = make(map[string]time.Time)
	}
	now := time.Now()
	order := []int{root.Selected}
	for i := root.Scroll; i < min(len(root.Page.Items), root.Scroll+s.model.rowsFor(root)); i++ {
		order = append(order, i)
	}
	for i := range root.Page.Items {
		order = append(order, i)
	}
	for _, index := range order {
		if index < 0 || index >= len(root.Page.Items) {
			continue
		}
		item := root.Page.Items[index]
		if item.ID == continueID || c.pending[item.ID] {
			continue
		}
		cached := s.selection.loader.libraries.cached(item.ID)
		if now.Before(cached.countUntil) {
			s.setLibraryCount(item.ID, cached.count)
			continue
		}
		if now.Before(c.retryAfter[item.ID]) {
			continue
		}
		if len(c.pending) >= libraryCountConcurrency {
			return
		}
		c.pending[item.ID] = true
		c.retryAfter[item.ID] = now.Add(libraryCacheTTL)
		client, ctx, generation := s.client, c.ctx, c.generation
		go func() {
			work, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			count, err := client.LibraryCount(work, item)
			s.send(ctx, libraryCountResult{generation: generation, id: item.ID, count: count, err: err})
		}()
	}
}

// setLibraryCount replaces the borrowed home slice without moving selection.
func (s *browserSession) setLibraryCount(id string, count *int) {
	root := &s.model.Stack[0]
	for i, item := range root.Page.Items {
		if item.ID == id {
			if item.LibraryCount == count {
				return
			}
			items := append([]media.Item(nil), root.Page.Items...)
			items[i].LibraryCount = count
			root.Page.Items = items
			return
		}
	}
}

type libraryCountResult struct {
	generation int
	id         string
	count      *int
	err        error
}

func (r libraryCountResult) apply(s *browserSession) bool {
	if r.generation != s.counts.generation || !s.counts.pending[r.id] {
		return false
	}
	delete(s.counts.pending, r.id)
	if errors.Is(r.err, media.ErrUnauthorized) {
		s.requireSignIn(r.err)
		s.counts.reset()
		return true
	}
	if r.err != nil {
		s.config.Diagnostics.Record("browser.count", slog.Bool("failed", true), slog.String("error_kind", diagnostics.ErrorKind(r.err)))
	} else {
		s.selection.loader.libraries.remember(r.id, func(c *cachedLibrary) { c.count = r.count; c.countUntil = time.Now().Add(libraryCacheTTL) })
		s.setLibraryCount(r.id, r.count)
	}
	s.loadLibraryCounts()
	return true
}
