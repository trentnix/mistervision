package jellyfin

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mistervision/internal/media"
)

func TestSegments(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/MediaSegments/item%2Fid" || !strings.Contains(r.Header.Get("Authorization"), `Token="private"`) {
			t.Errorf("incorrect segment request: %s", r.URL)
		}
		fmt.Fprint(w, `{"Items":[{"Type":"Intro","StartTicks":0,"EndTicks":900000000},{"Type":"Outro","StartTicks":8000000000,"EndTicks":9000000000},{"Type":"Recap","StartTicks":900000000,"EndTicks":1000000000},{"Type":"Preview","StartTicks":1000000000,"EndTicks":1100000000},{"Type":"Commercial","StartTicks":1200000000,"EndTicks":1300000000},{"Type":"Unknown","StartTicks":0,"EndTicks":1000},{"Type":"Intro","StartTicks":20,"EndTicks":10}]}`)
	}))
	defer srv.Close()
	c := NewClient(Config{Server: srv.URL}, Session{Token: "private"})
	got, err := c.Segments(t.Context(), "item/id")
	if err != nil || len(got) != 5 {
		t.Fatalf("segments: %+v, %v", got, err)
	}
	if got[0].EndTicks != 900000000 || got[4].Kind != media.SegmentCredits || got[4].EndTicks != 9000000000 {
		t.Fatalf("incorrect units or outro: %+v", got)
	}
}

func TestSegmentErrors(t *testing.T) {
	for _, status := range []int{200, 404, 401, 500} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status); fmt.Fprint(w, `{"Items":[]}`) }))
		c := NewClient(Config{Server: srv.URL}, Session{})
		got, err := c.Segments(t.Context(), "item")
		if len(got) != 0 || (err != nil) != (status == 401 || status == 500) {
			t.Errorf("status %d: %+v %v", status, got, err)
		}
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if _, err := c.Segments(ctx, "item"); !errors.Is(err, context.Canceled) {
			t.Error(err)
		}
		srv.Close()
	}
}
