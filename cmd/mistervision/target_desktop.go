package main

import (
	"context"

	"mistervision/internal/diagnostics"
	"mistervision/internal/input"
	"mistervision/internal/input/control"
	"mistervision/internal/platform"
	"mistervision/internal/playback"
	"mistervision/internal/player/ffplay"
	"mistervision/internal/player/mplayer"
	"mistervision/internal/player/pythonhelper"
	"mistervision/internal/sound/alsa"
	"mistervision/internal/videoout"
	"mistervision/internal/videoout/companion"
	"mistervision/internal/videoout/framefile"
)

// desktopTarget combines terminal input with either frame-file video for
// Ghostty or a companion player window. Both use the shared browser renderer.
func desktopTarget(d platform.Presenter, o launchOptions) browserTarget {
	config := desktopPlayback(o, d.Geometry())
	var output videoout.Output
	if o.terminalPlayer != "" {
		frames := framefile.New(d, inlineFramePath(o))
		frames.DisplayAspect = o.displayAspect
		output = frames
	} else {
		output = companion.New(d)
	}
	return browserTarget{initialControls: control.KeyboardLabels(), openSound: alsa.Open, player: config, output: output, readInput: func(ctx context.Context, _ *diagnostics.Log) (<-chan control.Event, <-chan struct{}, error) {
		return input.ReadTerminal(ctx)
	}}
}

// desktopPlayback defaults to FFplay and selects the Python helper when inline
// video is requested. An audio helper overrides audio only when no explicit
// player executable was supplied.
func desktopPlayback(o launchOptions, g platform.Geometry) playback.Config {
	decoder := ffplay.Decoder{Player: o.player, DisplayAspect: o.displayAspect}
	config := playback.Config{VideoDecoder: decoder, AudioDecoder: decoder, Timing: crtPlaybackTiming(g.OutputHeight)}
	if o.terminalPlayer != "" {
		// Frame files contain logical pixels. The presenter expands them to the
		// physical framebuffer, including line doubling and pillarboxing.
		helper := pythonhelper.Decoder{Script: o.terminalPlayer, Output: inlineFramePath(o), Width: g.Width, Height: g.Height, DisplayAspect: o.displayAspect}
		if o.misterDisplayCheck {
			helper.DisplayValidator = mplayer.Decoder{Width: g.OutputWidth, Height: g.OutputHeight}
		}
		config.VideoDecoder = helper
		config.AudioDecoder = config.VideoDecoder
	}
	if o.audioPlayer != "" && o.player == "" {
		config.AudioDecoder = pythonhelper.Decoder{Script: o.audioPlayer}
	}
	return config
}

// inlineFramePath joins the Python decoder and frame-file output at one destination.
func inlineFramePath(o launchOptions) string { return o.output + ".video" }
