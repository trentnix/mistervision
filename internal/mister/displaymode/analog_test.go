package displaymode

import "testing"

func TestNativeAnalogRoute(t *testing.T) {
	const native = "[MiSTer]\nfb_terminal=2\nvga_scaler=0\ndirect_video=0\nforced_scandoubler=0\n"
	for _, tc := range []struct {
		name, ini, core string
		w, h            int
		want            bool
	}{
		{"NTSC", native, "MENU", 640, 240, true},
		{"PAL", native, "MENU", 640, 288, true},
		{"defaults", "[MiSTer]\nfb_terminal=2", "MENU", 640, 240, true},
		{"empty", "", "MENU", 640, 240, false},
		{"scaler", native + "[Menu]\nvga_scaler=1", "MENU", 640, 240, false},
		{"terminal", native + "[Menu]\nfb_terminal=1", "MENU", 640, 240, false},
		{"direct", native + "[Menu]\ndirect_video=1", "MENU", 640, 240, false},
		{"scandoubler", native + "[Menu]\nforced_scandoubler=1", "MENU", 640, 240, false},
		{"interlaced", native, "MENU", 640, 480, false},
		{"PAL interlaced", native, "MENU", 640, 576, false},
		{"HDMI", native, "MENU", 640, 360, false},
		{"wide", native, "MENU", 1280, 240, false},
		{"invalid", native + "[Menu]\nfb_terminal=no", "MENU", 640, 240, false},
		{"comment", native + "[Menu]\nfb_terminal=2 ; native", "MENU", 640, 240, true},
		{"other core", native + "[ao486]\nvga_scaler=1", "MENU", 640, 240, true},
		{"handoff override", native + "[MiSTerVisionFramebuffer]\nfb_terminal=1", framebufferCore, 640, 240, false},
		{"handoff native", native + "[MiSTerVisionFramebuffer]\nfb_terminal=2", framebufferCore, 640, 240, true},
		{"video override", native + "[video=640x240]\nvga_scaler=1", "MENU", 640, 240, false},
		{"video group", native + "[Other]\n+video=640x240]\nfb_terminal=1", "MENU", 640, 240, false},
		{"video color", native + "[video=640x240]\nvideo_brightness=50", "MENU", 640, 240, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := nativeAnalogRoute([]byte(tc.ini), tc.core, tc.w, tc.h); got != tc.want {
				t.Fatalf("nativeAnalogRoute = %v, want %v", got, tc.want)
			}
		})
	}
}
