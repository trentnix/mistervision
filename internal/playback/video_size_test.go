package playback

import (
	"context"
	"math"
	"testing"

	"mistervision/internal/media"
)

func TestTranscodeSize(t *testing.T) {
	for _, tc := range []struct {
		w, h   int
		aspect float64
		want   media.VideoSize
	}{
		{640, 240, 4.0 / 3, media.VideoSize{Width: 320, Height: 240}},
		{640, 288, 4.0 / 3, media.VideoSize{Width: 384, Height: 288}},
		{640, 360, 16.0 / 9, media.VideoSize{Width: 640, Height: 360}},
		{640, 480, 4.0 / 3, media.VideoSize{Width: 640, Height: 480}},
		{640, 576, 4.0 / 3, media.VideoSize{Width: 768, Height: 576}},
		{960, 540, 16.0 / 9, media.VideoSize{Width: 960, Height: 540}},
		{1280, 720, 16.0 / 9, media.VideoSize{Width: 1280, Height: 720}},
		{1280, 720, 0, media.VideoSize{Width: 1280, Height: 720}},
		{640, 241, 4.0 / 3, media.VideoSize{Width: 320, Height: 240}},
		{0, 240, 4.0 / 3, media.VideoSize{}},
		{640, 240, math.NaN(), media.VideoSize{}},
	} {
		if got := TranscodeSize(tc.w, tc.h, tc.aspect); got != tc.want {
			t.Errorf("%+v: got %+v", tc, got)
		}
	}
}

type sizedVideoBackend struct {
	media.Playback
	request media.VideoRequest
}

func (*sizedVideoBackend) Identity() media.Identity {
	return media.Identity{Server: "test", User: "viewer"}
}

func (*sizedVideoBackend) PlaybackDetails(context.Context, string) (media.Item, error) {
	return media.Item{ID: "movie", Type: "Movie", RunTimeTicks: 1000000000}, nil
}
func (b *sizedVideoBackend) PrepareVideo(_ context.Context, r media.VideoRequest) (media.PreparedStream, error) {
	b.request = r
	return media.PreparedStream{}, nil
}

func TestPreparationRetainsSizeAcrossSeekAndPictureModes(t *testing.T) {
	size := media.VideoSize{Width: 320, Height: 240}
	for _, mode := range []PictureMode{PictureOriginal, PictureZoom43} {
		for _, offset := range []int64{0, 300000000} {
			backend := &sizedVideoBackend{}
			choices := trackPreparation{explicit: &TrackOptions{Picture: mode, Selection: media.TrackSelection{AudioIndex: -1, SubtitleIndex: -1}}}
			_, err := preparePlayback(t.Context(), backend, Config{VideoSize: size}, Request{Item: media.Item{ID: "movie", Type: "Movie"}, StartTicks: &offset}, choices)
			if err != nil {
				t.Fatal(err)
			}
			if backend.request.Size != size || backend.request.StartTicks != offset {
				t.Fatalf("lost geometry or seek: %+v", backend.request)
			}
		}
	}
}
