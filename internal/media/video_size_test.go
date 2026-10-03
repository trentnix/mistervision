package media

import "testing"

func TestVideoSizeCaps(t *testing.T) {
	for _, tc := range []struct {
		size VideoSize
		w, h int
		want VideoSize
	}{
		{VideoSize{320, 240}, 720, 576, VideoSize{320, 240}},
		{VideoSize{1280, 720}, 640, 0, VideoSize{640, 720}},
		{VideoSize{1280, 720}, 0, 480, VideoSize{1280, 480}},
		{VideoSize{1280, 720}, 0, 0, VideoSize{1280, 720}},
		{VideoSize{640, 480}, 319, 239, VideoSize{318, 238}},
		{VideoSize{}, 0, 0, VideoSize{720, 576}},
		{VideoSize{}, 960, 540, VideoSize{960, 540}},
	} {
		if got := tc.size.Capped(tc.w, tc.h); got != tc.want {
			t.Errorf("%+v: got %+v", tc, got)
		}
	}
}
