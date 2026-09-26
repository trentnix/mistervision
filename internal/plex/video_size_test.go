package plex

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"mistervision/internal/media"
)

func TestDisplaySizedRecordedVideo(t *testing.T) {
	for _, tc := range []struct {
		size   media.VideoSize
		cw, ch int
		want   string
	}{
		{media.VideoSize{Width: 320, Height: 240}, 0, 0, "320x240"},
		{media.VideoSize{Width: 640, Height: 360}, 0, 0, "640x360"},
		{media.VideoSize{Width: 640, Height: 480}, 0, 0, "640x480"},
		{media.VideoSize{Width: 1280, Height: 720}, 0, 0, "1280x720"},
		{media.VideoSize{Width: 320, Height: 240}, 720, 576, "320x240"},
		{media.VideoSize{Width: 1280, Height: 720}, 640, 480, "640x480"},
	} {
		t.Run(tc.want+fmt.Sprint(tc.cw, tc.ch), func(t *testing.T) {
			seen := false
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/library/parts/8":
				case "/video/:/transcode/universal/decision":
					q := r.URL.Query()
					if q.Get("videoResolution") != tc.want || q.Get("directPlay") != "0" || q.Get("directStream") != "0" {
						t.Errorf("incorrect conversion limits")
					}
					seen = true
					fmt.Fprint(w, `{"MediaContainer":{"generalDecisionCode":1001}}`)
				default:
					t.Errorf("unexpected path %s", r.URL.Path)
				}
			})
			c.Config.MaxWidth, c.Config.MaxHeight = tc.cw, tc.ch
			prepared, err := c.PrepareVideo(t.Context(), media.VideoRequest{Size: tc.size, Item: media.Item{ID: "42", MediaSources: []media.MediaSource{{ID: "0:8"}}}, SourceID: "0:8", SessionID: "test", Tracks: media.TrackSelection{AudioIndex: -1, SubtitleIndex: -1}, BurnSubtitle: -1})
			if err != nil {
				t.Fatal(err)
			}
			u, _ := url.Parse(prepared.URL)
			if !seen || u.Query().Get("videoResolution") != tc.want {
				t.Fatal("stream and decision dimensions differ")
			}
			if c.Config.MaxWidth != tc.cw || c.Config.MaxHeight != tc.ch {
				t.Fatal("request mutated shared configuration")
			}
		})
	}
}
