package media

import (
	"cmp"
	"context"
	"slices"
)

// SegmentKind identifies server-detected content that a viewer can skip.
type SegmentKind string

const (
	SegmentIntro      SegmentKind = "intro"
	SegmentCredits    SegmentKind = "credits"
	SegmentRecap      SegmentKind = "recap"
	SegmentPreview    SegmentKind = "preview"
	SegmentCommercial SegmentKind = "commercial"
)

// Segment describes an interval in the original video's timeline, in 100-nanosecond
// ticks. StartTicks is inclusive and EndTicks is exclusive. Credits can end before
// a post-credits scene, so clients must preserve the supplied endpoint.
type Segment struct {
	Kind                 SegmentKind
	StartTicks, EndTicks int64
}

// SegmentSource optionally supplies segments for on-demand video. An empty result
// means no skip actions are available. Implementations do not analyze media files.
type SegmentSource interface {
	Segments(context.Context, string) ([]Segment, error)
}

// ValidSegments copies, bounds, sorts, and deduplicates provider metadata. Unknown
// types and invalid intervals are discarded. Overlaps stay separate: merging them
// could skip content that belongs to neither intended action. Overlapping intervals
// with the earliest endpoint come first to offer the least destructive skip.
func ValidSegments(segments []Segment, duration int64) []Segment {
	var result []Segment
	for _, s := range segments {
		switch s.Kind {
		case SegmentIntro, SegmentCredits, SegmentRecap, SegmentPreview, SegmentCommercial:
		default:
			continue
		}
		if s.StartTicks < 0 {
			continue
		}
		if duration > 0 {
			s.EndTicks = min(s.EndTicks, duration)
		}
		if s.EndTicks <= s.StartTicks {
			continue
		}
		result = append(result, s)
	}
	slices.SortFunc(result, func(a, b Segment) int {
		return cmp.Or(cmp.Compare(a.EndTicks, b.EndTicks), cmp.Compare(a.StartTicks, b.StartTicks), cmp.Compare(a.Kind, b.Kind))
	})
	return slices.Compact(result)
}
