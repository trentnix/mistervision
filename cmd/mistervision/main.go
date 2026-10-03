package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"mistervision/internal/mister"
	"mistervision/internal/mister/displaymode"
	"mistervision/internal/platform"
	"mistervision/internal/update"
)

func run() (err error) {
	o, err := parseOptions(os.Args[1:])
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := recoverUpdate(o); err != nil {
		return err
	}
	source, err := loadSettings(o)
	if err != nil {
		return err
	}
	if o.migrateSettings {
		if err := migrateSettings(o, source); err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, "Migrated", source.Path, "(legacy files preserved)")
		return nil
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	interlaced := os.Getenv(displaymode.ActiveEnv) == "1"
	mode, loadErr := displaymode.Parse(source.Section("display"))
	consoleMode := o.headless == "" && !interlaced && displaymode.ConsoleModeActive()
	var progressive bool
	if o.headless == "" && !interlaced && !mode.Interlaced && !consoleMode && loadErr == nil {
		loadErr = displaymode.ValidateMenuFramebuffer()
		if loadErr == nil {
			progressive = displaymode.NeedsProgressiveSupervisor()
		}
	}
	supervised := o.headless == "" && !interlaced && (mode.Interlaced || progressive || consoleMode) && loadErr == nil
	trace, err := openStartupDiagnostics(o, supervised, source)
	if err != nil {
		return err
	}
	defer func() { trace.close(err) }()
	trace.phase("display-config")
	if loadErr != nil {
		return loadErr
	}
	if o.headless == "" {
		mister.RecordStartup(trace.log, interlaced)
	}
	if consoleMode {
		trace.phase("consolemode-supervisor")
		return displaymode.RunConsoleMode(ctx, filepath.Dir(source.Path), mode, os.Args[1:], trace.log)
	}
	if o.headless == "" && !interlaced && mode.Interlaced {
		trace.phase("interlaced-supervisor")
		return displaymode.Run(ctx, filepath.Dir(source.Path), os.Args[1:])
	}
	if progressive {
		trace.phase("framebuffer-supervisor")
		return displaymode.RunProgressive(ctx, mode, os.Args[1:])
	}
	trace.phase("display-open")
	d, err := platform.Open(platform.Options{Device: o.device, Headless: o.headless, Output: o.output, AspectRatio: mode.AspectRatio})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, d.Close()) }()
	g := d.Geometry()
	o.displayAspect = platform.ResolveAspect(mode.AspectRatio, g.OutputWidth, g.OutputHeight)
	rw, rh := platform.RasterSize(d)
	trace.log.Record("application.display", slog.Int("browsing_width", rw), slog.Int("browsing_height", rh), slog.Int("ui_width", g.Width), slog.Int("ui_height", g.Height), slog.Int("output_width", g.OutputWidth), slog.Int("output_height", g.OutputHeight), slog.Float64("display_aspect", o.displayAspect))
	if o.browse {
		return runBrowser(ctx, d, o, trace, source)
	}
	return runPreview(ctx, d, o)
}

func main() {
	if err := run(); err != nil {
		if update.RestartRequested(err) {
			os.Exit(update.RestartExitCode)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
