package rendering

import (
	"bytes"
	"fmt"
	"mistervision/internal/connection"
	"mistervision/internal/release"
	"mistervision/internal/ui"
	"os"
	"reflect"
	"strings"
	"testing"

	"mistervision/internal/input/control"
)

func TestReleaseNotesWrapAndRemoveDecoration(t *testing.T) {
	lines := ReleaseNotes("## Changes\n[Read this](https://example.com) **first**.\n"+strings.Repeat("x", 80)+"\nBad\x1bcontrol", 320)
	text := strings.Join(lines, "\n")
	for _, unwanted := range []string{"https://", "**", "##", "\x1b"} {
		if strings.Contains(text, unwanted) {
			t.Fatal(text)
		}
	}
	for _, line := range lines {
		if len([]rune(line)) > (320-48)/8 {
			t.Fatalf("line exceeds width: %q", line)
		}
	}
	if !strings.Contains(text, "Read this first.") {
		t.Fatal(text)
	}
}

func TestReleaseNoteScrollGeometryMatchesFooter(t *testing.T) {
	a := AboutPresentation{CanInstall: true, Notes: make([]string, 100)}
	for _, height := range []int{240, 288, 480} {
		labels := control.Labels{control.Back: "An unusually long controller button", control.Up: "Up", control.Down: "Down"}
		layout := a.notesLayout(640, height, labels)
		if a.ScrollLimit(640, height, labels) != 100-layout.rows {
			t.Fatal("input and rendering scroll limits differ")
		}
		if layout.top+layout.rows*12 > layout.statusY {
			t.Fatal("notes overlap status")
		}
	}
}

func TestInstallationCompletionStatus(t *testing.T) {
	for _, automatic := range []bool{false, true} {
		a := AboutPresentation{Installed: true, Restarting: automatic}
		want := "Installed. Reopen MiSTerVision."
		if automatic {
			want = "Update installed. Restarting..."
		}
		if got := a.Status(); got != want {
			t.Fatalf("completion status: %q, want %q", got, want)
		}
	}
}

func TestReleaseNotesPreserveParagraphsAndGlyphWidths(t *testing.T) {
	word := strings.Repeat("界\ufe0f", 34)
	lines := ReleaseNotes("# Heading\n\n"+word+"\n\nTail", 320)
	want := []string{"Heading", "", word, "", "Tail"}
	if !reflect.DeepEqual(lines, want) {
		t.Fatalf("paragraphs or glyph widths changed: %#v", lines)
	}
	for _, line := range ReleaseNotes(word+word, 320) {
		if ui.TextWidth(line) > 272 || strings.HasPrefix(line, "\ufe0f") {
			t.Fatalf("split glyph or oversized line: %q", line)
		}
	}
}

func TestAboutFailureFitsCompactDisplay(t *testing.T) {
	for _, labels := range []control.Labels{control.KeyboardLabels(), nil} {
		for _, size := range [][2]int{{320, 240}, {640, 240}, {640, 288}, {640, 480}} {
			w, h := size[0], size[1]
			scene := Scene{Controls: labels, About: AboutPresentation{
				Visible: true, Checked: true, CanInstall: true, ProfileAction: connection.ProfileChoose,
				ForgetLabel: "Forget user",
				Profile:     &connection.Profile{Name: "Test viewer"},
				Connections: []connection.Choice{{ID: "plex", Name: "Plex"}},
				Release:     release.Status{Available: true, Latest: "v1.2.1", HasBundle: true},
				Message:     "Could not check for updates. Check your internet connection, then try Check updates again.",
			}}
			c := ui.New(w, h)
			renderScene(c, &sceneCache{}, scene, Animation{})
			sy := safeY(w, h)
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					if x >= 24 && x < w-24 && y >= sy && y < h-sy {
						continue
					}
					i := (y*w + x) * 4
					if c.Pixels[i] != 0x13 || c.Pixels[i+1] != 0x0d || c.Pixels[i+2] != 0x0b {
						t.Fatalf("About escaped safe area at %dx%d (%d,%d)", w, h, x, y)
					}
				}
			}
			if w == 320 {
				for _, text := range []string{"MiSTerVision", "Version dev", "Trent Nix", "Based on MiSTerFin by Pudding", "Studio", "CC BY-NC 4.0. Components have", "separate licenses.", "Test viewer", "Check updates again."} {
					if !screenContainsText(c, text) {
						t.Fatalf("compact About lost %q", text)
					}
				}
			}
			if dir := os.Getenv("MISTERVISION_ABOUT_PREVIEWS"); dir != "" {
				writeSetupPreview(t, dir, fmt.Sprintf("about-%dx%d-%s.png", w, h, labels.Name(control.Open)), c)
			}
		}
	}
}

// screenContainsText matches the complete glyph mask in a text color. Requiring
// unlit pixels too prevents a solid button badge from passing as readable text.
func screenContainsText(c *ui.Canvas, text string) bool {
	return screenContainsScaledText(c, text, 1)
}

func screenContainsScaledText(c *ui.Canvas, text string, scale int) bool {
	glyphs := ui.New(ui.TextWidth(text)*scale, 8*scale)
	glyphs.TextScaled(0, 0, text, 0xffffff, glyphs.Width, scale)
	for _, color := range []uint32{titleColor, dimColor, 0xffffff, 0xc0c0c0, 0xd0d0d0} {
		for y := 0; y <= c.Height-glyphs.Height; y++ {
			for x := 0; x <= c.Width-glyphs.Width; x++ {
				matches := true
				for gy := 0; gy < glyphs.Height && matches; gy++ {
					for gx := 0; gx < glyphs.Width; gx++ {
						j := (gy*glyphs.Width + gx) * 4
						i := ((y+gy)*c.Width + x + gx) * 4
						pixel := uint32(c.Pixels[i]) | uint32(c.Pixels[i+1])<<8 | uint32(c.Pixels[i+2])<<16
						if (glyphs.Pixels[j] != 0) != (pixel == color) {
							matches = false
							break
						}
					}
				}
				if matches {
					return true
				}
			}
		}
	}
	return screenContainsFaceText(c, text, scale)
}

func TestAboutProfileActionMatchesAvailableChoices(t *testing.T) {
	for _, tc := range []struct {
		action connection.ProfileAction
		want   string
	}{
		{connection.ProfileUnchanged, ""}, {connection.ProfileAdd, "Add user"}, {connection.ProfileChoose, "Switch profile"},
	} {
		scene := Scene{About: AboutPresentation{Visible: true, Checking: true, Profile: &connection.Profile{Name: "Only viewer"}, ProfileAction: tc.action}}
		c := ui.New(640, 240)
		renderScene(c, &sceneCache{}, scene, Animation{})
		for _, label := range []string{"Add user", "Switch profile"} {
			if screenContainsText(c, label) != (label == tc.want) {
				t.Errorf("profile action %q, wanted %q", label, tc.want)
			}
		}
	}
}

func TestAboutDisplaysAccountFailureDuringReleaseCheck(t *testing.T) {
	for _, size := range [][2]int{{320, 240}, {640, 240}, {640, 480}} {
		w, h := size[0], size[1]
		scene := Scene{About: AboutPresentation{
			Visible: true, Checking: true,
			AccountMessage: connection.SignInStorageTitle + ". " + connection.SignInStorageMessage,
			Release:        release.Status{Available: true, Latest: "v9.0.0"},
		}}
		canvas := ui.New(w, h)
		renderScene(canvas, &sceneCache{}, scene, Animation{})
		for _, line := range messageLines(scene.About.AccountMessage, w-48, 6) {
			if !screenContainsText(canvas, line) {
				t.Fatalf("account error line %q was hidden at %dx%d", line, w, h)
			}
		}
		if screenContainsText(canvas, "Checking for updates...") {
			t.Fatal("release check replaced the account error")
		}
	}
}

// screenContainsFaceText matches antialiased glyphs over the setup/About backdrop.
func screenContainsFaceText(c *ui.Canvas, text string, scale int) bool {
	if c.Typeface == nil {
		return false
	}
	for _, ink := range []uint32{titleColor, dimColor, 0xffffff, 0xc0c0c0, 0xd0d0d0} {
		im, _ := c.Typeface.Rasterize(text, 4096, scale, ink)
		if im == nil {
			continue
		}
		type sample struct {
			x, y    int
			b, g, r byte
		}
		var samples []sample
		minX, minY := im.Bounds().Dx(), im.Bounds().Dy()
		maxX, maxY := 0, 0
		for y := range im.Bounds().Dy() {
			for x := range im.Bounds().Dx() {
				i := y*im.Stride + x*4
				a := int(im.Pix[i+3])
				if a == 0 {
					continue
				}
				blend := func(v byte, bg int) byte { return byte((int(v)*a + bg*(255-a) + 127) / 255) }
				samples = append(samples, sample{x, y, blend(im.Pix[i+2], 0x13), blend(im.Pix[i+1], 0x0d), blend(im.Pix[i], 0x0b)})
				minX, minY = min(minX, x), min(minY, y)
				maxX, maxY = max(maxX, x), max(maxY, y)
			}
		}
		if len(samples) == 0 {
			continue
		}
		for i := range samples {
			samples[i].x -= minX
			samples[i].y -= minY
		}
		for y := 0; y < c.Height-(maxY-minY); y++ {
			for x := 0; x < c.Width-(maxX-minX); x++ {
				matches := true
				for _, s := range samples {
					i := ((y+s.y)*c.Width + x + s.x) * 4
					if c.Pixels[i] != s.b || c.Pixels[i+1] != s.g || c.Pixels[i+2] != s.r {
						matches = false
						break
					}
				}
				if matches {
					return true
				}
			}
		}
	}
	return false
}

// Update checks may change actions and status text, but not the identity block.
func TestAboutUpdateCheckKeepsIdentityLayout(t *testing.T) {
	for _, size := range [][2]int{{320, 240}, {640, 240}, {640, 288}, {640, 480}} {
		for _, labels := range []control.Labels{nil, control.KeyboardLabels()} {
			for _, withAccount := range []bool{false, true} {
				scene := Scene{Controls: labels, About: AboutPresentation{Visible: true, Checked: true}}
				if withAccount {
					scene.About.Profile = &connection.Profile{Name: "Test viewer"}
					scene.About.ProfileAction = connection.ProfileChoose
					scene.About.ForgetLabel = "Forget user"
					scene.About.Connections = []connection.Choice{{ID: "plex", Name: "Plex"}}
				}
				cache := &sceneCache{}
				canvas := ui.New(size[0], size[1])
				renderScene(canvas, cache, scene, Animation{})
				identity := bytes.Clone(cache.aboutBase.Pixels)
				for _, state := range []string{"checking", "available", "checking again", "failed", "current"} {
					scene.About.Checking = state == "checking" || state == "checking again"
					scene.About.Release.Available = state == "available" || state == "checking again"
					scene.About.Release.Latest = "v2.0.0"
					scene.About.Message = ""
					if state == "failed" {
						scene.About.Message = "Could not check for updates. Try again."
					}
					renderScene(canvas, cache, scene, Animation{})
					if !bytes.Equal(identity, cache.aboutBase.Pixels) {
						t.Fatalf("%dx%d account=%v state=%s: update check moved the logo or attribution", size[0], size[1], withAccount, state)
					}
				}
			}
		}
	}
}

func TestAboutHintsStayVisibleDuringUpdateCheck(t *testing.T) {
	for _, compact := range []bool{false, true} {
		for _, available := range []bool{false, true} {
			a := AboutPresentation{Release: release.Status{Available: available}, ProfileAction: connection.ProfileChoose,
				Connections: []connection.Choice{{ID: "plex"}}, ForgetLabel: "Forget user"}
			before := aboutHints(a, control.KeyboardLabels(), true, compact)
			a.Checking = true
			if got := aboutHints(a, control.KeyboardLabels(), true, compact); !reflect.DeepEqual(got, before) {
				t.Fatalf("update check changed navigation hints: %v -> %v", before, got)
			}
		}
	}
}
