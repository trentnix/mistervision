package jellyfin

import (
	"context"
	"errors"
	"net/url"
)

// SetWatched uses Jellyfin's explicit played-state actions for this user. Both
// actions clear resume position and update the server's play history consistently.
func (c *Client) SetWatched(ctx context.Context, id string, watched bool) error {
	if id == "" {
		return errors.New("missing Jellyfin item ID")
	}
	method := "DELETE"
	if watched {
		method = "POST"
	}
	_, err := c.request(ctx, method, "/UserPlayedItems/"+url.PathEscape(id), url.Values{"userId": {c.Session.UserID}}, nil)
	return err
}
