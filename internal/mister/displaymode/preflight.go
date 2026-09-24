package displaymode

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const framebufferDisabled = "MiSTerVision needs the Linux framebuffer. In %s, set fb_terminal=1 in the active [Menu] settings and reload Menu. For analog output, also use vga_scaler=1 with a timing your display supports."

// ValidateMenuFramebuffer checks the selected Menu profile before the client
// opens its display. It does not infer the connected display from INI settings.
// Callers skip this check when a supervisor will install its own core settings.
func ValidateMenuFramebuffer() error {
	path, err := ActiveINIPath()
	if err != nil {
		return fmt.Errorf("cannot validate framebuffer configuration: %w", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read framebuffer configuration: %w", err)
	}
	return validateMenuFramebuffer(data, path)
}

func validateMenuFramebuffer(data []byte, path string) error {
	value, exists := menuSettings(data)["fb_terminal"]
	if !exists {
		return nil
	} // Main defaults to an enabled framebuffer.
	// Main accepts numeric INI values. Whitespace after the value does not
	// change whether it disables the framebuffer.
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return nil
	}
	enabled, err := strconv.Atoi(fields[0])
	if err == nil && enabled == 0 {
		return fmt.Errorf(framebufferDisabled, path)
	}
	return nil
}
