package media

import (
	"context"
	"errors"
	"time"
)

// Program is one scheduled airing. ChannelID uses the same opaque identity as
// the channel catalog. Start is inclusive and End is exclusive.
type Program struct {
	ChannelID, Title string
	Start, End       time.Time
}

// ProgramGuide is an optional catalog capability. Programs returns owned records
// overlapping [start, end) for the requested channel IDs. Empty results mean no
// listings. Errors must not prevent browsing or tuning. Implementations honor
// cancellation and bound response size. Times are absolute, not display strings.
type ProgramGuide interface {
	Programs(context.Context, []string, time.Time, time.Time) ([]Program, error)
}

// ValidateGuideRequest bounds guide work independently of backend query syntax.
func ValidateGuideRequest(channels []string, start, end time.Time) error {
	if len(channels) == 0 || len(channels) > 200 || start.IsZero() || !end.After(start) || end.Sub(start) > 24*time.Hour {
		return errors.New("invalid program guide window")
	}
	for _, id := range channels {
		if id == "" {
			return errors.New("missing guide channel")
		}
	}
	return nil
}

// CurrentNext selects valid current and upcoming airings regardless of server
// ordering. At a boundary, the new airing becomes current. Overlapping current
// airings prefer the latest start; Next never overlaps the chosen current airing.
func CurrentNext(programs []Program, now time.Time) (current, next *Program) {
	for i := range programs {
		p := &programs[i]
		if p.Title == "" || !p.End.After(p.Start) {
			continue
		}
		if !p.Start.After(now) && p.End.After(now) && (current == nil || p.Start.After(current.Start)) {
			current = p
		}
	}
	after := now
	if current != nil {
		after = current.End
	}
	for i := range programs {
		p := &programs[i]
		if p.Title == "" || !p.End.After(p.Start) || !p.Start.After(now) || p.Start.Before(after) {
			continue
		}
		if next == nil || p.Start.Before(next.Start) {
			next = p
		}
	}
	return
}
