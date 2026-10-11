package rendering

import (
	"bytes"
	"fmt"
	"os"
	"testing"
	"time"

	"mistervision/internal/caption"
	"mistervision/internal/input/control"
	"mistervision/internal/ui"
)

func TestSegmentPromptPreservesSubtitlesAndControls(t *testing.T) {
	for _, height := range []int{240, 288, 360, 480} {
		for _, controls := range []bool{false, true} {
			p := PlaybackPresentation{Active: true, Title: "Episode", Subtitle: "The story continues after the intro.", PositionTicks: 200000000, DurationTicks: 14400000000, Seekable: true, TracksAvailable: true, ControlsVisible: controls}
			render := func(p PlaybackPresentation, labels control.Labels) []byte {
				return renderVideoOverlayOn(ui.NewOverlay(640, height), p, time.Unix(100, 0), labels, &caption.Renderer{})
			}
			base := render(p, nil)
			p.SkipLabel = "Skip intro"
			got := render(p, nil)
			if bytes.Equal(base, got) {
				t.Fatal("missing prompt")
			}
			if !bytes.Equal(base[height/2*640*4:], got[height/2*640*4:]) {
				t.Fatal("prompt changed subtitles or controls")
			}
			// Other playback overlays take precedence over the prompt.
			for _, mode := range []string{"wait", "destination", "notice", "tracks", "unbound"} {
				hidden := p
				labels := control.Labels(nil)
				switch mode {
				case "wait":
					hidden.WaitLabel = "Buffering..."
				case "destination":
					hidden.HasDestination = true
				case "notice":
					hidden.Notice = "Could not change playback."
				case "tracks":
					hidden.Tracks = &TrackMenu{}
				case "unbound":
					labels = control.Labels{control.Options: ""}
				}
				a := render(hidden, labels)
				hidden.SkipLabel = ""
				if !bytes.Equal(a, render(hidden, labels)) {
					t.Fatalf("prompt visible during %s", mode)
				}
			}
			custom := render(p, control.Labels{control.Options: "Y"})
			if bytes.Equal(got, custom) {
				t.Fatal("prompt ignored configured button label")
			}
			if dir := os.Getenv("SEGMENT_PREVIEW_DIR"); dir != "" && (height == 240 || height == 360) {
				c := ui.New(640, height)
				// A neutral stand-in for moving video makes the actual overlay easy to review.
				for y := 0; y < height; y++ {
					c.Rect(0, y, 640, 1, uint32(0x182832+y/12*0x010101))
				}
				ui.Composite(c.Pixels, got)
				name := fmt.Sprintf("skip-intro-%d-controls-%t.png", height, controls)
				if height == 240 {
					writeSetupPreview(t, dir, name, c)
				} else {
					writeRasterPreview(t, dir, name, c)
				}
			}
		}
	}
}
