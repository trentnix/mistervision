package main

import (
	"testing"

	"mistervision/internal/media"
	"mistervision/internal/platform"
)

func TestTargetsUsePlaybackRasterForTranscoding(t *testing.T) {
	for _, tc := range []struct {
		w, h   int
		aspect float64
		want   media.VideoSize
	}{
		{640, 240, 0, media.VideoSize{Width: 320, Height: 240}},
		{640, 288, 0, media.VideoSize{Width: 384, Height: 288}},
		{640, 360, 0, media.VideoSize{Width: 640, Height: 360}},
		{640, 480, 0, media.VideoSize{Width: 640, Height: 480}},
		{960, 540, 0, media.VideoSize{Width: 960, Height: 540}},
		{1280, 720, 0, media.VideoSize{Width: 1280, Height: 720}},
		{640, 360, 4.0 / 3, media.VideoSize{Width: 480, Height: 360}},
	} {
		// Browsing can use a different raster. Both targets must use video output.
		g := platform.Geometry{Width: 640, Height: 288, OutputWidth: tc.w, OutputHeight: tc.h}
		o := launchOptions{displayAspect: tc.aspect}
		native := misterPlayback(o, g)
		desktop := desktopPlayback(o, g)
		if native.VideoSize != tc.want || desktop.VideoSize != tc.want {
			t.Fatalf("%+v: native=%+v desktop=%+v", tc, native.VideoSize, desktop.VideoSize)
		}
	}
}
