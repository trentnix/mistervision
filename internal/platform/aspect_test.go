package platform

import "testing"

func TestResolveAspect(t *testing.T) {
	for _, tc := range []struct {
		setting string
		w, h    int
		want    float64
	}{
		{"", 640, 240, 4.0 / 3}, {"auto", 640, 288, 4.0 / 3}, {"auto", 640, 480, 4.0 / 3}, {"auto", 640, 576, 4.0 / 3},
		{"auto", 640, 360, 16.0 / 9}, {"auto", 480, 270, 16.0 / 9}, {"auto", 1920, 1080, 16.0 / 9},
		{"4:3", 640, 360, 4.0 / 3}, {"16:9", 640, 480, 16.0 / 9},
	} {
		if got := ResolveAspect(tc.setting, tc.w, tc.h); got != tc.want {
			t.Fatalf("%+v: %v", tc, got)
		}
	}
}
