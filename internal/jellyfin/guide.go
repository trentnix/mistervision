package jellyfin

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"mistervision/internal/media"
)

var _ media.ProgramGuide = (*Client)(nil)

// Programs requests a bounded schedule without images or user playback data.
// Channel IDs and UTC boundaries are translated here, outside the shared UI.
func (c *Client) Programs(ctx context.Context, channels []string, start, end time.Time) ([]media.Program, error) {
	if err := media.ValidateGuideRequest(channels, start, end); err != nil {
		return nil, err
	}
	q := url.Values{
		"userId":                 {c.Session.UserID},
		"channelIds":             {strings.Join(channels, ",")},
		"minEndDate":             {start.UTC().Format(time.RFC3339Nano)},
		"maxStartDate":           {end.UTC().Format(time.RFC3339Nano)},
		"sortBy":                 {"StartDate"},
		"sortOrder":              {"Ascending"},
		"limit":                  {"2000"},
		"enableImages":           {"false"},
		"enableUserData":         {"false"},
		"enableTotalRecordCount": {"true"},
	}
	var response struct {
		Items []struct {
			ChannelID          string `json:"ChannelId"`
			Name               string
			StartDate, EndDate time.Time
		}
		TotalRecordCount int
	}
	if err := c.json(ctx, "GET", "/LiveTv/Programs", q, nil, &response); err != nil {
		return nil, err
	}
	if response.TotalRecordCount > 2000 || len(response.Items) > 2000 {
		return nil, errors.New("program guide exceeds window limit")
	}
	allowed := make(map[string]bool, len(channels))
	for _, id := range channels {
		allowed[id] = true
	}
	var result []media.Program
	for _, p := range response.Items {
		if allowed[p.ChannelID] && p.Name != "" && !p.StartDate.IsZero() && p.EndDate.After(p.StartDate) && p.EndDate.After(start) && p.StartDate.Before(end) {
			result = append(result, media.Program{ChannelID: p.ChannelID, Title: p.Name, Start: p.StartDate, End: p.EndDate})
		}
	}
	return result, nil
}
