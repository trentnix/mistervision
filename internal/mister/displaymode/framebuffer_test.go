package displaymode

import (
	"context"
	"errors"
	"fmt"
	"mistervision/internal/settings"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestFramebufferDivisor(t *testing.T) {
	for _, tc := range []struct{ w, h, mw, mh, div int }{
		{1920, 1080, 0, 0, 3}, {1920, 1080, 480, 270, 4}, {1280, 720, 0, 0, 2}, {640, 480, 0, 0, 1},
		{1920, 1080, 1280, 720, 2}, {1280, 720, 1280, 720, 1}, {1920, 1080, 1920, 1080, 1},
		{3840, 2160, 0, 0, 0}, {1920, 1080, 320, 240, 0},
	} {
		c := Config{FramebufferMaxWidth: tc.mw, FramebufferMaxHeight: tc.mh}
		div, err := c.FramebufferDivisor(tc.w, tc.h)
		if div != tc.div || (err != nil) != (tc.div == 0) {
			t.Fatalf("%+v: %d %v", tc, div, err)
		}
	}
}

func TestFramebufferLimitsValidation(t *testing.T) {
	for _, data := range []string{`{}`, `{"framebuffer_max_width":1280,"framebuffer_max_height":720}`, `{"framebuffer_max_width":0}`} {
		if _, err := Parse(settings.Section{Data: []byte(data)}); err != nil {
			t.Fatal(err)
		}
	}
	for _, data := range []string{`{"framebuffer_max_width":-1}`, `{"framebuffer_max_height":2160}`, `{"framebuffer_max_width":"1280"}`} {
		if _, err := Parse(settings.Section{Data: []byte(data)}); err == nil {
			t.Fatalf("accepted %s", data)
		}
	}
	for _, h := range []int{240, 288, 480, 576} {
		if !crtFramebuffer(640, h) {
			t.Fatalf("changed CRT %d", h)
		}
	}
	if crtFramebuffer(640, 360) || crtFramebuffer(480, 270) {
		t.Fatal("HDMI reduction misclassified as CRT")
	}
}

func TestFramebufferNegotiation(t *testing.T) {
	for _, tc := range []struct {
		name                string
		ignore, wrong, fail bool
		wantErr             bool
	}{
		{name: "acknowledged"}, {name: "ignored", ignore: true, wantErr: true},
		{name: "wrong-geometry", wrong: true, wantErr: true}, {name: "command-failed", fail: true, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			mode, revision := filepath.Join(dir, "mode"), filepath.Join(dir, "revision")
			write := func(path, value string) {
				t.Helper()
				if err := os.WriteFile(path, []byte(value), 0600); err != nil {
					t.Fatal(err)
				}
			}
			write(mode, "8888 1 960 540 3840")
			write(revision, "0")
			var commands []string
			f := framebufferControl{modePath: mode, revisionPath: revision, send: func(ctx context.Context, command string) error {
				commands = append(commands, command)
				if tc.fail {
					return errors.New("command failed")
				}
				if tc.ignore {
					return nil
				}
				div := 0
				if _, err := fmt.Sscanf(command, "fb_cmd0 8888 1 %d", &div); err != nil {
					t.Fatal(err)
				}
				if tc.wrong {
					div = 1
				}
				write(mode, fmt.Sprintf("8888 1 %d %d %d", 1920/div, 1080/div, 1920/div*4))
				write(revision, fmt.Sprint(len(commands)))
				return nil
			}}
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			err := f.configure(ctx, Config{})
			if (err != nil) != tc.wantErr {
				t.Fatalf("error %v", err)
			}
			if !tc.wantErr {
				if !reflect.DeepEqual(commands, []string{"fb_cmd0 8888 1 4", "fb_cmd0 8888 1 3"}) {
					t.Fatal(commands)
				}
				w, h, e := f.read()
				if e != nil || w != 640 || h != 360 {
					t.Fatalf("%dx%d %v", w, h, e)
				}
			}
		})
	}
}

func TestFramebufferFormatValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mode")
	for _, value := range []string{"", "garbage", "565 1 640 480 1280", "8888 1 640 480 1"} {
		if err := os.WriteFile(path, []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := (framebufferControl{modePath: path}).read(); err == nil {
			t.Fatalf("accepted %q", value)
		}
	}
}

func TestAspectConfiguration(t *testing.T) {
	for _, value := range []string{`{}`, `{"aspect_ratio":"auto"}`, `{"aspect_ratio":"4:3"}`, `{"aspect_ratio":"16:9"}`} {
		if _, err := Parse(settings.Section{Data: []byte(value)}); err != nil {
			t.Fatal(err)
		}
	}
	for _, value := range []string{`{"aspect_ratio":"wide"}`, `{"aspect_ratio":16}`} {
		if _, err := Parse(settings.Section{Data: []byte(value)}); err == nil {
			t.Fatalf("accepted %s", value)
		}
	}
}
