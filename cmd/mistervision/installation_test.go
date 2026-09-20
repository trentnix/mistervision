package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"mistervision/internal/mister/displaymode"
)

// Installation presets apply once, without replacing current or legacy choices.
func TestInstallationDisplayPreset(t *testing.T) {
	for _, tc := range []struct {
		name, preset, file, data string
		want, created            bool
	}{
		{"fresh progressive", "0", "", "", false, true},
		{"fresh interlaced", "1", "", "", true, true},
		{"existing progressive", "1", "settings.json", `{"display":{"interlaced":false},"ui":{"title":"Mine"}}`, false, false},
		{"existing interlaced", "0", "settings.json", `{"display":{"interlaced":true}}`, true, false},
		{"existing omitted mode", "1", "settings.json", `{}`, false, false},
		{"legacy display", "0", "display.json", `{"interlaced":true}`, true, false},
		{"legacy other settings", "1", "ui.json", `{"title":"Mine"}`, false, false},
		{"legacy connection", "1", "jellyfin.conf", "SERVER http://example.invalid", false, false},
		{"no launcher preset", "", "", "", false, false},
		{"invalid launcher preset", "invalid", "", "", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("MISTERVISION_INITIAL_INTERLACED", tc.preset)
			if tc.file != "" {
				if err := os.WriteFile(filepath.Join(dir, tc.file), []byte(tc.data), 0600); err != nil {
					t.Fatal(err)
				}
			}
			o := launchOptions{browse: true, config: filepath.Join(dir, "jellyfin.conf")}
			source, err := loadSettings(o)
			if err != nil {
				t.Fatal(err)
			}
			mode, err := displaymode.Parse(source.Section("display"))
			if err != nil || mode.Interlaced != tc.want {
				t.Fatalf("mode %+v: %v", mode, err)
			}
			if tc.file != "" {
				got, err := os.ReadFile(filepath.Join(dir, tc.file))
				if err != nil || !bytes.Equal(got, []byte(tc.data)) {
					t.Fatal("existing settings changed")
				}
			}
			_, err = os.Stat(filepath.Join(dir, "settings.json"))
			if (err == nil) != (tc.created || tc.file == "settings.json") {
				t.Fatalf("unexpected settings creation: %v", err)
			}
			// A later update installs the progressive launcher, but keeps the saved mode.
			t.Setenv("MISTERVISION_INITIAL_INTERLACED", "0")
			source, err = loadSettings(o)
			if err != nil {
				t.Fatal(err)
			}
			mode, err = displaymode.Parse(source.Section("display"))
			if err != nil || mode.Interlaced != tc.want {
				t.Fatal("update changed mode")
			}
		})
	}
}

func TestInstallationPresetDoesNotMaskInvalidSettings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	t.Setenv("MISTERVISION_INITIAL_INTERLACED", "1")
	if err := os.WriteFile(path, []byte("{broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadSettings(launchOptions{browse: true, config: filepath.Join(dir, "jellyfin.conf")}); err == nil {
		t.Fatal("malformed settings accepted")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "{broken" {
		t.Fatal("malformed settings overwritten")
	}
}
