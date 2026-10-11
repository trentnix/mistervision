package plex

import (
	"context"
	"errors"
	"net/url"
)

// SetWatched changes this user's watched state. Plex's scrobble endpoints also
// reset the resume point. Ordinary playback reports retain their own lifecycle.
func (c *Client) SetWatched(ctx context.Context, id string, watched bool) error {
	if !validID(id) {
		return errors.New("invalid Plex item ID")
	}
	path := "/:/unscrobble"
	if watched {
		path = "/:/scrobble"
	}
	_, err := c.request(ctx, "PUT", path, url.Values{"key": {id}, "identifier": {"com.plexapp.plugins.library"}})
	return err
}
