package displaymode

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"mistervision/internal/diagnostics"
	"mistervision/internal/update"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const consoleModeEnv = "MISTERVISION_CONSOLEMODE"
const consoleModeHost = "MiSTer_ConsoleM" // Linux truncates comm to 15 bytes.
const consoleModeCore = "ConsoleMode/menu_ConsoleMode.rbf"
const framebufferCore = "MiSTerVisionFramebuffer"

// ConsoleModeActive reports a live ConsoleMode host, not merely an installation.
// The supervised client must not take ownership of the same session again.
func ConsoleModeActive() bool {
	return os.Getenv(consoleModeEnv) != "1" && processRunning("/proc", consoleModeHost)
}

// processRunning ignores zombies so handoff waits acknowledge a live owner.
func processRunning(root, name string) bool {
	entries, _ := os.ReadDir(root)
	for _, entry := range entries {
		if _, err := strconv.Atoi(entry.Name()); err != nil {
			continue
		}
		comm, err := os.ReadFile(filepath.Join(root, entry.Name(), "comm"))
		if err != nil || strings.TrimSpace(string(comm)) != name {
			continue
		}
		status, err := os.ReadFile(filepath.Join(root, entry.Name(), "status"))
		if err == nil && !strings.Contains(string(status), "State:\tZ") && !strings.Contains(string(status), "State:\tX") {
			return true
		}
	}
	return false
}

func waitProcess(ctx context.Context, name string, running bool) error {
	for processRunning("/proc", name) != running {
		if err := delay(ctx); err != nil {
			return fmt.Errorf("waiting for %s: %w", name, err)
		}
	}
	return nil
}

// RunConsoleMode transfers the display to the original MiSTer host before the
// client maps pixels. It restores ConsoleMode after playback, failure, or update.
// The INI handoff section inherits the user's Menu timing and connector settings.
func RunConsoleMode(ctx context.Context, directory string, c Config, args []string, log *diagnostics.Log) (err error) {
	stage := "prepare"
	started := time.Now()
	phase := func(next string) {
		stage = next
		log.Record("mister.handoff", slog.String("stage", stage), slog.Int64("elapsed_ms", time.Since(started).Milliseconds()))
		RecordState(log, stage)
	}
	phase(stage)
	defer func() {
		result := err
		if update.RestartRequested(result) {
			result = nil
		}
		log.Record("mister.handoff.result", slog.String("stage", stage), slog.Bool("failed", result != nil), slog.String("error_kind", diagnostics.ErrorKind(result)), slog.Int64("elapsed_ms", time.Since(started).Milliseconds()))
	}()
	lock, err := os.OpenFile("/tmp/mistervision-display.lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return errors.New("another client owns the display")
	}
	mgl, err := prepareConsoleModeCore(directory, c.Interlaced)
	if err != nil {
		return err
	}
	owner := strconv.Itoa(os.Getpid())
	var mainPID int
	switched := false
	defer func() {
		if err != nil && !update.RestartRequested(err) {
			log.Record("mister.handoff.failure", slog.String("stage", stage), slog.String("error_kind", diagnostics.ErrorKind(err)))
		}
		phase("cleanup")
		err = errors.Join(err, stopOrphans(owner))
		if mainPID > 0 {
			err = errors.Join(err, syscall.Kill(mainPID, syscall.SIGCONT))
		}
		// Unlock before restoring terminals: a failed enable may have left Main
		// waiting for VT1. Do not leave that wait blocking the return command.
		if switched {
			err = errors.Join(err, unlockConsole())
		}
		err = errors.Join(err, restoreConsole())
		if switched {
			phase("return-consolemode")
			restore, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			returnErr := returnConsoleMode(restore, command, waitProcess, func() error {
				return os.WriteFile("/tmp/consolemode_launch_request", nil, 0600)
			})
			err = errors.Join(err, returnErr)
			if returnErr == nil {
				phase("returned")
			}
		}
	}()
	phase("prepare-console")
	if err = prepareConsole(); err != nil {
		return err
	}
	if err = os.Remove("/tmp/OSD_VISIBLE"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	phase("load-core")
	switched = true // A timed-out command may still have reached Main.
	if err = command(ctx, "load_core "+mgl); err != nil {
		return err
	}
	ready, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	name := framebufferCore
	if c.Interlaced {
		name = coreName
	}
	phase("wait-host")
	if err = waitCore(ready, name, c.Interlaced); err != nil {
		return err
	}
	if err = waitProcess(ready, "MiSTer", true); err != nil {
		return err
	}
	if err = waitProcess(ready, "ConsoleMode_arm", false); err != nil {
		return err
	}
	if err = waitMenuReady(ready, "/tmp/OSD_VISIBLE"); err != nil {
		return err
	}
	phase("activate-framebuffer")
	if err = unlockConsole(); err != nil {
		return err
	}
	if err = enableConsole(); err != nil {
		return err
	}
	phase("configure-output")
	environment := []string{consoleModeEnv + "=1", ownerEnv + "=" + owner}
	if c.Interlaced {
		mainPID, err = stopMain(ready)
		if err != nil {
			return err
		}
		environment = append(environment, ActiveEnv+"=1")
	} else {
		f := nativeFramebufferControl()
		w, h, readErr := f.read()
		if readErr != nil {
			return readErr
		}
		if !crtFramebuffer(w, h) {
			if err = f.configure(ready, c); err != nil {
				return err
			}
		}
		environment = append(environment, scaledEnv+"=1")
	}
	phase("client")
	return runChild(ctx, args, environment...)
}

// returnConsoleMode reloads through the restored host because standard Main
// expands relative paths, while ConsoleMode recognizes its initial core by the
// relative path. Waiting for the frontend confirms the complete return handoff.
func returnConsoleMode(ctx context.Context, send func(context.Context, string) error, wait func(context.Context, string, bool) error, request func() error) error {
	if err := send(ctx, "load_core "+consoleModeCore); err != nil {
		return err
	}
	if err := wait(ctx, consoleModeHost, true); err != nil {
		return err
	}
	if err := request(); err != nil {
		return err
	}
	if err := send(ctx, "load_core "+consoleModeCore); err != nil {
		return err
	}
	return wait(ctx, "ConsoleMode_arm", true)
}
