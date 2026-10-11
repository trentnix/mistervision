package plex

import (
	"context"
	"errors"
	"math"
	"net/url"

	"mistervision/internal/media"
)

// Segments requests analyzed intro, credits, and commercial markers. Plex returns
// these only when available for the item and account. Milliseconds are converted
// to the shared original-video timeline without extending final credits markers.
func (c *Client) Segments(ctx context.Context, id string) ([]media.Segment, error) {
	if !validID(id) {
		return nil, errors.New("invalid Plex item ID")
	}
	var response struct {
		Container *struct {
			Metadata []struct {
				ID      identifier `json:"ratingKey"`
				Markers []struct {
					Type  string `json:"type"`
					Start int64  `json:"startTimeOffset"`
					End   int64  `json:"endTimeOffset"`
				} `json:"Marker"`
			} `json:"Metadata"`
		} `json:"MediaContainer"`
	}
	if err := c.json(ctx, "/library/metadata/"+id, url.Values{"includeMarkers": {"1"}}, &response); err != nil {
		return nil, err
	}
	if response.Container == nil || len(response.Container.Metadata) != 1 || string(response.Container.Metadata[0].ID) != id {
		return nil, errors.New("invalid Plex segment metadata")
	}
	var segments []media.Segment
	for _, m := range response.Container.Metadata[0].Markers {
		if m.Start < 0 || m.End < 0 || m.Start > math.MaxInt64/10000 || m.End > math.MaxInt64/10000 {
			continue
		}
		segments = append(segments, media.Segment{Kind: media.SegmentKind(m.Type), StartTicks: m.Start * 10000, EndTicks: m.End * 10000})
	}
	return media.ValidSegments(segments, 0), nil
}
