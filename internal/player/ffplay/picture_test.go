package ffplay

import (
	"strings"
	"testing"

	"mistervision/internal/jellyfin"
)

func TestFFplayZoomCropMatchesSourceAspect(t *testing.T) {
	for _, tc := range []struct {
		aspect, crop string
	}{
		{"16:9", "ih*1.333333333/sar"},
		{"4:3", "iw*3/8"},
		{"1:1", "iw*sar/1.333333333"},
	} {
		item := jellyfin.Item{MediaStreams: []jellyfin.MediaStream{{Type: "Video", AspectRatio: tc.aspect}}}
		if filter := ffplayZoomFilter(item, 0); !strings.Contains(filter, tc.crop) {
			t.Fatalf("aspect %s used wrong crop: %s", tc.aspect, filter)
		}
	}
}
