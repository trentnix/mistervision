package rendering

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"mistervision/internal/ui"
)

func TestRootHeadingTruncatesBeforeClock(t *testing.T) {
	for _, width := range []int{320, 640} {
		for _, height := range []int{240, 288, 480} {
			long := strings.Repeat("Wide title ", 20)
			var first []byte
			for _, seconds := range []float64{0, 2, 20} {
				got, expected := ui.New(width, height), ui.New(width, height)
				scene := Scene{Root: true, Title: &long, Now: time.Unix(100, 0)}
				p := screenPainter{canvas: got, width: width, height: height, safeY: safeY(width, height), scene: scene, animation: Animation{TitleSeconds: seconds}}
				p.header(scene.title(), p.safeY+4)
				p.canvas = expected
				p.clock()
				if first == nil {
					first = bytes.Clone(got.Pixels)
				} else if !bytes.Equal(got.Pixels, first) {
					t.Fatalf("root heading scrolled at %dx%d, time %v", width, height, seconds)
				}
				for y := range height {
					for x := range width {
						if x >= 24 && x < width-84 {
							continue
						}
						i := (y*width + x) * 4
						if !bytes.Equal(got.Pixels[i:i+4], expected.Pixels[i:i+4]) {
							t.Fatal("heading drew outside its safe column")
						}
					}
				}
			}
		}
	}
	if got := (Scene{Root: true}).title(); got != "MiSTerVision" {
		t.Fatal(got)
	}
	custom := "Custom"
	if got := (Scene{Title: &custom, Content: Content{Title: "Library"}}).title(); got != "Library" {
		t.Fatal(got)
	}
}

func TestEmptyTitleHidesHeadingAndKeepsClock(t *testing.T) {
	title := ""
	for _, height := range []int{240, 288, 480} {
		for _, list := range []bool{false, true} {
			scene := Scene{Root: true, ListMode: list, Title: &title, Now: time.Unix(100, 0)}
			if scene.title() != "" {
				t.Fatal("empty title restored the default")
			}
			got, expected := ui.New(640, height), ui.New(640, height)
			p := screenPainter{canvas: got, width: 640, height: height, safeY: safeY(640, height), scene: scene}
			if list {
				p.list()
			} else {
				p.carousel()
			}
			p.canvas = expected
			p.clock()
			start, end := p.safeY*640*4, (p.safeY+20)*640*4
			if !bytes.Equal(got.Pixels[start:end], expected.Pixels[start:end]) {
				t.Fatalf("hidden heading changed clock or drew title pixels: height=%d list=%v", height, list)
			}
		}
	}
}

func TestScrollingHeadingCompletesCycleAcrossRasterDensities(t *testing.T) {
	for _, size := range [][2]int{{640, 240}, {640, 480}, {480, 360}, {960, 720}} {
		canvas := ui.NewRaster(640, 240, size[0], size[1])
		p := screenPainter{canvas: canvas, width: 640, height: 240, safeY: safeY(640, 240)}
		title := "Boards of Canada / Music Has The Right To Children"
		advance := p.headingText(title, 4096, 28, 1, titleColor).Bounds().Dx()
		if advance <= 640-84-24 {
			t.Fatal("test heading must scroll")
		}
		for offset := 0; offset <= advance+40; offset++ {
			p.animation.TitleSeconds = (float64(offset) + 0.25) / 15
			p.header(title, p.safeY+4)
		}
	}
}
