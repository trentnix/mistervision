package jellyfin

import (
	"errors"
	"regexp"
	"strconv"
	"strings"

	"mistervision/internal/media"
)

// TranscodeProfile limits server-side video conversion. Dimensions are maximums,
// not an output aspect ratio. Zero dimensions follow the playback display.
// Explicit caps use widths of 160–1920 and heights of 120–1080. Zero bitrate
// uses 12 Mbps. Explicit bitrates use 100,000–50,000,000 bits per second.
// The display pipeline, not this profile, selects the frame-rate cap.
type TranscodeProfile struct {
	MaxWidth, MaxHeight int
	VideoBitrate        int
}

// DefaultTranscodeProfile selects automatic dimensions and a 12 Mbps budget.
// Live TV uses VideoBitrate as its negotiated streaming bitrate budget.
func DefaultTranscodeProfile() TranscodeProfile {
	return TranscodeProfile{VideoBitrate: 12000000}
}

// transcodeProfile combines the current playback size with explicit user caps.
// The client configuration remains immutable across simultaneous requests.
func (c Config) transcodeProfile(size media.VideoSize) TranscodeProfile {
	p := c.Transcode
	size = size.Capped(p.MaxWidth, p.MaxHeight)
	p.MaxWidth, p.MaxHeight = size.Width, size.Height
	if p.VideoBitrate == 0 {
		p.VideoBitrate = DefaultTranscodeProfile().VideoBitrate
	}
	return p
}

// Reserve profile-shaped lines even when malformed so an invalid profile cannot
// silently become an API key. This prefix is reserved for profile settings.
var profileLine = regexp.MustCompile(`^[+-]?\d+\s*[xX]`)

// parseTranscodeProfile follows the C WIDTHxHEIGHT[@BITRATE] format. Omitting
// bitrate retains the preceding value. Invalid input returns no partial update.
// Errors never include the line, which belongs to a credential-bearing file.
func parseTranscodeProfile(line string, current TranscodeProfile) (TranscodeProfile, error) {
	dimensions, bitrate, hasBitrate := strings.Cut(line, "@")
	width, height, ok := strings.Cut(dimensions, "x")
	decimal := func(s string) (int, bool) {
		if s == "" {
			return 0, false
		}
		for _, c := range s {
			if c < '0' || c > '9' {
				return 0, false
			}
		}
		n, err := strconv.Atoi(s)
		return n, err == nil
	}
	w, validWidth := decimal(width)
	h, validHeight := decimal(height)
	rate, validRate := current.VideoBitrate, true
	if hasBitrate {
		rate, validRate = decimal(bitrate)
	}
	if !ok || !validWidth || !validHeight || !validRate {
		return current, errors.New("use WIDTHxHEIGHT or WIDTHxHEIGHT@BITRATE")
	}
	if w < 160 || w > 1920 || h < 120 || h > 1080 {
		return current, errors.New("width must be 160-1920 and height must be 120-1080")
	}
	if rate < 100000 || rate > 50000000 {
		return current, errors.New("bitrate must be 100000-50000000 bits per second")
	}
	return TranscodeProfile{MaxWidth: w, MaxHeight: h, VideoBitrate: rate}, nil
}
