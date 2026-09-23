package displaymode

import (
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"mistervision/internal/diagnostics"
)

// RecordState records bounded observations of display ownership and ConsoleMode
// selections. Missing files are observations, not startup failures. No arbitrary
// core names, process arguments, environment values, or configuration text escape.
func RecordState(log *diagnostics.Log, stage string) {
	recordState(log, stage, "/")
}

// recordState accepts a filesystem root so tests can exercise real file parsing
// without reading or changing the development machine's display state.
func recordState(log *diagnostics.Log, stage, root string) {
	if log == nil {
		return
	}
	proc := filepath.Join(root, "proc")
	core := "unavailable"
	if data, err := readObservation(filepath.Join(root, "tmp/CORENAME"), 128); err == nil {
		switch value := strings.TrimSpace(string(data)); value {
		case "MENU", coreName, framebufferCore:
			core = value
		default:
			core = "other"
		}
	}
	vt := 0
	if data, err := readObservation(filepath.Join(root, "sys/class/tty/tty0/active"), 16); err == nil {
		if n, e := strconv.Atoi(strings.TrimPrefix(strings.TrimSpace(string(data)), "tty")); e == nil && n >= 1 && n <= 63 {
			vt = n
		}
	}
	log.Record("mister.state", slog.String("stage", stage), slog.Bool("standard_host", processRunning(proc, "MiSTer")), slog.Bool("consolemode_host", processRunning(proc, consoleModeHost)), slog.Bool("consolemode_frontend", processRunning(proc, "ConsoleMode_arm")), slog.String("core", core), slog.Int("active_vt", vt))
	if data, err := readObservation(filepath.Join(root, "sys/module/MiSTer_fb/parameters/mode"), 128); err == nil {
		var format, swap, w, h, stride int
		if n, e := fmt.Sscanf(string(data), "%d %d %d %d %d", &format, &swap, &w, &h, &stride); e == nil && n == 5 {
			log.Record("mister.state.framebuffer", slog.String("stage", stage), slog.Int("format", format), slog.Int("swap", swap), slog.Int("width", w), slog.Int("height", h), slog.Int("stride", stride))
		} else {
			log.Record("mister.state.framebuffer", slog.String("stage", stage), slog.String("error_kind", "invalid-mode"))
		}
	} else {
		log.Record("mister.state.framebuffer", slog.String("stage", stage), slog.String("error_kind", diagnostics.ErrorKind(err)))
	}
	for _, setting := range []string{"crt", "video_mode", "rotation"} {
		path := filepath.Join(root, "media/fat/config/consolemode_"+setting+".bin")
		data, err := readObservation(path, 4)
		attrs := []slog.Attr{slog.String("stage", stage), slog.String("setting", setting)}
		switch {
		case err != nil:
			attrs = append(attrs, slog.String("error_kind", diagnostics.ErrorKind(err)))
		case len(data) == 1:
			attrs = append(attrs, slog.Int("bytes", 1), slog.Uint64("raw_value", uint64(data[0])))
		case len(data) == 4:
			attrs = append(attrs, slog.Int("bytes", 4), slog.Uint64("raw_value", uint64(binary.LittleEndian.Uint32(data))))
		default:
			attrs = append(attrs, slog.String("error_kind", "invalid-size"))
		}
		log.Record("mister.consolemode.setting", attrs...)
	}
}

// readObservation bounds reads of the known diagnostic files, including malformed
// files. The caller records only a parsed value or a classified read failure.
func readObservation(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err == nil && int64(len(data)) > limit {
		return nil, fmt.Errorf("diagnostic observation exceeds %d bytes", limit)
	}
	return data, err
}
