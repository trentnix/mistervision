package plex

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"

	"mistervision/internal/media"
	"mistervision/internal/plex/companion"
	"mistervision/internal/remote"
)

// remoteSource binds LAN control to this immutable server and viewer session.
// Listener failures leave ordinary browsing and playback available.
func (c *Client) remoteSource() remote.Source {
	version := c.Version
	if version == "" {
		version = "dev"
	}
	source, err := companion.New(companion.Config{
		Identifier: c.Session.DeviceID, Name: "MiSTerVision", Version: version, Listen: ":32433",
		Control: &companion.ControlConfig{ServerID: c.Session.ServerID, Authorize: c.authorizeController},
		Observe: func(e companion.Event) {
			if e.Operation == "/player/timeline/poll" && e.Status == 200 {
				// Successful idle polls are continuous and need no disk logging.
				return
			}
			c.Diagnostics.Record("plex.companion", slog.String("operation", e.Operation), slog.Int("status", e.Status), slog.Int64("command", e.CommandID))
		},
	})
	if err != nil {
		c.Diagnostics.Record("plex.companion-unavailable")
		return nil
	}
	return source
}

// authorizeController verifies an account token with Plex, not with a controller
// supplied address. Matching the active viewer prevents cross-Home-profile control.
// Transient playback tokens are deliberately not promoted to account credentials.
func (c *Client) authorizeController(ctx context.Context, token string) error {
	if token == "" || c.Session.UserID == "" {
		return media.ErrUnauthorized
	}
	data, _, err := c.fetch(ctx, c.accountHTTP, c.accountURL, token, "GET", "/api/v2/user", nil)
	if err != nil {
		return err
	}
	var user accountIdentity
	if json.Unmarshal(data, &user) != nil || user.ID <= 0 || strconv.Itoa(user.ID) != c.Session.UserID {
		return media.ErrUnauthorized
	}
	return nil
}

// RemoteItems resolves only individual movies and episodes for the first Plex
// control milestone. The active viewer's credential enforces library access.
// Plex queue expansion and music control are separate future capabilities.
func (c *Client) RemoteItems(ctx context.Context, ids []string, mix bool) ([]media.Item, error) {
	if mix || len(ids) != 1 {
		return nil, errors.New("Plex remote control requires one video")
	}
	item, err := c.Details(ctx, ids[0])
	if err != nil {
		return nil, err
	}
	if item.Type != "Movie" && item.Type != "Episode" {
		return nil, errors.New("Plex remote control supports movies and episodes")
	}
	return []media.Item{item}, nil
}
