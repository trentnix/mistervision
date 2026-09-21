package player

import "fmt"

// UnsupportedDisplayError reports physical framebuffer dimensions that a decoder
// cannot use for video. It contains no server or media information.
type UnsupportedDisplayError struct {
	Width, Height int
}

// Error describes the rejected framebuffer for diagnostics and callers.
func (e *UnsupportedDisplayError) Error() string {
	return fmt.Sprintf("video playback does not support the %dx%d framebuffer", e.Width, e.Height)
}
