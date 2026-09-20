package jellyfin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProgramsWindowAndChannelScope(t *testing.T) {
	now := time.Date(2026, 9, 20, 0, 0, 0, 0, time.FixedZone("test", -5*3600))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/LiveTv/Programs" || q.Get("channelIds") != "a,b" || q.Get("userId") != "viewer" || q.Get("minEndDate") != now.UTC().Format(time.RFC3339Nano) || q.Get("limit") != "2000" {
			t.Errorf("unexpected guide request: %v", q)
		}
		json.NewEncoder(w).Encode(map[string]any{"Items": []map[string]any{
			{"ChannelId": "a", "Name": "Current", "StartDate": now.Add(-time.Hour), "EndDate": now.Add(time.Hour)},
			{"ChannelId": "b", "Name": "Next", "StartDate": now.Add(time.Hour), "EndDate": now.Add(2 * time.Hour)},
			{"ChannelId": "other", "Name": "Wrong channel", "StartDate": now, "EndDate": now.Add(time.Hour)},
			{"ChannelId": "a", "Name": "Expired", "StartDate": now.Add(-time.Hour), "EndDate": now},
			{"ChannelId": "a", "Name": "Malformed", "StartDate": now, "EndDate": now},
		}})
	}))
	defer server.Close()
	c := NewClient(Config{Server: server.URL}, Session{UserID: "viewer"})
	got, err := c.Programs(t.Context(), []string{"a", "b"}, now, now.Add(6*time.Hour))
	if err != nil || len(got) != 2 || got[0].Title != "Current" || got[1].ChannelID != "b" {
		t.Fatalf("%+v %v", got, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err = c.Programs(ctx, []string{"a", "b"}, now, now.Add(time.Hour)); err == nil {
		t.Fatal("ignored cancellation")
	}
}

func TestProgramsRejectsIncompleteResponses(t *testing.T) {
	for _, body := range []string{
		`{"TotalRecordCount":2001,"Items":[]}`,
		`{"Items":`,
	} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
			defer server.Close()
			c := NewClient(Config{Server: server.URL}, Session{UserID: "viewer"})
			now := time.Now()
			if _, err := c.Programs(t.Context(), []string{"a"}, now, now.Add(time.Hour)); err == nil {
				t.Fatal("incomplete guide was accepted as a successful replacement")
			}
		})
	}
}
