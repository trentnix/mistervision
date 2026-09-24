package playback

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"mistervision/internal/media"
	"os/exec"
	"sync/atomic"
	"syscall"
	"time"

	"mistervision/internal/diagnostics"
	"mistervision/internal/player"
)

var diagnosticSequence atomic.Uint64

// playbackTrace records session milestones on the playback loop. IDs are local
// counters, not server session or item identifiers. Nil disables all work.
type playbackTrace struct {
	log                     *diagnostics.Log
	id                      uint64
	start                   time.Time
	stage                   string
	bufferingKnown, waiting bool
}

func newPlaybackTrace(log *diagnostics.Log, config Config, request Request) *playbackTrace {
	if log == nil {
		return nil
	}
	t := &playbackTrace{log: log, id: diagnosticSequence.Add(1), start: time.Now()}
	name := "unknown"
	if decoder := config.decoder(request.Item); decoder != nil {
		name = decoder.Name()
	}
	t.record("playback.start", slog.String("decoder", name), slog.Bool("audio", request.Item.Type == "Audio"))
	return t
}

func (t *playbackTrace) record(event string, attrs ...slog.Attr) {
	if t == nil {
		return
	}
	attrs = append(attrs, slog.Uint64("playback", t.id), slog.Int64("elapsed_ms", time.Since(t.start).Milliseconds()))
	t.log.Record(event, attrs...)
}

// phase identifies where a failure or cancellation occurred, without retaining
// a raw error, command line, media title, or authenticated stream URL.
func (t *playbackTrace) phase(stage string) {
	if t == nil {
		return
	}
	t.stage = stage
	t.record("playback.phase", slog.String("stage", stage))
}

func (t *playbackTrace) finish(ctx context.Context, err error) {
	if t == nil {
		return
	}
	kind := diagnostics.ErrorKind(err)
	var display *player.UnsupportedDisplayError
	switch {
	case errors.As(err, &display):
		kind = "unsupported-display"
	case errors.Is(err, ErrStartupTimeout):
		kind = "startup-timeout"
	case errors.Is(err, ErrNotStarted):
		kind = "decoder-start"
	case errors.Is(err, ErrInterrupted):
		kind = "decoder-interrupted"
	case errors.Is(err, ErrProgress):
		kind = "progress-report"
	}
	t.record("playback.end", slog.String("stage", t.stage), slog.Bool("failed", err != nil), slog.Bool("canceled", ctx.Err() != nil), slog.String("error_kind", kind))
}

// prepared records the adapter's numeric limits without parsing its private URL.
// Audio streams and absent parameters report zero.
func (t *playbackTrace) prepared(s *playbackSession) {
	if t == nil {
		return
	}
	delivery := s.stream.Delivery
	t.record("playback.delivery", slog.String("provider", known(delivery.Provider, "plex", "jellyfin")),
		slog.String("requested_method", known(delivery.RequestedMethod, "transcode", "direct")),
		slog.String("server_video_decision", media.DiagnosticDecision(delivery.VideoDecision)),
		slog.String("server_audio_decision", media.DiagnosticDecision(delivery.AudioDecision)),
		slog.String("address_class", known(delivery.AddressClass, "private", "public", "loopback")), slog.Bool("tls", delivery.TLS))
	streams := s.item.MediaStreams
	if s.liveTV {
		streams = s.stream.Streams
	} else if s.stream.SourceID != "" {
		streams = nil
		for _, source := range s.item.MediaSources {
			if source.ID == s.stream.SourceID {
				streams = source.MediaStreams
				break
			}
		}
	}
	var format media.VideoFormat
	for _, stream := range streams {
		if stream.Type != "Video" {
			continue
		}
		fps := stream.RealFrameRate
		if fps <= 0 || math.IsNaN(fps) || math.IsInf(fps, 0) {
			fps = stream.AverageFrameRate
		}
		format = media.VideoFormat{Codec: stream.Codec, Width: stream.Width, Height: stream.Height, FrameRate: fps, BitRate: stream.BitRate}
		break
	}
	t.videoFormat("playback.source", format)
	t.videoFormat("playback.decoder-input", media.VideoFormat{})
	limits := s.stream.Limits
	t.record("playback.prepared", slog.Int64("start_ticks", s.start), slog.Bool("live", s.liveTV),
		slog.Float64("maxWidth", limits.MaxWidth), slog.Float64("maxHeight", limits.MaxHeight),
		slog.Float64("videoBitRate", limits.VideoBitrate), slog.Float64("maxFramerate", limits.MaxFrameRate))
}

func (t *playbackTrace) buffering(waiting bool) {
	if t == nil || t.bufferingKnown && t.waiting == waiting {
		return
	}
	t.bufferingKnown, t.waiting = true, waiting
	t.record("playback.buffering", slog.Bool("waiting", waiting))
}

// decoderExit records numeric process status without retaining stderr or an
// exec error's command text. Signal numbers follow the host operating system.
func (t *playbackTrace) decoderExit(err error) {
	if t == nil {
		return
	}
	code, signal := 0, 0
	if err != nil {
		code = -1
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		code = exit.ExitCode()
		if status, ok := exit.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			signal = int(status.Signal())
		}
	}
	t.record("playback.decoder-exit", slog.Int("exit_code", code), slog.Int("signal", signal), slog.Bool("failed", err != nil))
}

// videoFormat logs bounded observations only. Unknown values are explicit nulls,
// never requested maxima or measurements of displayed FPS.
func (t *playbackTrace) videoFormat(event string, format media.VideoFormat) {
	t.record(event, slog.String("video_codec", media.DiagnosticCodec(format.Codec)),
		slog.Any("width", observed(float64(format.Width), 16384)), slog.Any("height", observed(float64(format.Height), 16384)),
		slog.Any("frame_rate", observed(format.FrameRate, 1000)), slog.Any("bit_rate", observed(float64(format.BitRate), 10000000000)))
}

func observed(value, maximum float64) any {
	if value <= 0 || value > maximum || math.IsNaN(value) || math.IsInf(value, 0) {
		return nil
	}
	return value
}

func known(value string, allowed ...string) string {
	for _, candidate := range allowed {
		if value == candidate {
			return value
		}
	}
	return "unknown"
}
