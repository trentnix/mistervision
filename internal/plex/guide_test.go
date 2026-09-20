package plex

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestProgramsResolveChannelAliasesAndWindow(t *testing.T) {
	now := time.Unix(2000000000, 0)
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/livetv/dvrs":
			fmt.Fprint(w, testDVRs)
		case "/livetv/epg/channels":
			fmt.Fprint(w, `{"MediaContainer":{"Channel":[{"key":"two-two","identifier":"station-id"}]}}`)
		case "/tv.plex.providers.epg.cloud:2/grid":
			if r.URL.Query().Get("beginsAt<") != "2000021600" || r.URL.Query().Get("endsAt>") != "2000000000" {
				t.Errorf("wrong window %v", r.URL.Query())
			}
			fmt.Fprintf(w, `{"MediaContainer":{"Metadata":[{"title":"Episode","grandparentTitle":"Show","Media":[{"channelIdentifier":"station-id","beginsAt":%d,"endsAt":%d}]},{"title":"Next","Media":[{"channelIdentifier":"2.2","beginsAt":%d,"endsAt":%d}]},{"title":"Other","Media":[{"channelIdentifier":"ten","beginsAt":%d,"endsAt":%d}]}]}}`, now.Unix()-60, now.Unix()+60, now.Unix()+60, now.Unix()+120, now.Unix()-60, now.Unix()+60)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(404)
		}
	})
	id := liveID("2", "two-two")
	got, err := c.Programs(t.Context(), []string{id}, now, now.Add(6*time.Hour))
	if err != nil || len(got) != 2 || got[0].ChannelID != id || got[0].Title != "Show" || got[1].Title != "Next" {
		t.Fatalf("%+v %v", got, err)
	}
}
func TestProgramsMissingGuideAndFailure(t *testing.T) {
	for _, status := range []int{200, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				if status != 200 {
					w.WriteHeader(status)
					return
				}
				fmt.Fprint(w, `{"MediaContainer":{"Dvr":[{"key":"2"}]}}`)
			})
			now := time.Now()
			got, err := c.Programs(t.Context(), []string{liveID("2", "two-two")}, now, now.Add(time.Hour))
			if len(got) != 0 || (err != nil) != (status != 200) {
				t.Fatalf("%v %v", got, err)
			}
		})
	}
}

func TestProgramGridRejectsUnsupportedProviderWithoutRequest(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("unsupported guide provider reached the network")
		w.WriteHeader(http.StatusNotFound)
	})
	now := time.Now()
	for _, provider := range []string{"unknown:2", "tv.plex.providers.epg.cloud:../secret", "invalid"} {
		if _, err := c.programGrid(t.Context(), provider, now, now.Add(time.Hour)); err == nil {
			t.Errorf("provider %q was treated as an empty guide", provider)
		}
	}
}

func TestProgramGridRejectsIncompleteResponses(t *testing.T) {
	for _, body := range []string{
		`{}`,
		`{"MediaContainer":{"totalSize":2001,"Metadata":[]}}`,
		`{"MediaContainer":`,
	} {
		t.Run(body, func(t *testing.T) {
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) })
			now := time.Now()
			if _, err := c.programGrid(t.Context(), "tv.plex.providers.epg.cloud:2", now, now.Add(time.Hour)); err == nil {
				t.Fatal("incomplete guide was accepted as a successful replacement")
			}
		})
	}
}
