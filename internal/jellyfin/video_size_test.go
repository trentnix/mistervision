package jellyfin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"mistervision/internal/media"
)

func TestDisplaySizedVideoAndLiveRequests(t *testing.T) {
	for _, tc := range []struct {
		size media.VideoSize
		cap  TranscodeProfile
		w, h int
	}{
		{media.VideoSize{Width: 320, Height: 240}, TranscodeProfile{}, 320, 240},
		{media.VideoSize{Width: 640, Height: 360}, TranscodeProfile{}, 640, 360},
		{media.VideoSize{Width: 640, Height: 480}, TranscodeProfile{}, 640, 480},
		{media.VideoSize{Width: 1280, Height: 720}, TranscodeProfile{}, 1280, 720},
		{media.VideoSize{Width: 320, Height: 240}, TranscodeProfile{MaxWidth: 720, MaxHeight: 576}, 320, 240},
		{media.VideoSize{Width: 1280, Height: 720}, TranscodeProfile{MaxWidth: 640, MaxHeight: 480}, 640, 480},
	} {
		t.Run(fmt.Sprintf("%dx%d_cap_%dx%d", tc.size.Width, tc.size.Height, tc.cap.MaxWidth, tc.cap.MaxHeight), func(t *testing.T) {
			seen := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/Items/channel/PlaybackInfo" {
					t.Errorf("unexpected request %s", r.URL.Path)
					return
				}
				var request struct {
					EnableDirectPlay, EnableDirectStream bool
					DeviceProfile                        struct {
						CodecProfiles []struct {
							Conditions []struct{ Property, Value string }
						}
					}
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					return
				}
				limits := map[string]string{}
				for _, p := range request.DeviceProfile.CodecProfiles {
					for _, c := range p.Conditions {
						limits[c.Property] = c.Value
					}
				}
				if limits["Width"] != strconv.Itoa(tc.w) || limits["Height"] != strconv.Itoa(tc.h) || request.EnableDirectPlay || request.EnableDirectStream {
					t.Errorf("incorrect tuner profile: %+v", request)
				}
				seen = true
				fmt.Fprintf(w, `{"PlaySessionId":"session","MediaSources":[{"Id":"source","LiveStreamId":"tuner","TranscodingUrl":"/Videos/channel/stream.ts?maxWidth=%d&maxHeight=%d"}]}`, tc.w, tc.h)
			}))
			defer server.Close()
			c := NewClient(Config{Server: server.URL, Transcode: tc.cap}, Session{})
			video, err := c.PrepareVideo(t.Context(), media.VideoRequest{Size: tc.size, Item: media.Item{ID: "movie"}, BurnSubtitle: -1})
			if err != nil {
				t.Fatal(err)
			}
			u, _ := url.Parse(video.URL)
			if u.Query().Get("maxWidth") != strconv.Itoa(tc.w) || u.Query().Get("maxHeight") != strconv.Itoa(tc.h) || video.Limits.MaxWidth != float64(tc.w) {
				t.Fatalf("incorrect recorded-video limits: %+v", video.Limits)
			}
			live, err := c.PrepareLive(t.Context(), media.LiveRequest{Size: tc.size, ChannelID: "channel", MaxFrameRate: 30, AudioIndex: -1})
			if err != nil {
				t.Fatal(err)
			}
			if !seen || live.Limits.MaxWidth != float64(tc.w) || live.Limits.MaxHeight != float64(tc.h) {
				t.Fatalf("incorrect live limits: %+v", live.Limits)
			}
			if c.Config.Transcode != tc.cap {
				t.Fatal("request mutated shared configuration")
			}
		})
	}
}
