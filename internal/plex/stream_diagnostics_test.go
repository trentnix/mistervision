package plex

import (
	"context"
	"encoding/json"
	"fmt"
	"mistervision/internal/serverstate"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestDecisionDiagnosticsDoNotInventServerDecision(t *testing.T) {
	for _, metadata := range []string{`[{"Media":[{"Part":[{"Stream":[{"streamType":1,"decision":"transcode"},{"streamType":2,"decision":"copy"}]}]}]}]`, `[{"Media":[{"videoDecision":"transcode","audioDecision":"copy"}]}]`, `[]`, `{"invalid":"private"}`} {
		t.Run(metadata, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, `{"MediaContainer":{"generalDecisionCode":1001,"Metadata":%s}}`, metadata)
			}))
			defer server.Close()
			client := NewClient(Config{Server: server.URL}, serverstate.Session{})
			got, err := client.decideVideo(context.Background(), url.Values{})
			if err != nil {
				t.Fatal(err)
			}
			if got.RequestedMethod != "transcode" {
				t.Fatal(got)
			}
			if metadata[0] == '[' && len(metadata) > 2 {
				if got.VideoDecision != "transcode" || got.AudioDecision != "copy" {
					t.Fatal(got)
				}
			} else if got.VideoDecision != "" || got.AudioDecision != "" {
				t.Fatal("invented decision", got)
			}
		})
	}
}

func TestSourceVideoMetadata(t *testing.T) {
	var m metadata
	if err := json.Unmarshal([]byte(`{"ratingKey":"1","type":"movie","Media":[{"Width":3840,"Height":2160,"Part":[{"id":2,"Stream":[{"id":3,"streamType":1,"codec":"hevc","frameRate":"23.976","bitrate":40000}]}]}]}`), &m); err != nil {
		t.Fatal(err)
	}
	item := m.item()
	s := item.MediaSources[0].MediaStreams[0]
	if s.Width != 3840 || s.Height != 2160 || s.RealFrameRate != 23.976 || s.BitRate != 40000000 {
		t.Fatalf("source metadata: %+v", s)
	}
}
