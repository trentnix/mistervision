package plex

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"testing"

	"mistervision/internal/media"
)

func TestSegments(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/library/metadata/42" || r.URL.Query().Get("includeMarkers") != "1" {
			t.Errorf("wrong marker request: %s", r.URL)
		}
		fmt.Fprint(w, `{"MediaContainer":{"Metadata":[{"ratingKey":"42","Marker":[{"type":"intro","startTimeOffset":1250,"endTimeOffset":62500},{"type":"credits","startTimeOffset":300000,"endTimeOffset":310000,"final":0},{"type":"credits","startTimeOffset":320000,"endTimeOffset":330000,"final":1},{"type":"commercial","startTimeOffset":90000,"endTimeOffset":95000},{"type":"intro","startTimeOffset":0,"endTimeOffset":9223372036854775807},{"type":"unknown","startTimeOffset":0,"endTimeOffset":10}]}]}}`)
	})
	got, err := c.Segments(t.Context(), "42")
	want := []media.Segment{{Kind: media.SegmentIntro, StartTicks: 12500000, EndTicks: 625000000}, {Kind: media.SegmentCommercial, StartTicks: 900000000, EndTicks: 950000000}, {Kind: media.SegmentCredits, StartTicks: 3000000000, EndTicks: 3100000000}, {Kind: media.SegmentCredits, StartTicks: 3200000000, EndTicks: 3300000000}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("segments: %+v, %v", got, err)
	}
}

func TestSegmentsMissingAndInvalid(t *testing.T) {
	for _, tc := range []struct {
		body   string
		failed bool
	}{
		{`{"MediaContainer":{"Metadata":[{"ratingKey":42}]}}`, false},
		{`{"MediaContainer":{"Metadata":[{"ratingKey":"43"}]}}`, true},
		{`{"MediaContainer":{}}`, true}, {`not json`, true},
	} {
		c := testClient(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, tc.body) })
		got, err := c.Segments(t.Context(), "42")
		if (err != nil) != tc.failed || len(got) != 0 {
			t.Fatalf("%s: %+v %v", tc.body, got, err)
		}
	}
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusUnauthorized) })
	if _, err := c.Segments(t.Context(), "42"); !errors.Is(err, media.ErrUnauthorized) {
		t.Fatal(err)
	}
	if _, err := c.Segments(t.Context(), "../42"); err == nil {
		t.Fatal("accepted invalid ID")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := c.Segments(ctx, "42"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
