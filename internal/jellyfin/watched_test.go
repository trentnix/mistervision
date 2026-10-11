package jellyfin

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mistervision/internal/media"
)

func TestSetWatched(t *testing.T) {
	for _, watched := range []bool{true, false} {
		t.Run(map[bool]string{true: "watched", false: "unwatched"}[watched], func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				method := "DELETE"
				if watched {
					method = "POST"
				}
				if r.Method != method || r.URL.EscapedPath() != "/UserPlayedItems/item%2Fid" || r.URL.Query().Get("userId") != "viewer" || !strings.Contains(r.Header.Get("Authorization"), `Token="private"`) {
					t.Errorf("incorrect authenticated action: %s %s", r.Method, r.URL)
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			defer srv.Close()
			c := NewClient(Config{Server: srv.URL}, Session{UserID: "viewer", Token: "private"})
			if err := c.SetWatched(t.Context(), "item/id", watched); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSetWatchedRejectsUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusUnauthorized) }))
	defer srv.Close()
	c := NewClient(Config{Server: srv.URL}, Session{})
	if err := c.SetWatched(t.Context(), "item", true); !errors.Is(err, media.ErrUnauthorized) {
		t.Fatal(err)
	}
}
