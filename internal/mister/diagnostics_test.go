package mister

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mistervision/internal/diagnostics"
)

func TestSettingsLogPreservesSectionsAndExcludesPrivateValues(t *testing.T) {
	data := settingsLog(t, strings.NewReader(`
ypbpr=0
password=secret-password
[MiSTer]
video_mode=640,480,60 ; secret comment
[Menu]
vsync_adjust=2
[MiSTerVisionInterlaced]
video_mode_ntsc=640,16,64,80,480,1,3,14,12.587
[private-core]
video_mode=42
[Menu]
video_mode=secret-value
`))
	for _, secret := range []string{"secret", "password", "private-core"} {
		if strings.Contains(data, secret) {
			t.Fatalf("log exposed %s", secret)
		}
	}
	var settings []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(data), "\n") {
		var e map[string]any
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatal(err)
		}
		if e["msg"] == "mister.setting" {
			settings = append(settings, e)
		}
	}
	if len(settings) != 4 || settings[0]["section"] != "top" || settings[1]["section"] != "mister" || settings[2]["section"] != "menu" || settings[3]["section"] != "mistervisioninterlaced" {
		t.Fatal(settings)
	}
	if !strings.Contains(data, `"rejected_values":1`) {
		t.Fatal(data)
	}
}

func TestSettingsReadAndEventCountsAreBounded(t *testing.T) {
	for _, source := range []string{strings.Repeat("ypbpr=0\n", 10000), strings.Repeat(";ignored\n", 20000)} {
		data := settingsLog(t, strings.NewReader(source))
		if !strings.Contains(data, `"limited":true`) {
			t.Fatal("missing truncation indication")
		}
		if strings.Count(data, `"msg":"mister.setting"`) > 64 {
			t.Fatal("unbounded settings inventory")
		}
	}
	RecordStartup(nil, false) // Disabled logging performs no hardware reads.
}

func TestNumericSettingsRejectUnrecognizedText(t *testing.T) {
	for _, value := range []string{"", "secret.invalid", "1\n2", "1=2", "+", strings.Repeat("1", 193)} {
		if numericSetting(value) {
			t.Fatalf("accepted %q", value)
		}
	}
	for _, value := range []string{"0", "-1", "640,16,64,80,480,1,3,14,12.587"} {
		if !numericSetting(value) {
			t.Fatalf("rejected %q", value)
		}
	}
}

// settingsLog exercises the real bounded writer with a synthetic INI source.
func settingsLog(t *testing.T, source io.Reader) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "debug.log")
	log, err := diagnostics.Open(diagnostics.Config{Enabled: true, Path: path, MaxBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	recordSettings(log, source)
	if err := log.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestConsoleModeINISettingsAreAllowlisted(t *testing.T) {
	data := settingsLog(t, strings.NewReader(`[Menu]
vga_mode=cvbs
main=ConsoleMode/MiSTer_ConsoleMode
fb_terminal=1
fb_size=1
[MiSTerVisionFramebuffer]
main=MiSTer
log_file_entry=1
vga_mode=secret-connector
main=secret-host
`))
	if strings.Contains(data, "secret") {
		t.Fatal(data)
	}
	for _, want := range []string{`"value":"cvbs"`, `"value":"ConsoleMode/MiSTer_ConsoleMode"`, `"section":"mistervisionframebuffer"`, `"key":"fb_terminal"`, `"key":"log_file_entry"`, `"rejected_values":2`} {
		if !strings.Contains(data, want) {
			t.Fatalf("missing %s: %s", want, data)
		}
	}
}

func TestSettingsLogIncludesWildcardAndGroupedDisplaySections(t *testing.T) {
	for _, tc := range []struct{ name, header, section string }{
		{"wildcard", "[MeNu*]", "menu"},
		{"group", "[private-core]\n+Menu", "menu"},
		{"wildcard group", "[private-core]\n+Men*", "menu"},
		{"named core", "[MiSTerVisionFrame*]", "mistervisionframebuffer"},
		// Main matches only the prefix before *, but the log must not copy the suffix.
		{"private suffix", "[Menu*private-section]", "menu"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := settingsLog(t, strings.NewReader(tc.header+` ; private-comment
vga_scaler=1
vga_mode=ypbpr
password=private-password
video_mode=private-value
main=private-host
+private-other
fb_terminal=1
[private-core]
video_mode=99
`))
			if strings.Contains(data, "private") {
				t.Fatalf("private text leaked: %s", data)
			}
			var entries []map[string]any
			for _, line := range strings.Split(strings.TrimSpace(data), "\n") {
				var event map[string]any
				if err := json.Unmarshal([]byte(line), &event); err != nil {
					t.Fatal(err)
				}
				if event["msg"] == "mister.setting" {
					entries = append(entries, event)
				}
			}
			if len(entries) != 3 {
				t.Fatalf("missing or unrelated settings: %s", data)
			}
			for i, key := range []string{"vga_scaler", "vga_mode", "fb_terminal"} {
				if entries[i]["key"] != key || entries[i]["section"] != tc.section {
					t.Fatal(entries)
				}
			}
			if !strings.Contains(data, `"rejected_values":2`) {
				t.Fatal(data)
			}
		})
	}
}

func TestGroupedSettingsLogRemainsBounded(t *testing.T) {
	for _, header := range []string{"[Menu*]", "[private-core]\n+Menu"} {
		data := settingsLog(t, strings.NewReader(header+"\n"+strings.Repeat("vga_scaler=1\n", 10000)))
		if strings.Count(data, `"msg":"mister.setting"`) != 64 || !strings.Contains(data, `"limited":true`) {
			t.Fatal("grouped settings bypassed event limit")
		}
	}
}
