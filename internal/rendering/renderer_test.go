package rendering

import (
	"bytes"
	"image"
	"testing"
	"time"

	"mistervision/internal/media"
	"mistervision/internal/ui"
)

func TestRasterRendererMatchesSceneAndClearsOverlays(t *testing.T) {
	m, art := benchmarkScene()
	now := time.Unix(1800000000, 0)
	var renderer Renderer = NewRenderer()
	for _, size := range [][2]int{{640, 240}, {640, 480}, {640, 288}, {640, 576}, {640, 240}} {
		for _, list := range []bool{true, false} {
			m.ListMode = list
			scene := testScene(m, PlaybackPresentation{}, SetupPresentation{}, art, "", now)
			frame := renderer.Render(size[0], size[1], scene)
			want := render(size[0], size[1], m, SetupPresentation{}, art, "", Animation{}, now)
			if frame.Video || frame.Overlay != nil || !bytes.Equal(frame.UI, want) {
				t.Fatal("renderer changed browser scene")
			}
			scene.Video = true
			scene.Playback = PlaybackPresentation{WaitLabel: "Loading..."}
			frame = renderer.Render(size[0], size[1], scene)
			if !frame.Video || !bytes.Equal(frame.Overlay, renderVideoOverlay(size[0], size[1], scene.Playback, now)) {
				t.Fatal("renderer changed overlay")
			}
			scene.Playback = PlaybackPresentation{}
			frame = renderer.Render(size[0], size[1], scene)
			for _, v := range frame.Overlay {
				if v != 0 {
					t.Fatal("hidden overlay retained pixels from previous frame")
				}
			}
		}
	}
}

func TestRendererAnimationOwnsTitleAndSelectionTiming(t *testing.T) {
	var a animationState
	m, _ := benchmarkScene()
	m.Root = false
	m.Content = Content{Title: "first"}
	now := time.Unix(100, 0)
	s := testScene(m, PlaybackPresentation{}, SetupPresentation{}, Artwork{}, "", now)
	a.advance(s, 6)
	s.Now = now.Add(35 * time.Millisecond)
	s.Content.Selected = 1
	got := a.advance(s, 6)
	if got.Selection < 0.63 || got.Selection > 0.64 {
		t.Fatalf("unexpected selection easing: %+v", got)
	}
	s.Now = now.Add(time.Second)
	s.Content.Title = "second"
	if got = a.advance(s, 6); got.TitleSeconds != 0 {
		t.Fatal("new title inherited marquee offset")
	}
	s.Now = s.Now.Add(time.Second)
	if got = a.advance(s, 6); got.TitleSeconds != 1 {
		t.Fatal("marquee time did not advance")
	}
}

func TestVideoBackgroundCacheMatchesFreshRender(t *testing.T) {
	m, art := benchmarkScene()
	m.Content.Detail = &media.Item{Name: "Episode", Type: "Episode"}
	r := NewRenderer()
	for _, height := range []int{240, 480, 288, 576, 240} {
		for _, backdrop := range []Artwork{art, {}, art} {
			s := testScene(m, PlaybackPresentation{}, SetupPresentation{}, backdrop, "", time.Unix(100, 0))
			s.Video = true
			for i := 0; i < 2; i++ {
				frame := r.Render(640, height, s)
				want := renderScene(ui.New(640, height), nil, s, Animation{})
				if !bytes.Equal(frame.UI, want) {
					t.Fatal("cached video backdrop changed pixels")
				}
			}
		}
	}
}

func TestVideoBackdropReusesJPEGAndPNGImages(t *testing.T) {
	for _, source := range []image.Image{
		image.NewYCbCr(image.Rect(0, 0, 32, 24), image.YCbCrSubsampleRatio420),
		image.NewNRGBA(image.Rect(0, 0, 32, 24)),
	} {
		m, _ := benchmarkScene()
		m.Content.Detail = &media.Item{Name: "Episode", Type: "Episode"}
		s := testScene(m, PlaybackPresentation{}, SetupPresentation{}, Artwork{Backdrop: source}, "", time.Unix(100, 0))
		s.Video = true
		r := NewRenderer()
		r.Render(640, 240, s)
		prepared := r.cache.videoBackground
		r.Render(640, 240, s)
		if prepared != r.cache.videoBackground {
			t.Fatalf("did not reuse %T backdrop", source)
		}
	}
}

func TestGlobalExitConfirmationCoversEveryScreen(t *testing.T) {
	for _, size := range [][2]int{{640, 240}, {640, 288}, {640, 360}, {640, 480}} {
		for _, screen := range []string{"browse", "about", "setup", "music", "options", "video", "video-options"} {
			s := Scene{Root: true, Now: time.Unix(100, 0)}
			switch screen {
			case "about":
				s.About.Visible = true
			case "setup":
				s.Setup.Kind = SetupPIN
			case "music", "options":
				s.Audio = true
				s.Content.Detail = &media.Item{Type: "Audio", Name: "Track"}
				if screen == "options" {
					s.Options = &OptionsMenu{Title: "Music options", Rows: []string{"Repeat: Off"}}
				}
			case "video", "video-options":
				s.Video = true
				s.Content.Detail = &media.Item{Type: "Movie", Name: "Movie"}
				if screen == "video-options" {
					s.Playback.Tracks = &TrackMenu{}
				}
			}
			r := NewRenderer()
			base := r.Render(size[0], size[1], s)
			uiPixels, overlayPixels := bytes.Clone(base.UI), bytes.Clone(base.Overlay)
			s.ExitConfirm = true
			confirmed := r.Render(size[0], size[1], s)
			if bytes.Equal(uiPixels, confirmed.UI) {
				t.Fatalf("exit missing on %s at %v", screen, size)
			}
			if s.Video {
				if bytes.Equal(overlayPixels, confirmed.Overlay) {
					t.Fatal("exit missing from video overlay")
				}
				// The translucent confirmation stripe must span the video frame.
				if confirmed.Overlay[(size[1]/2*size[0])*4+3] == 0 {
					t.Fatal("video confirmation stripe missing")
				}
			}
			s.ExitConfirm = false
			canceled := r.Render(size[0], size[1], s)
			if !bytes.Equal(uiPixels, canceled.UI) || !bytes.Equal(overlayPixels, canceled.Overlay) {
				t.Fatalf("cancel left exit pixels on %s", screen)
			}
		}
	}
}
