package displaymode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mistervision/internal/diagnostics"
)

func TestStateLogAllowlist(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"tmp/CORENAME":                                "private-server-token",
		"sys/class/tty/tty0/active":                   "tty3\n",
		"sys/module/MiSTer_fb/parameters/mode":        "8888 1 640 240 2560",
		"media/fat/config/consolemode_crt.bin":        string([]byte{1}),
		"media/fat/config/consolemode_video_mode.bin": string([]byte{2, 0, 0, 0}),
		"media/fat/config/consolemode_rotation.bin":   "secret-oversized-setting",
		"proc/42/comm":                                consoleModeHost + "\n", "proc/42/status": "State:\tS (sleeping)\n",
	}
	for name, data := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(t.TempDir(), "debug.log")
	log, err := diagnostics.Open(diagnostics.Config{Enabled: true, Path: path, MaxBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	recordState(log, "startup", root)
	if err = log.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, secret := range []string{"private-server-token", "secret-oversized-setting", root} {
		if strings.Contains(text, secret) {
			t.Fatalf("exposed private text: %s", text)
		}
	}
	var events []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		var e map[string]any
		if err = json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatal(err)
		}
		events = append(events, e)
	}
	if len(events) != 5 {
		t.Fatalf("unexpected events: %s", text)
	}
	if events[0]["core"] != "other" || events[0]["consolemode_host"] != true || events[0]["active_vt"] != float64(3) {
		t.Fatal(events[0])
	}
	if events[1]["width"] != float64(640) || events[1]["height"] != float64(240) {
		t.Fatal(events[1])
	}
	if events[2]["raw_value"] != float64(1) || events[3]["raw_value"] != float64(2) {
		t.Fatal(events)
	}
	if _, ok := events[4]["raw_value"]; ok {
		t.Fatal("oversized value recorded")
	}
	if events[4]["error_kind"] == nil {
		t.Fatal("missing malformed-value indication")
	}
	RecordState(nil, "disabled")
}

func TestObservationReadBound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "value")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", 1024)), 0600); err != nil {
		t.Fatal(err)
	}
	if data, err := readObservation(path, 4); err == nil || data != nil {
		t.Fatal("oversized observation accepted")
	}
}

func TestConsoleModePreflight(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "ConsoleMode"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"MiSTer", "menu.rbf", "ConsoleMode/MiSTer_ConsoleMode", consoleModeCore, "ConsoleMode/ConsoleMode_arm"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("test"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := validateConsoleModeFiles(root); err != nil {
		t.Fatal(err)
	}
	host := filepath.Join(root, "MiSTer")
	if err := os.Chmod(host, 0600); err != nil {
		t.Fatal(err)
	}
	if err := validateConsoleModeFiles(root); err == nil {
		t.Fatal("accepted non-executable host")
	}
	if err := os.Chmod(host, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, consoleModeCore), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := validateConsoleModeFiles(root); err == nil {
		t.Fatal("accepted empty return core")
	}
	if err := os.Remove(host); err != nil {
		t.Fatal(err)
	}
	if err := validateConsoleModeFiles(root); err == nil {
		t.Fatal("accepted missing host")
	}
}
