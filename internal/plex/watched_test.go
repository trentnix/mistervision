package plex

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"mistervision/internal/media"
)

func TestSetWatched(t *testing.T) {
	for _, watched := range []bool{true, false} {
		path := "/:/unscrobble"
		if watched {
			path = "/:/scrobble"
		}
		c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "PUT" || r.URL.Path != path || r.URL.Query().Get("key") != "42" || r.URL.Query().Get("identifier") != "com.plexapp.plugins.library" {
				t.Errorf("wrong watched request: %s %s", r.Method, r.URL)
			}
			w.WriteHeader(http.StatusNoContent)
		})
		if err := c.SetWatched(t.Context(), "42", watched); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSetWatchedFailureAndCancellation(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusUnauthorized) })
	if err := c.SetWatched(t.Context(), "42", true); !errors.Is(err, media.ErrUnauthorized) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := c.SetWatched(ctx, "42", false); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := c.SetWatched(t.Context(), "../42", true); err == nil {
		t.Fatal("accepted invalid ID")
	}
}
