package displaymode

import (
	"fmt"
	"os"
	"strings"
)

// NativeAnalogEnv selects Menu's native analog frame counter in the player.
// Only display supervisors set it, after resolving the active INI and raster.
const NativeAnalogEnv = "MISTERVISION_NATIVE_ANALOG_SYNC"

// nativeAnalogEnvironment leaves scaler synchronization unchanged unless both
// the configured route and actual framebuffer identify native analog scanout.
func nativeAnalogEnvironment(name string) (string, error) {
	path, err := ActiveINIPath()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	w, h, err := nativeFramebufferControl().read()
	if err != nil {
		return "", err
	}
	value := "0"
	if nativeAnalogRoute(data, name, w, h) {
		value = "1"
	}
	return NativeAnalogEnv + "=" + value, nil
}

func nativeAnalogRoute(data []byte, name string, width, height int) bool {
	if width != 640 || (height != 240 && height != 288) {
		return false
	}
	values := menuSettings(data, name)
	for key, expected := range map[string]int{"fb_terminal": 2, "vga_scaler": 0, "direct_video": 0, "forced_scandoubler": 0} {
		value, exists := values[key]
		if !exists {
			if expected != 0 { // Main defaults the other route flags to zero.
				return false
			}
			continue
		}
		var number int
		if _, err := fmt.Sscanf(value, "%d", &number); err != nil || number != expected {
			return false
		}
	}
	// Timing-specific overrides need Main's exact refresh-rate match. Do not
	// infer a route from the base section if a video section can change it.
	video := false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.ToLower(strings.TrimSpace(strings.SplitN(line, ";", 2)[0]))
		if strings.HasPrefix(line, "[") {
			video = strings.HasPrefix(line, "[video=")
		} else if strings.HasPrefix(line, "+video=") {
			video = true
		}
		if video {
			key, _, ok := strings.Cut(line, "=")
			if ok {
				switch strings.TrimSpace(key) {
				case "fb_terminal", "vga_scaler", "direct_video", "forced_scandoubler":
					return false
				}
			}
		}
	}
	return true
}

// NativeAnalogEnabled records only the supervisor's selection, not arbitrary
// environment values or connection details.
func NativeAnalogEnabled() bool {
	return os.Getenv(NativeAnalogEnv) == "1"
}
