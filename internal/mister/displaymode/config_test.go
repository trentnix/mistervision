package displaymode

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCoreConfigPreservesOtherModes(t *testing.T) {
	for _, tc := range []struct{ name, setting, want string }{
		{"rgb", "ypbpr=0", "forced_scandoubler=1"},
		{"component", "ypbpr=1", "forced_scandoubler=1"},
		{"component-name", "vga_mode=ypbpr", "forced_scandoubler=1"},
		{"svideo", "vga_mode=svideo", "forced_scandoubler=1"},
		{"composite", "vga_mode=cvbs", "forced_scandoubler=1"},
		{"new-key-precedence", "ypbpr=1\nvga_mode=rgb", "forced_scandoubler=1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := []byte("[MiSTer]\r\n" + tc.setting + "\r\ncomposite_sync=1\r\n[Menu]\r\nvideo_mode=640,240,60\r\n[NeoGeo]\r\nforced_scandoubler=0\r\n")
			got, err := CoreConfig(original)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.HasPrefix(got, original) {
				t.Fatal("existing configuration changed")
			}
			if !strings.Contains(string(got), "[MiSTerVisionInterlaced]\nmain=MiSTer\ndirect_video=1\n"+tc.want) {
				t.Fatalf("wrong mode block: %s", got)
			}
			managed := string(got[len(original):])
			if !strings.Contains(managed, "\nlog_file_entry=1\n") {
				t.Fatal("interlaced core does not publish menu readiness")
			}
			again, err := CoreConfig(got)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(again, got) {
				t.Fatal("repeated launches changed configuration")
			}
		})
	}
}

func TestCoreConfigRefusesUnmanagedOrIncompleteSection(t *testing.T) {
	for _, text := range []string{"[MiSTerVisionInterlaced]\ncustom=1\n", blockStart + "\n[MiSTerVisionInterlaced]\n"} {
		if _, err := CoreConfig([]byte(text)); err == nil {
			t.Fatal("overwrote existing settings")
		}
	}
}

func TestLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "display.json")
	if c, err := Load(path); err != nil || c.Interlaced {
		t.Fatalf("default: %+v %v", c, err)
	}
	for _, tc := range []struct {
		data           string
		enabled, valid bool
	}{
		{`{"interlaced":true}`, true, true}, {`{"interlaced":false}`, false, true},
		{strings.Repeat(" ", 4097), false, false}, {`{}`, false, true}, {`null`, false, false}, {`{"interlcaed":true}`, false, false},
		{`{"interlaced":"true"}`, false, false}, {`{} {}`, false, false},
	} {
		if err := os.WriteFile(path, []byte(tc.data), 0600); err != nil {
			t.Fatal(err)
		}
		c, err := Load(path)
		if (err == nil) != tc.valid || c.Interlaced != tc.enabled {
			t.Fatalf("%s: %+v %v", tc.data, c, err)
		}
	}
}

func TestOwnedProcessesMatchWholeMarker(t *testing.T) {
	root := t.TempDir()
	for pid, value := range map[string]string{"10": "MISTERVISION_DISPLAY_OWNER=123", "11": "OTHER=MISTERVISION_DISPLAY_OWNER=123", "12": "MISTERVISION_DISPLAY_OWNER=1234"} {
		dir := filepath.Join(root, pid)
		os.Mkdir(dir, 0700)
		os.WriteFile(filepath.Join(dir, "environ"), []byte("BEFORE=1\x00"+value+"\x00AFTER=2\x00"), 0600)
	}
	got := ownedProcesses(root, "123")
	if len(got) != 1 || got[0] != 10 {
		t.Fatalf("owned processes: %v", got)
	}
}

func TestCoreConfigRejectsUnverifiedEncoderModes(t *testing.T) {
	if _, err := CoreConfig([]byte("[MiSTer]\nvga_mode=rgb_sc\n")); err == nil {
		t.Fatal("accepted unverified encoder mode")
	}
}

func TestMenuSettingsSectionMatching(t *testing.T) {
	for _, tc := range []struct {
		name, text, alias string
		want              map[string]string
	}{
		{"wildcard", "[MiSTer]\nvga_mode=rgb\n[Menu*]\nvga_mode=ypbpr\nvga_scaler=1\n", "", map[string]string{"vga_mode": "ypbpr", "vga_scaler": "1"}},
		{"included group", "[Other]\nvga_scaler=0\n+Menu\nvga_scaler=1\n+Unrelated\nvga_mode=ypbpr\n[Other]\nvga_scaler=0\n", "", map[string]string{"vga_scaler": "1", "vga_mode": "ypbpr"}},
		{"included wildcard", "[Other]\n+mEn* ; include Menu\nvga_scaler=1\n", "", map[string]string{"vga_scaler": "1"}},
		{"file order", "[Menu*]\nvga_scaler=1\n[MiSTer]\nvga_scaler=0\n[Menu]\nvga_scaler=1\n", "", map[string]string{"vga_scaler": "1"}},
		{"named core", "[MiSTerVision*]\nvga_mode=ypbpr\n[MiSTerVisionFramebuffer]\nvga_scaler=1\n", framebufferCore, map[string]string{"vga_mode": "ypbpr", "vga_scaler": "1"}},
		{"unrelated", "vga_scaler=1\n[Other]\nvga_scaler=1\n+Other*\nvga_mode=ypbpr\n", "", map[string]string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := menuSettings([]byte(tc.text), tc.alias)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCoreConfigComponentGroups(t *testing.T) {
	for _, section := range []string{"[Menu*]", "[Other]\n+Menu", "[MiSTerVision*]"} {
		t.Run(section, func(t *testing.T) {
			original := []byte("[MiSTer]\nvga_mode=rgb\n" + section + "\nvga_mode=ypbpr\n")
			got, err := CoreConfig(original)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.HasPrefix(got, original) || !bytes.Contains(got, []byte("forced_scandoubler=1\n")) {
				t.Fatalf("component routing lost: %s", got)
			}
		})
	}
}

// Existing component installations must receive the corrected framebuffer setting.
func TestCoreConfigRepairsComponentFramebuffer(t *testing.T) {
	original := "[MiSTer]\nvga_mode=ypbpr\nvga_sog=1\nforced_scandoubler=0\n"
	old := original + blockStart + "\n[MiSTerVisionInterlaced]\nforced_scandoubler=0\n" + blockEnd + "\n"
	got, err := CoreConfig([]byte(old))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(got), original) || !strings.Contains(string(got), "[MiSTerVisionInterlaced]\nmain=MiSTer\ndirect_video=1\nforced_scandoubler=1\n") {
		t.Fatalf("component framebuffer was not repaired without changing global settings: %s", got)
	}
}

// Y/C settings must survive scoped INI overrides without changing game cores.
func TestCoreConfigPreservesYCSettings(t *testing.T) {
	for _, mode := range []string{"svideo", "cvbs"} {
		for _, section := range []string{"[Menu]", "[Menu*]", "[Other]\n+Menu", "[MiSTerVision*]"} {
			original := []byte("[MiSTer]\nvga_mode=rgb\n" + section + "\nvga_mode=" + mode + "\nntsc_mode=1\nmenu_pal=1\nvga_sog=1\ncomposite_sync=1\n[NeoGeo]\nvga_mode=ypbpr\n")
			result, err := CoreConfig(original)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.HasPrefix(result, original) {
				t.Fatal("changed existing INI configuration")
			}
			settings := menuSettings(result, coreName)
			for key, want := range map[string]string{"vga_mode": mode, "ntsc_mode": "1", "menu_pal": "1", "vga_sog": "1", "composite_sync": "1", "direct_video": "1", "forced_scandoubler": "1", "fb_terminal": "1"} {
				if settings[key] != want {
					t.Fatalf("%s %s: %s=%q, want %q", mode, section, key, settings[key], want)
				}
			}
		}
	}
}

func TestBundledInterlacedCoreMatchesRuntime(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	manifestData, err := os.ReadFile(filepath.Join(root, "tools", "interlaced-core.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Version string
		Files   map[string]struct{ Path, SHA256 string }
	}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	spec := manifest.Files["InterlacedMenu.rbf"]
	data, err := os.ReadFile(filepath.Join(root, spec.Path))
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Version != coreVersion || spec.SHA256 != coreSHA256 || len(data) != coreSize || fmt.Sprintf("%x", sha256.Sum256(data)) != coreSHA256 {
		t.Fatal("bundled interlaced core, release manifest, and runtime verification disagree")
	}
	// A changed byte must fail before prepareCore reads the host INI or writes
	// a launch descriptor. A truncated or older core must fail as well.
	data[len(data)/2] ^= 1
	for _, sample := range [][]byte{data, data[:100]} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "InterlacedMenu.rbf"), sample, 0600); err != nil {
			t.Fatal(err)
		}
		_, err := prepareCore(dir)
		if err == nil || !strings.Contains(err.Error(), "interlaced core") {
			t.Fatalf("invalid core reached hardware setup: %v", err)
		}
		if _, err := os.Stat(filepath.Join(dir, "Interlaced.mgl")); !os.IsNotExist(err) {
			t.Fatal("invalid core wrote a launch descriptor")
		}
	}
}
