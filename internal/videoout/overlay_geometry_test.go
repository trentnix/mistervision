package videoout

import (
	"mistervision/internal/platform"
	"testing"
)

func TestOverlayKeepsFourThreeProportionsOnWideScreen(t *testing.T) {
	g := platform.Geometry{Width: 640, Height: 240, OutputWidth: 640, OutputHeight: 360}
	src := make([]byte, 640*240*4)
	for i := range src {
		src[i] = 255
	}
	dst := ScaleOverlay(nil, src, g, 16.0/9)
	for y := 0; y < 360; y++ {
		for x := 0; x < 640; x++ {
			want := byte(0)
			if x >= 80 && x < 560 {
				want = 255
			}
			if dst[(y*640+x)*4] != want {
				t.Fatalf("pixel %d,%d", x, y)
			}
		}
	}
	// A following full-screen overlay must reuse and overwrite the old margins.
	dst = ScaleOverlay(dst, src, g, 4.0/3)
	for _, v := range dst {
		if v != 255 {
			t.Fatal("stale margin")
		}
	}
}
