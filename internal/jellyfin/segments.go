package jellyfin

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"

	"mistervision/internal/media"
)

// Segments reads Jellyfin's media-segment API (10.10 and later). Older servers
// return no actions. Segment-provider plugins populate these records on the server.
func (c *Client) Segments(ctx context.Context, id string) ([]media.Segment, error) {
	if id == "" {
		return nil, errors.New("missing Jellyfin item ID")
	}
	data, err := c.request(ctx, "GET", "/MediaSegments/"+url.PathEscape(id), nil, nil)
	if errors.Is(err, media.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var response struct {
		Items []struct {
			Type                 string
			StartTicks, EndTicks int64
		}
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}
	kinds := map[string]media.SegmentKind{
		"Intro": media.SegmentIntro, "Outro": media.SegmentCredits,
		"Recap": media.SegmentRecap, "Preview": media.SegmentPreview,
		"Commercial": media.SegmentCommercial,
	}
	var segments []media.Segment
	for _, item := range response.Items {
		segments = append(segments, media.Segment{Kind: kinds[item.Type], StartTicks: item.StartTicks, EndTicks: item.EndTicks})
	}
	return media.ValidSegments(segments, 0), nil
}
