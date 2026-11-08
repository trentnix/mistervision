package displaymode

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestConsoleModeConfigPreservesDisplay(t *testing.T) {
	for _, timing := range []string{"8", "640,30,60,70,240,4,4,14,12587", "640,20,60,80,288,2,3,19,12500"} {
		original := []byte("[MiSTer]\nmain=ConsoleMode/MiSTer_ConsoleMode\nvga_mode=cvbs\n[Menu]\nvga_scaler=1\nvideo_mode=" + timing + "\n[OtherCore]\nvideo_mode=6\n")
		got, err := consoleModeConfig(original, true)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(string(got), string(original)) {
			t.Fatal("existing configuration changed")
		}
		added := string(got[len(original):])
		for _, forbidden := range []string{"video_mode=", "vga_scaler=", "direct_video="} {
			if strings.Contains(added, forbidden) {
				t.Fatalf("handoff overrides output: %s", added)
			}
		}
		again, err := consoleModeConfig(got, true)
		if err != nil || string(again) != string(got) {
			t.Fatalf("not idempotent: %v", err)
		}
	}
}

func TestConsoleModeConfigRejectsConflicts(t *testing.T) {
	for _, text := range []string{consoleModeBlockStart + "\n", "[MiSTerVisionFramebuffer]\nvideo_mode=8\n"} {
		if _, err := consoleModeConfig([]byte(text), true); err == nil {
			t.Fatalf("accepted conflicting block: %q", text)
		}
	}
}

func TestConsoleModeMenuSettings(t *testing.T) {
	got := menuSettings([]byte("[MiSTer]\nvga_scaler=0\nvga_mode=rgb\n[Menu]\nvga_scaler=1 ; analog framebuffer\nvideo_mode=custom\n[Other]\nvga_scaler=0\n"))
	want := map[string]string{"vga_scaler": "1", "vga_mode": "rgb", "video_mode": "custom"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestConsoleModeReturn(t *testing.T) {
	want := []string{"send load_core " + consoleModeCore, "wait " + consoleModeHost, "request", "send load_core " + consoleModeCore, "wait ConsoleMode_arm"}
	for fail := -1; fail < len(want); fail++ {
		var calls []string
		failure := errors.New("handoff failed")
		record := func(s string) error {
			calls = append(calls, s)
			if len(calls)-1 == fail {
				return failure
			}
			return nil
		}
		err := returnConsoleMode(context.Background(), func(_ context.Context, s string) error { return record("send " + s) }, func(_ context.Context, s string, on bool) error {
			if !on {
				t.Fatal("return must wait for live owner")
			}
			return record("wait " + s)
		}, func() error { return record("request") })
		if fail < 0 {
			if err != nil || !reflect.DeepEqual(calls, want) {
				t.Fatalf("return: %v %v", calls, err)
			}
		} else if !errors.Is(err, failure) || !reflect.DeepEqual(calls, want[:fail+1]) {
			t.Fatalf("failure %d: %v %v", fail, calls, err)
		}
	}
}

func TestConsoleModeProcessDetection(t *testing.T) {
	root := t.TempDir()
	for name, state := range map[string]string{"12": "Z (zombie)", "13": "S (sleeping)"} {
		path := filepath.Join(root, name)
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
		for file, data := range map[string]string{"comm": consoleModeHost + "\n", "status": "State:\t" + state + "\n"} {
			if err := os.WriteFile(filepath.Join(path, file), []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	if !processRunning(root, consoleModeHost) {
		t.Fatal("live ConsoleMode not detected")
	}
	if processRunning(root, "MiSTer") {
		t.Fatal("matched a different host")
	}
	if err := os.RemoveAll(filepath.Join(root, "13")); err != nil {
		t.Fatal(err)
	}
	if processRunning(root, consoleModeHost) {
		t.Fatal("zombie treated as an active host")
	}
}

// Exercise route selection through the generated handoff configuration, including
// migration of the override written by earlier versions.
func TestConsoleModeFramebufferRoutes(t *testing.T) {
	const native = "[MiSTer]\nmain=ConsoleMode/MiSTer_ConsoleMode\nvga_mode=svideo\n[Menu]\nfb_terminal=2\nvga_scaler=0\ndirect_video=0\nforced_scandoubler=0\nmenu_pal=0\n"
	const oldBlock = consoleModeBlockStart + "\n[MiSTerVisionFramebuffer]\nmain=MiSTer\nfb_terminal=1\nlog_file_entry=1\n" + consoleModeBlockEnd + "\n"
	for _, tc := range []struct {
		name, ini             string
		crt                   bool
		terminal              string
		wantNative, wantError bool
	}{
		{"native NTSC", native, true, "2", true, false},
		{"native PAL", native + "menu_pal=1\n", true, "2", true, false},
		{"native defaults", "[MiSTer]\nfb_terminal=2\n", true, "2", true, false},
		{"old managed override", native + oldBlock, true, "2", true, false},
		{"native when HDMI selected", native, false, "2", true, false},
		{"scaler", "[MiSTer]\nvga_mode=ypbpr\n[Menu]\nvga_scaler=1\n", true, "1", false, false},
		{"direct video", "[Menu]\ndirect_video=1\n", true, "1", false, false},
		{"HDMI", "[MiSTer]\nvga_scaler=0\n", false, "1", false, false},
		{"missing CRT route", "[MiSTer]\nvga_scaler=0\n", true, "", false, true},
		{"native scandoubled", native + "forced_scandoubler=1\n", true, "", false, true},
		{"native disabled", native + "fb_terminal=0\n", true, "", false, true},
		{"invalid native flag", native + "fb_terminal=2invalid\n", true, "", false, true},
		{"named scaler override", native + "[MiSTerVision*]\nvga_scaler=1\n", true, "1", false, false},
		{"named native override", "[Menu]\nvga_scaler=1\n[MiSTerVision*]\nvga_scaler=0\nfb_terminal=2\n", true, "2", true, false},
		{"timing override", native + "[video=640x240]\nfb_terminal=1\n", true, "", false, true},
		{"timing color only", native + "[video=640x240]\nvideo_brightness=50\n", true, "2", true, false},
		{"unrelated core", native + "[ao486]\nfb_terminal=1\nvga_scaler=1\n", true, "2", true, false},
		{"changed back to scaler", native + "vga_scaler=1\n" + oldBlock, true, "1", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := consoleModeConfig([]byte(tc.ini), tc.crt)
			if (err != nil) != tc.wantError {
				t.Fatalf("error = %v, want error %v", err, tc.wantError)
			}
			if tc.wantError {
				return
			}
			values := menuSettings(got, framebufferCore)
			if values["main"] != "mister" || values["fb_terminal"] != tc.terminal {
				t.Fatalf("incorrect handoff: %v", values)
			}
			for _, size := range [][2]int{{640, 240}, {640, 288}, {640, 480}, {640, 360}, {1280, 720}} {
				want := tc.wantNative && size[0] == 640 && (size[1] == 240 || size[1] == 288)
				if nativeAnalogRoute(got, framebufferCore, size[0], size[1]) != want {
					t.Fatalf("incorrect playback route for %v", size)
				}
			}
			again, err := consoleModeConfig(got, tc.crt)
			if err != nil || string(again) != string(got) {
				t.Fatalf("not idempotent: %v", err)
			}
		})
	}
}

func TestConsoleModeInvalidCRTRouteDoesNotChangeINI(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "MiSTer_SVID.ini")
	original := []byte("[MiSTer]\nvga_mode=svideo\n[Menu]\nvga_scaler=0\nfb_terminal=1\n")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	err := configureINI(path, root, "consolemode", func(data []byte) ([]byte, error) { return consoleModeConfig(data, true) })
	if err == nil {
		t.Fatal("invalid CRT route accepted")
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil || string(got) != string(original) {
		t.Fatal("invalid configuration was modified", readErr)
	}
	if _, err := os.Stat(path + ".before-consolemode"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("validation should precede backup", err)
	}
}
