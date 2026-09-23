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
		got, err := consoleModeConfig(original)
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
		again, err := consoleModeConfig(got)
		if err != nil || string(again) != string(got) {
			t.Fatalf("not idempotent: %v", err)
		}
	}
}

func TestConsoleModeConfigRejectsConflicts(t *testing.T) {
	for _, text := range []string{consoleModeBlockStart + "\n", "[MiSTerVisionFramebuffer]\nvideo_mode=8\n"} {
		if _, err := consoleModeConfig([]byte(text)); err == nil {
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
