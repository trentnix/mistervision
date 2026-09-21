package mplayer

import (
	"encoding/json"
	"errors"
	"fmt"
	"mistervision/internal/jellyfin"
	"mistervision/internal/media"
	"mistervision/internal/player"
	"strings"
	"testing"
)

func TestMPlayerCRTAspect(t *testing.T) {
	var item jellyfin.Item
	json.Unmarshal([]byte(`{"MediaStreams":[{"Type":"Video","Width":720,"Height":576,"AspectRatio":"16:9"}]}`), &item)
	for _, h := range []int{240, 288} {
		args := Decoder{Width: 640, Height: h, Device: "/dev/fb0"}.Args(item, "")
		want := fmt.Sprintf("mistervision=640:%d:1.777777778:0", h)
		if !strings.Contains(strings.Join(args, " "), want) {
			t.Fatalf("args %v", args)
		}
	}
}

func TestLiveTVAspectFallbackAndMetadata(t *testing.T) {
	for _, tc := range []struct {
		aspect string
		dar    float64
	}{{"", 16.0 / 9}, {"16:9", 16.0 / 9}, {"4:3", 4.0 / 3}} {
		item := jellyfin.Item{Type: "TvChannel"}
		if tc.aspect != "" {
			item.MediaStreams = []jellyfin.MediaStream{{Type: "Video", Width: 720, Height: 576, AspectRatio: tc.aspect}}
		}
		args := Decoder{Width: 640, Height: 240, Device: "/dev/fb0"}.Args(item, "")
		want := fmt.Sprintf("mistervision=640:240:%.9f:0", tc.dar)
		if !strings.Contains(strings.Join(args, " "), want) {
			t.Fatalf("aspect %q: %v", tc.aspect, args)
		}
	}
}

func TestHardwareVideoSynchronization(t *testing.T) {
	for _, tc := range []struct{ kind, autosync, cacheMinimum string }{
		{"Movie", "30", "20"}, {"Episode", "30", "20"}, {"TvChannel", "1", "0"}, {"LiveTvChannel", "1", "0"},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			args := Decoder{Width: 640, Height: 240, Device: "/dev/fb0"}.Args(jellyfin.Item{Type: tc.kind}, "")
			joined := " " + strings.Join(args, " ") + " "
			if !strings.Contains(joined, " -framedrop ") || !strings.Contains(joined, " -autosync "+tc.autosync+" ") {
				t.Fatalf("missing hardware synchronization policy: %v", args)
			}
			if !strings.Contains(joined, " -cache 8192 ") || !strings.Contains(joined, " -cache-min "+tc.cacheMinimum+" ") {
				t.Fatalf("incorrect startup buffering policy: %v", args)
			}
			if strings.Contains(joined, " -fps ") || strings.Contains(joined, " -speed ") {
				t.Fatalf("hardware playback must respect stream timing: %v", args)
			}
		})
	}
}

func TestValidateFramebufferModes(t *testing.T) {
	for _, size := range [][2]int{{640, 240}, {640, 288}, {640, 480}, {640, 576}, {1920, 1080}, {1280, 720}, {720, 480}, {640, 360}, {480, 270}, {320, 180}, {3840, 2160}, {641, 480}, {640, 119}, {0, 0}} {
		d := Decoder{Width: size[0], Height: size[1]}
		supported := size != [2]int{3840, 2160} && size != [2]int{641, 480} && size != [2]int{640, 119} && size != [2]int{0, 0}
		for _, kind := range []string{"Movie", "Episode", "TvChannel", "Audio"} {
			err := d.Validate(media.Item{Type: kind})
			if supported || kind == "Audio" {
				if err != nil {
					t.Fatalf("%v %s: %v", size, kind, err)
				}
			} else {
				var display *player.UnsupportedDisplayError
				if !errors.As(err, &display) || display.Width != size[0] || display.Height != size[1] {
					t.Fatalf("%v: wrong error %v", size, err)
				}
			}
		}
	}
}

func TestDisplayAspectIsSeparateFromSourceAspect(t *testing.T) {
	item := media.Item{MediaStreams: []media.MediaStream{{Type: "Video", AspectRatio: "4:3"}}}
	args := Decoder{Width: 640, Height: 360, DisplayAspect: 16.0 / 9}.Args(item, "")
	if !strings.Contains(strings.Join(args, " "), "mistervision=640:360:1.333333333:0:1.777777778") {
		t.Fatalf("source and display aspects not passed independently: %v", args)
	}
}
