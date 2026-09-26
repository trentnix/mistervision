package playback

import (
	"math"

	"mistervision/internal/media"
)

// TranscodeSize converts a playback raster into a square-pixel server limit.
// DisplayAspect is the physical screen's width/height ratio. It accounts for
// non-square CRT pixels without stretching the source. Zero uses square pixels.
// Picture mode is intentionally absent: zoom reuses the same decoded stream.
func TranscodeSize(width, height int, displayAspect float64) media.VideoSize {
	if width <= 0 || height <= 0 {
		return media.VideoSize{}
	}
	if displayAspect == 0 {
		displayAspect = float64(width) / float64(height)
	}
	if math.IsNaN(displayAspect) || math.IsInf(displayAspect, 0) || displayAspect <= 0 {
		return media.VideoSize{}
	}
	// Round floating-point noise before rounding down to even encoder dimensions.
	w := int(math.Round(float64(height) * displayAspect))
	if w < 2 || w > 8192 || height > 8192 {
		return media.VideoSize{}
	}
	return media.VideoSize{Width: w - w%2, Height: height - height%2}
}
