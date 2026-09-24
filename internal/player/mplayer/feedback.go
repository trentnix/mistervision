package mplayer

import (
	"io"
	"math"
	"mistervision/internal/media"
	"strconv"
	"strings"

	"mistervision/internal/player"
	"mistervision/internal/player/feedback"
)

// Feedback creates an independent stdout/stderr parser for one process.
// emit receives validated observations and must return without blocking.
func (d Decoder) Feedback(emit func(player.Feedback)) io.Writer {
	var format media.VideoFormat
	return feedback.NewWriter(func(line string) (player.Feedback, bool) {
		if value, ok := feedback.ParseANS(line); ok {
			return value, true
		}
		if parseVideoFormat(line, &format) {
			return player.Feedback{Kind: player.FeedbackVideoFormat, VideoFormat: format}, true
		}
		return player.Feedback{}, false
	}, emit)
}

// parseVideoFormat accepts only MPlayer's numeric identification fields and
// known codec names. Every emitted snapshot is cumulative for this process.
func parseVideoFormat(line string, format *media.VideoFormat) bool {
	key, value, ok := strings.Cut(line, "=")
	if !ok {
		return false
	}
	if key == "ID_VIDEO_CODEC" {
		codec := media.DiagnosticCodec(value)
		if codec == "unknown" || codec == format.Codec {
			return false
		}
		format.Codec = codec
		return true
	}
	number, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) || number <= 0 {
		return false
	}
	previous := *format
	switch key {
	case "ID_VIDEO_WIDTH", "ID_VIDEO_HEIGHT":
		if number > 16384 || number != math.Trunc(number) {
			return false
		}
		if key == "ID_VIDEO_WIDTH" {
			format.Width = int(number)
		} else {
			format.Height = int(number)
		}
	case "ID_VIDEO_FPS":
		if number > 1000 {
			return false
		}
		format.FrameRate = number
	case "ID_VIDEO_BITRATE":
		if number > 10000000000 || number != math.Trunc(number) {
			return false
		}
		format.BitRate = int64(number)
	default:
		return false
	}
	return previous != *format
}
