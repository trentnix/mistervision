package displaymode

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const consoleModeBlockStart = "; BEGIN MiSTerVision ConsoleMode handoff"
const consoleModeBlockEnd = "; END MiSTerVision ConsoleMode handoff"

// consoleModeConfig adds only host and console settings. The original Menu
// section still matches the underlying MENU core, preserving display timing.
func consoleModeConfig(data []byte) ([]byte, error) {
	text := string(data)
	if start := strings.Index(text, consoleModeBlockStart); start >= 0 {
		end := strings.Index(text[start:], consoleModeBlockEnd)
		if end < 0 {
			return nil, errors.New("incomplete MiSTerVision ConsoleMode configuration block")
		}
		end += start + len(consoleModeBlockEnd)
		if end < len(text) && text[end] == '\n' {
			end++
		}
		text = text[:start] + text[end:]
	}
	if strings.Contains(strings.ToLower(text), "["+strings.ToLower(framebufferCore)+"]") {
		return nil, errors.New("MiSTerVisionFramebuffer INI section already exists outside the managed block")
	}
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return []byte(text + fmt.Sprintf("%s\n[%s]\nmain=MiSTer\nfb_terminal=1\nlog_file_entry=1\n%s\n", consoleModeBlockStart, framebufferCore, consoleModeBlockEnd)), nil
}

// prepareConsoleModeCore verifies the return path before changing any hardware.
func prepareConsoleModeCore(directory string, interlaced bool) (string, error) {
	const root = "/media/fat"
	if err := validateConsoleModeFiles(root); err != nil {
		return "", err
	}
	if interlaced {
		return prepareCore(directory)
	}
	ini, err := ActiveINIPath()
	if err != nil {
		return "", err
	}
	original, err := os.ReadFile(ini)
	if err != nil {
		return "", err
	}
	crt, err := os.ReadFile(root + "/config/consolemode_crt.bin")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	values := menuSettings(original, framebufferCore)
	if len(crt) > 0 && crt[0] != 0 && values["vga_scaler"] != "1" && values["direct_video"] != "1" {
		return "", errors.New("ConsoleMode CRT output needs a working CRT framebuffer configuration: set vga_scaler=1 with CRT-compatible video_mode in the active MiSTer INI [Menu]")
	}
	directory, err = filepath.Abs(directory)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(directory, root+"/") || strings.ContainsAny(directory, "\r\n") {
		return "", errors.New("ConsoleMode handoff settings must be on the SD card")
	}
	if err := configureINI(ini, directory, "consolemode", consoleModeConfig); err != nil {
		return "", err
	}
	mgl := filepath.Join(directory, "Framebuffer.mgl")
	if err = replace(mgl, []byte("<mistergamedescription><rbf>menu</rbf><setname>"+framebufferCore+"</setname></mistergamedescription>\n")); err != nil {
		return "", err
	}
	return mgl, nil
}

// validateConsoleModeFiles checks both handoff directions before touching the
// core. A regular but non-executable host cannot take ownership of the display.
func validateConsoleModeFiles(root string) error {
	for _, file := range []struct {
		name       string
		executable bool
	}{
		{"MiSTer", true}, {"menu.rbf", false}, {"ConsoleMode/MiSTer_ConsoleMode", true}, {consoleModeCore, false}, {"ConsoleMode/ConsoleMode_arm", true},
	} {
		info, err := os.Stat(filepath.Join(root, file.name))
		if err != nil || !info.Mode().IsRegular() || info.Size() == 0 || (file.executable && info.Mode().Perm()&0111 == 0) {
			return fmt.Errorf("ConsoleMode handoff requires a usable %s", filepath.Join(root, file.name))
		}
	}
	return nil
}
