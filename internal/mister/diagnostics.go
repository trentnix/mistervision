// Package mister provides MiSTer-specific startup observations. Display control
// and background-music coordination remain in their respective subpackages.
package mister

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"mistervision/internal/diagnostics"
	"mistervision/internal/mister/displaymode"
)

// RecordStartup reads a bounded snapshot of display settings when logging is
// enabled. INI entries describe configured values, not resolved hardware state.
// It never changes settings or reads credentials, device serials, or environments.
func RecordStartup(log *diagnostics.Log, interlaced bool) {
	if log == nil {
		return
	}
	log.Record("mister.display", slog.Bool("interlaced", interlaced))
	displaymode.RecordState(log, "startup")
	recordActiveINI(log)
	f, err := os.Open("/sys/module/MiSTer_fb/parameters/mode")
	if err != nil {
		log.Record("mister.framebuffer", slog.String("error_kind", diagnostics.ErrorKind(err)))
		return
	}
	defer f.Close()
	var format, swap, width, height, stride int
	n, err := fmt.Fscanf(io.LimitReader(f, 256), "%d %d %d %d %d", &format, &swap, &width, &height, &stride)
	if err != nil || n != 5 {
		log.Record("mister.framebuffer", slog.String("error_kind", "invalid-mode"))
		return
	}
	log.Record("mister.framebuffer", slog.Int("format", format), slog.Int("swap", swap), slog.Int("width", width), slog.Int("height", height), slog.Int("stride", stride))
}

// recordActiveINI records the selected profile and only its allowed display keys.
// A selection failure must not substitute Main and produce misleading evidence.
func recordActiveINI(log *diagnostics.Log) {
	ini, err := displaymode.ActiveINIPath()
	if err != nil {
		log.Record("mister.ini_profile", slog.String("error_kind", diagnostics.ErrorKind(err)))
		return
	}
	log.Record("mister.ini_profile", slog.String("file", filepath.Base(ini)))
	f, err := os.Open(ini)
	if err != nil {
		log.Record("mister.settings", slog.String("error_kind", diagnostics.ErrorKind(err)))
		return
	}
	defer f.Close()
	recordSettings(log, f)
}

// recordSettings labels each entry with its known matched section, not arbitrary
// source text. Wildcard and included groups follow the startup matching rules.
// Only display-related sections, numeric values, and known host/connector names are eligible. Limits
// bound startup reads and queue use even when the INI is unexpectedly large.
func recordSettings(log *diagnostics.Log, source io.Reader) {
	const limit = 128 << 10
	reader := &io.LimitedReader{R: source, N: limit + 1}
	scan := bufio.NewScanner(reader)
	section := "top"
	entries, rejected := 0, 0
	for scan.Scan() {
		line := strings.TrimSpace(strings.SplitN(scan.Text(), ";", 2)[0])
		if strings.HasPrefix(line, "[") {
			section = displaymode.MatchMenuSection(line[1:], "MiSTerVisionInterlaced", "MiSTerVisionFramebuffer")
			continue
		}
		if strings.HasPrefix(line, "+") {
			if section == "" || section == "top" {
				section = displaymode.MatchMenuSection(line[1:], "MiSTerVisionInterlaced", "MiSTerVisionFramebuffer")
			}
			continue
		}
		if section == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key, value = strings.ToLower(strings.TrimSpace(key)), strings.TrimSpace(value)
		switch key {
		case "ypbpr", "composite_sync", "forced_scandoubler", "vga_scaler", "direct_video", "vsync_adjust", "video_mode", "video_mode_ntsc", "video_mode_pal", "fb_terminal", "fb_size", "vscale_mode", "vscale_border", "ntsc_mode", "log_file_entry":
		case "vga_mode":
			if value != "rgb" && value != "ypbpr" && value != "svideo" && value != "cvbs" {
				rejected++
				continue
			}
		case "main":
			if value != "MiSTer" && value != "ConsoleMode/MiSTer_ConsoleMode" {
				rejected++
				continue
			}
		default:
			continue
		}
		if key != "vga_mode" && key != "main" && !numericSetting(value) {
			rejected++
			continue
		}
		if entries == 64 {
			break
		}
		log.Record("mister.setting", slog.String("section", section), slog.String("key", key), slog.String("value", value))
		entries++
	}
	log.Record("mister.settings", slog.Int("entries", entries), slog.Int("rejected_values", rejected), slog.Bool("limited", reader.N == 0 || entries == 64), slog.Bool("read_failed", scan.Err() != nil))
}

// numericSetting permits numeric mode indices and timing tuples, never arbitrary
// INI strings. Unknown value formats are counted but not copied into the log.
func numericSetting(value string) bool {
	if len(value) == 0 || len(value) > 192 {
		return false
	}
	digit := false
	for _, c := range value {
		if c >= '0' && c <= '9' {
			digit = true
			continue
		}
		if !strings.ContainsRune(" ,.+-\t", c) {
			return false
		}
	}
	return digit
}
