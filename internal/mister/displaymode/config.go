// Package displaymode owns session-scoped MiSTer display changes. It selects
// an optional interlaced core or a bounded framebuffer before the client opens
// the display, then restores Menu after the child exits.
package displaymode

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mistervision/internal/settings"
)

// Config selects native output at startup. Defaults preserve CRT rasters and
// bound software rendering on other outputs. Interlaced uses the bundled core.
type Config struct {
	// Interlaced loads the supported standalone core for this application run.
	Interlaced bool `json:"interlaced"`
	// AspectRatio describes the screen, independently of the media aspect ratio.
	// Empty and auto preserve known CRT modes and otherwise assume square pixels.
	AspectRatio string `json:"aspect_ratio"`
	// FramebufferMaxWidth and FramebufferMaxHeight bound software rendering on
	// non-CRT outputs. Zero selects 640 and 480. Native CRT rasters are preserved.
	FramebufferMaxWidth  int `json:"framebuffer_max_width"`
	FramebufferMaxHeight int `json:"framebuffer_max_height"`
}

// Load reads legacy display settings. A missing file selects conservative defaults.
func Load(path string) (Config, error) { return Parse(settings.Read(path, 4096, false)) }

// Parse validates the display section before any hardware changes occur.
func Parse(source settings.Section) (Config, error) {
	var c Config
	if err := source.Decode(&c); err != nil {
		return c, fmt.Errorf("display configuration %s: %w", source.Path, err)
	}
	if (c.FramebufferMaxWidth != 0 && (c.FramebufferMaxWidth < 320 || c.FramebufferMaxWidth > 1920)) ||
		(c.FramebufferMaxHeight != 0 && (c.FramebufferMaxHeight < 240 || c.FramebufferMaxHeight > 1080)) {
		return Config{}, errors.New("display framebuffer limits must be 320–1920 pixels wide and 240–1080 pixels high")
	}
	switch c.AspectRatio {
	case "", "auto", "4:3", "16:9":
	default:
		return Config{}, errors.New("display aspect_ratio must be auto, 4:3, or 16:9")
	}
	return c, nil
}

const coreName = "MiSTerVisionInterlaced"
const blockStart = "; BEGIN MiSTerVision interlaced output"
const blockEnd = "; END MiSTerVision interlaced output"

// CoreConfig preserves existing INI text and adds an isolated MGL-name section.
// The section is inert for the ordinary Menu core and other applications.
// RGB and component inherit their existing color/sync settings unchanged.
func CoreConfig(data []byte) ([]byte, error) {
	text := string(data)
	if start := strings.Index(text, blockStart); start >= 0 {
		end := strings.Index(text[start:], blockEnd)
		if end < 0 {
			return nil, errors.New("incomplete MiSTerVision interlaced configuration block")
		}
		end += start + len(blockEnd)
		if end < len(text) && text[end] == '\n' {
			end++
		}
		text = text[:start] + text[end:]
	}
	if strings.Contains(strings.ToLower(text), "["+strings.ToLower(coreName)+"]") {
		return nil, errors.New("MiSTerVisionInterlaced INI section already exists outside the managed block")
	}
	values := menuSettings([]byte(text))
	scandoubler := 1
	component := values["ypbpr"] == "1"
	if mode := values["vga_mode"]; mode != "" {
		if mode != "rgb" && mode != "ypbpr" {
			return nil, errors.New("interlaced output currently supports RGB or component vga_mode")
		}
		component = mode == "ypbpr"
	}
	if component {
		scandoubler = 0
	}
	block := fmt.Sprintf("%s\n[%s]\nmain=MiSTer\ndirect_video=1\nforced_scandoubler=%d\nfb_size=1\nfb_terminal=1\nlog_file_entry=1\n%s\n", blockStart, coreName, scandoubler, blockEnd)
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return []byte(text + block), nil
}

// prepareCore verifies the bitstream, preserves existing INI settings, and
// writes the launch descriptor. The returned path is ready for Main to load.
func prepareCore(directory string) (string, error) {
	const root = "/media/fat"
	core := filepath.Join(directory, "InterlacedMenu.rbf")
	info, statErr := os.Stat(core)
	if statErr != nil {
		return "", fmt.Errorf("install standalone InterlacedMenu.rbf v0.0.1 at %s: %w", core, statErr)
	}
	if info.Size() != 2489556 {
		return "", errors.New("unsupported interlaced core: install InterlacedMenu.rbf v0.0.1")
	}
	bitstream, readErr := os.ReadFile(core)
	if readErr != nil {
		return "", readErr
	}
	if fmt.Sprintf("%x", sha256.Sum256(bitstream)) != "0158e0338a00441271f38be0703c22253d53ea39b60a1a96b7ec964bedae8999" {
		return "", errors.New("interlaced core checksum does not match v0.0.1")
	}
	original, err := os.ReadFile(root + "/MiSTer.ini")
	if err != nil {
		return "", err
	}
	configured, err := CoreConfig(original)
	if err != nil {
		return "", err
	}
	if string(configured) != string(original) {
		backup := root + "/mistervision/MiSTer.ini.before-interlaced"
		if _, e := os.Stat(backup); errors.Is(e, os.ErrNotExist) {
			if e = os.WriteFile(backup, original, 0600); e != nil {
				return "", e
			}
		}
		if err := replace(root+"/MiSTer.ini", configured); err != nil {
			return "", err
		}
	}
	// MGL resolves RBF paths relative to the SD card root, not the descriptor.
	relative, err := filepath.Rel(root, core)
	if err != nil || strings.HasPrefix(relative, "..") || strings.ContainsAny(relative, "<&>\r\n") {
		return "", errors.New("interlaced core must be inside /media/fat with an XML-safe path")
	}
	mgl := filepath.Join(directory, "Interlaced.mgl")
	xml := "<mistergamedescription><rbf>" + strings.TrimSuffix(relative, ".rbf") + "</rbf><setname>" + coreName + "</setname></mistergamedescription>\n"
	if err := replace(mgl, []byte(xml)); err != nil {
		return "", err
	}
	return mgl, nil
}

// replace publishes a complete file before it can be read during a core load.
func replace(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".interlaced-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	err = errors.Join(err, f.Close())
	if err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

// menuSettings reads effective global/Menu values without interpreting timings.
// Core-specific handoff settings must not change the user's analog routing.
func menuSettings(data []byte) map[string]string {
	values := map[string]string{}
	section := ""
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(strings.SplitN(line, ";", 2)[0])
		if strings.HasPrefix(line, "[") {
			section = strings.ToLower(strings.TrimSpace(strings.Trim(line, "[]")))
			continue
		}
		if section != "mister" && section != "menu" {
			continue
		}
		if key, value, ok := strings.Cut(line, "="); ok {
			values[strings.ToLower(strings.TrimSpace(key))] = strings.ToLower(strings.TrimSpace(value))
		}
	}
	return values
}
