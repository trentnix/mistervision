package rendering

import (
	"bytes"
	"image"
	"image/color"
	"testing"
	"time"

	"mistervision/internal/media"
)

func TestWideBrowsingBackgrounds(t *testing.T) {
	art := image.NewRGBA(image.Rect(0, 0, 160, 90))
	for y := range 90 {
		for x := range 160 {
			c := color.RGBA{G: 180, A: 255}
			if x < 10 || x >= 150 {
				c = color.RGBA{R: 200, A: 255}
			}
			art.SetRGBA(x, y, c)
		}
	}
	for _, kind := range []string{"Movie", "Series", "Season", "MusicArtist", "MusicAlbum", "root-list", "custom-list"} {
		t.Run(kind, func(t *testing.T) {
			s := Scene{Now: time.Unix(100, 0), Artwork: Artwork{Backdrop: art}, Content: Content{Page: media.Page{Items: []media.Item{{Name: "Selected item", Type: kind}}}}}
			switch kind {
			case "Movie":
				s.Content.Detail = &media.Item{Name: "Movie preview", Type: kind}
			case "root-list":
				s.Root = true
				s.ListMode = true
			case "custom-list":
				s.Background = art
				s.Artwork.Backdrop = nil
			}
			r := NewRendererForRaster(480, 360)
			r.SetDisplayAspect(16.0 / 9)
			f := r.Render(640, 288, s)
			if !f.FullScreen || f.UIWidth != 640 || f.UIHeight != 360 {
				t.Fatalf("wrong display geometry: %dx%d", f.UIWidth, f.UIHeight)
			}
			// A matching 16:9 image retains both edges, including the gradient across them.
			for _, x := range []int{0, 639} {
				at := (90*640 + x) * 4
				if f.UI[at+2] == 0 || f.UI[at+1] != 0 {
					t.Fatalf("background edge missing or cropped at %d", x)
				}
			}
			first := bytes.Clone(f.UI)
			base := r.wide.cache.background
			if !bytes.Equal(first, r.Render(640, 288, s).UI) || base != r.wide.cache.background {
				t.Fatal("steady frame changed or rebuilt its backdrop")
			}
			// Clearing artwork must not retain the previous full-width image.
			s.Background = nil
			s.Artwork = Artwork{}
			f = r.Render(640, 288, s)
			if f.UI[(90*640)*4+2] != 0 {
				t.Fatal("old artwork survived selection change")
			}
			// Disabling widescreen restores the ordinary renderer, including its cache.
			r.SetDisplayAspect(4.0 / 3)
			got := r.Render(640, 288, s)
			want := NewRendererForRaster(480, 360).Render(640, 288, s)
			if got.FullScreen || !bytes.Equal(got.UI, want.UI) {
				t.Fatal("4:3 output changed")
			}
		})
	}
}

func TestWideBackgroundDoesNotReplaceMedia(t *testing.T) {
	for _, kind := range []string{"video", "photo"} {
		t.Run(kind, func(t *testing.T) {
			s := Scene{Now: time.Unix(100, 0)}
			switch kind {
			case "video":
				s.Video = true
				s.Content.Detail = &media.Item{Type: "Movie"}
			case "photo":
				s.Content.Detail = &media.Item{Type: "Photo"}
			}
			r := NewRendererForRaster(480, 360)
			r.SetDisplayAspect(16.0 / 9)
			got := r.Render(640, 288, s)
			want := NewRendererForRaster(480, 360).Render(640, 288, s)
			if got.FullScreen || !bytes.Equal(got.UI, want.UI) {
				t.Fatal("browsing background changed another presentation")
			}
		})
	}
}

// Account pages share one full-width background. Their foreground must remain
// identical to the 4:3 layout, including during updates and connection recovery.
func TestWideAccountBackgrounds(t *testing.T) {
	scenes := map[string]Scene{
		"about":             {About: AboutPresentation{Visible: true}},
		"connections":       {About: AboutPresentation{Visible: true, ConnectionsVisible: true}},
		"checking updates":  {About: AboutPresentation{Visible: true, Checking: true}},
		"release notes":     {About: AboutPresentation{Visible: true, NotesVisible: true, Notes: []string{"Release notes"}}},
		"installing update": {About: AboutPresentation{Visible: true, NotesVisible: true, Updating: true}},
		"PIN":               {Setup: SetupPresentation{Kind: SetupPIN}},
		"profiles":          {Setup: SetupPresentation{Kind: SetupProfiles}},
		"connecting":        {Setup: SetupPresentation{Kind: SetupConnecting}},
		"approval":          {Setup: SetupPresentation{Kind: SetupApproval, Code: "TEST"}},
		"failure":           {Setup: SetupPresentation{Kind: SetupFailure}},
		"servers":           {Setup: SetupPresentation{Kind: SetupServers}},
		"confirmation":      {Setup: SetupPresentation{Kind: SetupConfirm}},
	}
	for name, scene := range scenes {
		t.Run(name, func(t *testing.T) {
			r := NewRendererForRaster(480, 360)
			r.SetDisplayAspect(16.0 / 9)
			// Paint another page first so the assertion also catches stale side panels.
			r.Render(640, 288, Scene{Root: true, Background: image.NewRGBA(image.Rect(0, 0, 16, 9))})
			frame := r.Render(640, 288, scene)
			if !frame.FullScreen || frame.UIWidth != 640 || frame.UIHeight != 360 {
				t.Fatal("account screen is pillarboxed")
			}
			for y := 0; y < 360; y++ {
				for _, x := range []int{0, 79, 560, 639} {
					at := (y*640 + x) * 4
					if !bytes.Equal(frame.UI[at:at+3], []byte{0x13, 0x0d, 0x0b}) {
						t.Fatalf("background at %d,%d: %v", x, y, frame.UI[at:at+3])
					}
				}
			}
			normal := NewRendererForRaster(480, 360).Render(640, 288, scene)
			for y := 0; y < 360; y++ {
				if !bytes.Equal(frame.UI[(y*640+80)*4:(y*640+560)*4], normal.UI[y*480*4:(y+1)*480*4]) {
					t.Fatal("account foreground changed")
				}
			}
			r.SetDisplayAspect(4.0 / 3)
			restored := r.Render(640, 288, scene)
			if restored.FullScreen || !bytes.Equal(restored.UI, normal.UI) {
				t.Fatal("4:3 layout did not restore")
			}
		})
	}
}

func TestWideExitStripeAndDenseText(t *testing.T) {
	for _, height := range []int{360, 540, 720} {
		r := NewRendererForRaster(height*4/3, height)
		r.SetDisplayAspect(16.0 / 9)
		art := image.NewRGBA(image.Rect(0, 0, 16, 9))
		for i := range art.Pix {
			art.Pix[i] = 255
		}
		s := Scene{Root: true, Background: art, Now: time.Unix(100, 0)}
		before := bytes.Clone(r.Render(640, 288, s).UI)
		s.ExitConfirm = true
		frame := r.Render(640, 288, s)
		// Both outer edges and the empty foreground margin must receive the
		// same shade exactly once. The old overlay left the side panels bright.
		for _, x := range []int{0, frame.UIWidth / 8, frame.UIWidth - 1} {
			at := (height/2*frame.UIWidth + x) * 4
			want := byte(int(before[at]) * (255 - 210) / 255)
			if want == before[at] || frame.UI[at] != want {
				t.Fatalf("%d: stripe at %d got %d, want %d", height, x, frame.UI[at], want)
			}
		}
		found := false
		for key, entry := range r.cache.text.images {
			if key.text == "Exit?" {
				found = true
				if entry.denseTrimmed == nil || entry.denseTrimmed.Source.Bounds().Dy() <= entry.trimmed.Bounds().Dy() {
					t.Fatal("exit text did not use display density")
				}
			}
		}
		if !found {
			t.Fatal("exit bypassed shared text cache")
		}
	}
}
