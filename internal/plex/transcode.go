package plex

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"mistervision/internal/media"
)

// videoQuery applies one transcode policy to recorded video and tuner streams.
// Callers add their source offset and ownership identifiers before negotiation.
func (c *Client) videoQuery(path, session string, fps float64, size media.VideoSize) (url.Values, media.StreamLimits) {
	size = size.Capped(c.Config.MaxWidth, c.Config.MaxHeight)
	width, height, bitrate := size.Width, size.Height, c.Config.VideoBitrate
	if bitrate == 0 {
		bitrate = 12000000
	}
	// Plex does not supply the MPEG-2 encoder used by the Jellyfin adapter.
	profile := "add-transcode-target(type=videoProfile&context=streaming&protocol=http&container=mkv&videoCodec=h264&audioCodec=mp3&replace=true)" +
		"+add-limitation(scope=videoCodec&scopeName=h264&type=upperBound&name=video.frameRate&value=" + strconv.FormatFloat(fps, 'f', -1, 64) + "&replace=true)"
	q := url.Values{
		"path":                        {path},
		"mediaIndex":                  {"0"},
		"partIndex":                   {"0"},
		"protocol":                    {"http"},
		"directPlay":                  {"0"},
		"directStream":                {"0"},
		"directStreamAudio":           {"0"},
		"fastSeek":                    {"1"},
		"location":                    {"lan"},
		"offset":                      {"0"},
		"videoResolution":             {fmt.Sprintf("%dx%d", width, height)},
		"videoBitrate":                {strconv.Itoa(bitrate / 1000)},
		"maxVideoBitrate":             {strconv.Itoa(bitrate / 1000)},
		"audioBoost":                  {"100"},
		"session":                     {session},
		"X-Plex-Session-Identifier":   {session},
		"X-Plex-Client-Profile-Name":  {"Generic"},
		"X-Plex-Client-Profile-Extra": {profile},
		"subtitles":                   {"none"}}
	return q, media.StreamLimits{MaxWidth: float64(width), MaxHeight: float64(height), VideoBitrate: float64(bitrate), MaxFrameRate: fps}
}

// decideVideo registers the playback identity before opening the stream.
func (c *Client) decideVideo(ctx context.Context, q url.Values) (media.StreamDelivery, error) {
	delivery := media.Delivery("plex", "transcode", c.Config.Server)
	var decision struct {
		Container *struct {
			Code     int             `json:"generalDecisionCode"`
			Metadata json.RawMessage `json:"Metadata"`
		} `json:"MediaContainer"`
	}
	if err := c.json(ctx, "/video/:/transcode/universal/decision", q, &decision); err != nil {
		return delivery, err
	}
	if decision.Container == nil || decision.Container.Code < 1000 || decision.Container.Code >= 2000 {
		return delivery, media.ErrConversion
	}
	var entries []metadata
	if json.Unmarshal(decision.Container.Metadata, &entries) == nil && len(entries) > 0 && len(entries[0].Media) > 0 {
		v := entries[0].Media[0]
		if v.VideoDecision != "" {
			delivery.VideoDecision = media.DiagnosticDecision(v.VideoDecision)
		}
		if v.AudioDecision != "" {
			delivery.AudioDecision = media.DiagnosticDecision(v.AudioDecision)
		}
		for _, p := range v.Part {
			for _, stream := range p.Streams {
				if stream.Type == 1 && delivery.VideoDecision == "" {
					delivery.VideoDecision = media.DiagnosticDecision(stream.Decision)
				}
				if stream.Type == 2 && delivery.AudioDecision == "" {
					delivery.AudioDecision = media.DiagnosticDecision(stream.Decision)
				}
			}
		}
	}
	return delivery, nil
}
