//go:build linux && cgo

package platform

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestPresentation(t *testing.T) {
	for _, tc := range []struct {
		spec           string
		g              Geometry
		bx, by, bw, bh int
	}{
		{"640x288", Geometry{640, 288, 640, 288}, 0, 0, 640, 288},
		{"640x240", Geometry{640, 240, 640, 240}, 0, 0, 640, 240},
		{"720x576", Geometry{640, 288, 720, 576}, 0, 18, 720, 540},
		{"720x480", Geometry{640, 288, 720, 480}, 40, 0, 640, 480},
		{"1920x1080", Geometry{640, 288, 1920, 1080}, 240, 0, 1440, 1080},
		{"640x480", Geometry{640, 240, 640, 480}, 0, 0, 640, 480},
		{"1280x720", Geometry{640, 288, 1280, 720}, 160, 0, 960, 720},
		{"480x270", Geometry{640, 288, 480, 270}, 60, 0, 360, 270},
		{"320x180", Geometry{640, 288, 320, 180}, 40, 0, 240, 180},
		{"640x360", Geometry{640, 288, 640, 360}, 80, 0, 480, 360},
		{"600x800", Geometry{640, 288, 600, 800}, 0, 175, 600, 450},
	} {
		t.Run(tc.spec, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "frame.raw")
			d, err := Open(Options{Headless: tc.spec, Output: path})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := d.Close(); err != nil {
					t.Error(err)
				}
			})
			if d.Geometry() != tc.g {
				t.Fatalf("geometry: %+v", d.Geometry())
			}
			pixels := make([]byte, tc.g.Width*tc.g.Height*4)
			for y := 0; y < tc.g.Height; y++ {
				for x := 0; x < tc.g.Width; x++ {
					i := (y*tc.g.Width + x) * 4
					pixels[i], pixels[i+1], pixels[i+2] = byte(x), byte(y), 127
				}
			}
			if err := d.Present(pixels); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if len(raw) != tc.g.OutputWidth*tc.g.OutputHeight*4 {
				t.Fatalf("dump size %d", len(raw))
			}
			for y := 0; y < tc.g.OutputHeight; y++ {
				for x := 0; x < tc.g.OutputWidth; x++ {
					want := []byte{0, 0, 0, 0}
					if x >= tc.bx && x < tc.bx+tc.bw && y >= tc.by && y < tc.by+tc.bh {
						sx, sy := (x-tc.bx)*tc.g.Width/tc.bw, (y-tc.by)*tc.g.Height/tc.bh
						i := (sy*tc.g.Width + sx) * 4
						want = pixels[i : i+4]
					}
					i := (y*tc.g.OutputWidth + x) * 4
					if !bytes.Equal(raw[i:i+4], want) {
						t.Fatalf("pixel %d,%d: got %v want %v", x, y, raw[i:i+4], want)
					}
				}
			}
			// Reuse the Go buffer across cgo calls, then close twice.
			clear(pixels)
			if err := d.Present(pixels); err != nil {
				t.Fatal(err)
			}
			raw, err = os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(raw, make([]byte, len(raw))) {
				t.Fatal("stale pixels after buffer reuse")
			}
			if err := d.Close(); err != nil {
				t.Fatal(err)
			}
			if err := d.Present(pixels); err == nil {
				t.Fatal("present after close succeeded")
			}
		})
	}
}

func TestErrors(t *testing.T) {
	for _, spec := range []string{"0x240", "640x-1", "640x288junk", "999999999999999x2", "8192x8192"} {
		if d, err := Open(Options{Headless: spec}); err == nil {
			d.Close()
			t.Errorf("accepted %q", spec)
		}
	}
	for _, options := range []Options{{Device: "/nonexistent/misterfin-fb"}, {Output: "frame.raw"}, {Headless: "1x1", Output: "bad\x00path"}} {
		if d, err := Open(options); err == nil {
			d.Close()
			t.Errorf("accepted %+v", options)
		}
	}
	d, err := Open(Options{Headless: "2x2", Output: filepath.Join(t.TempDir(), "missing", "frame.raw")})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	for _, pixels := range [][]byte{nil, make([]byte, 15), make([]byte, 17), make([]byte, 16)} {
		if err := d.Present(pixels); err == nil {
			t.Errorf("expected error for %d bytes or unwritable dump", len(pixels))
		}
	}
}

func TestVideoFillsOutputAndRestoresBrowsingViewport(t *testing.T) {
	for _, aspect := range []string{"auto", "4:3", "16:9"} {
		t.Run(aspect, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "frame")
			d, err := Open(Options{Headless: "640x360", Output: path, AspectRatio: aspect})
			if err != nil {
				t.Fatal(err)
			}
			defer d.Close()
			g := d.Geometry()
			pixels := bytes.Repeat([]byte{120, 120, 120, 255}, g.Width*g.Height)
			read := func() []byte {
				b, e := os.ReadFile(path)
				if e != nil {
					t.Fatal(e)
				}
				return b
			}
			if err = d.Present(pixels); err != nil {
				t.Fatal(err)
			}
			before := read()
			want := byte(0)
			if aspect == "4:3" {
				want = 120
			}
			if before[(180*640)*4] != want {
				t.Fatal("wrong browsing aspect")
			}
			if err = PresentVideo(d, pixels); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(read(), bytes.Repeat([]byte{120, 120, 120, 255}, 640*360)) {
				t.Fatal("video did not fill output")
			}
			if err = d.Present(pixels); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, read()) {
				t.Fatal("video changed browsing viewport")
			}
		})
	}
}

func TestBrowsingRasterUsesViewportWithoutResampling(t *testing.T) {
	for _, tc := range []struct {
		spec string
		w, h int
	}{{"640x240", 640, 240}, {"640x480", 640, 240}, {"640x576", 640, 288}, {"960x540", 720, 540}, {"1280x720", 960, 720}} {
		t.Run(tc.spec, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "raster.raw")
			d, err := Open(Options{Headless: tc.spec, Output: path})
			if err != nil {
				t.Fatal(err)
			}
			defer d.Close()
			w, h := RasterSize(d)
			if w != tc.w || h != tc.h {
				t.Fatalf("raster %dx%d, want %dx%d", w, h, tc.w, tc.h)
			}
			pixels := make([]byte, w*h*4)
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					pixels[(y*w+x)*4] = byte(x)
					pixels[(y*w+x)*4+1] = byte(y)
				}
			}
			if err := PresentRaster(d, pixels, w, h); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			g := d.Geometry()
			if g.OutputWidth != 640 {
				left := (g.OutputWidth - w) / 2
				for y := 0; y < h; y++ {
					for x := 0; x < w; x++ {
						at := (y*g.OutputWidth + left + x) * 4
						if raw[at] != byte(x) || raw[at+1] != byte(y) {
							t.Fatalf("raster was resampled at %d,%d", x, y)
						}
					}
				}
			}
			if err := PresentRaster(d, pixels[:len(pixels)-1], w, h); err == nil {
				t.Fatal("accepted incomplete raster")
			}
		})
	}
}

// A full-screen background must not change the next screen's normal viewport.
func TestFullRasterRestoresBrowsingViewport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "frame.raw")
	d, err := Open(Options{Headless: "640x360", Output: path})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	full := d.(FullRasterPresenter)
	pixels := bytes.Repeat([]byte{10, 20, 30, 0}, 640*360)
	if err = full.PresentFullRaster(pixels, 640, 360); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, pixels) {
		t.Fatal("full-screen frame was cropped or pillarboxed")
	}
	if err = PresentRaster(d, bytes.Repeat([]byte{40, 50, 60, 0}, 480*360), 480, 360); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if raw[0] != 0 || raw[80*4] != 40 || raw[560*4] != 0 {
		t.Fatal("normal viewport was not restored")
	}
}
