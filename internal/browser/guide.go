package browser

import (
	"context"
	"log/slog"
	"time"

	"mistervision/internal/connection"
	"mistervision/internal/diagnostics"
	"mistervision/internal/media"
)

// guideState retains the most recent channel page's schedule for this connection.
// Leaving the list cancels work but keeps listings for an immediate return.
// The browser loop serializes state; authentication discards the cache.
type guideState struct {
	active     bool
	channels   []string
	generation int
	connection int
	cancel     context.CancelFunc
	refresh    time.Time
	programs   map[string][]media.Program
}

func (g *guideState) reset() {
	if g.cancel != nil {
		g.cancel()
	}
	*g = guideState{generation: g.generation + 1}
}

// suspend invalidates pending work without discarding the last successful guide.
// Returning to the list must refresh even within the ordinary refresh interval.
func (g *guideState) suspend() {
	if !g.active {
		return
	}
	if g.cancel != nil {
		g.cancel()
	}
	g.cancel = nil
	g.active = false
	g.generation++
	g.refresh = time.Time{}
}

// matches compares the loaded channels without allocating during each frame.
func (g *guideState) matches(items []media.Item) bool {
	index := 0
	for _, item := range items {
		if !media.IsLive(item) {
			continue
		}
		if index == len(g.channels) || g.channels[index] != item.ID {
			return false
		}
		index++
	}
	return index == len(g.channels)
}

// refreshGuide runs during presentation but never performs network I/O there.
// Guide failures are optional metadata failures, not channel playback failures.
func (s *browserSession) refreshGuide(now time.Time) {
	g := &s.guide
	if g.connection != s.connection.generation {
		g.reset()
		g.connection = s.connection.generation
	}
	v := s.model.Current()
	provider, supported := s.client.(media.ProgramGuide)
	if !supported || v.Detail != nil || s.controller.running || s.setup.Kind != connection.SetupHidden || v.Location.Kind != "livetv" || len(v.Page.Items) == 0 {
		g.suspend()
		return
	}
	if !g.matches(v.Page.Items) {
		previous := g.programs
		g.reset()
		g.connection = s.connection.generation
		for _, item := range v.Page.Items {
			if !media.IsLive(item) {
				continue
			}
			g.channels = append(g.channels, item.ID)
			programs, ok := previous[item.ID]
			if !ok {
				continue
			}
			if g.programs == nil {
				g.programs = make(map[string][]media.Program)
			}
			g.programs[item.ID] = programs
		}
	}
	if len(g.channels) == 0 {
		g.suspend()
		return
	}
	g.active = true
	if now.Before(g.refresh) {
		return
	}
	if g.cancel != nil {
		g.cancel()
	}
	g.generation++
	generation := g.generation
	connection := g.connection
	ids := g.channels // Immutable snapshot shared with the worker.
	g.refresh = now.Add(time.Minute)
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	g.cancel = cancel
	go func() {
		defer cancel()
		// Do not start a network request for each page crossed during held scrolling.
		timer := time.NewTimer(150 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		programs, err := provider.Programs(ctx, ids, now, now.Add(6*time.Hour))
		s.send(s.ctx, guideResult{generation: generation, connection: connection, programs: programs, err: err})
	}()
}

type guideResult struct {
	generation, connection int
	programs               []media.Program
	err                    error
}

func (r guideResult) apply(s *browserSession) bool {
	g := &s.guide
	if r.generation != g.generation || r.connection != s.connection.generation {
		return false
	}
	s.config.Diagnostics.Record("browser.guide", slog.Bool("failed", r.err != nil), slog.String("error_kind", diagnostics.ErrorKind(r.err)))
	if r.err != nil {
		return false
	}
	g.programs = make(map[string][]media.Program)
	for _, p := range r.programs {
		g.programs[p.ChannelID] = append(g.programs[p.ChannelID], p)
	}
	return true
}
