package displaymode

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const scaledEnv = "MISTERVISION_SCALED_FRAMEBUFFER"
const framebufferMode = "/sys/module/MiSTer_fb/parameters/mode"
const framebufferRevision = "/sys/module/MiSTer_fb/parameters/res_count"

// crtFramebuffer identifies the non-square-pixel rasters whose timing and
// geometry must remain untouched by the HDMI framebuffer policy.
func crtFramebuffer(width, height int) bool {
	return width == 640 && (height == 240 || height == 288 || height == 480 || height == 576)
}

// FramebufferDivisor selects the highest resolution within the configured
// limits using Main's full-screen FPGA scaling command. The signal resolution
// is unchanged. An error means none of Main's four divisors fits safely.
func (c Config) FramebufferDivisor(width, height int) (int, error) {
	mw, mh := c.FramebufferMaxWidth, c.FramebufferMaxHeight
	if mw == 0 {
		mw = 640
	}
	if mh == 0 {
		mh = 480
	}
	for div := 1; div <= 4; div++ {
		w, h := width/div, height/div
		if w >= 120 && h >= 120 && w <= mw && h <= mh && w%2 == 0 && h%2 == 0 {
			return div, nil
		}
	}
	return 0, fmt.Errorf("no supported framebuffer divisor fits %dx%d within %dx%d", width, height, mw, mh)
}

// framebufferControl isolates Main's command and acknowledgement protocol from
// geometry policy. Tests use temporary kernel parameter files and a fake Main.
type framebufferControl struct {
	modePath, revisionPath string
	send                   func(context.Context, string) error
}

func nativeFramebufferControl() framebufferControl {
	return framebufferControl{framebufferMode, framebufferRevision, command}
}

func (f framebufferControl) read() (int, int, error) {
	data, err := os.ReadFile(f.modePath)
	if err != nil {
		return 0, 0, err
	}
	var format, swap, width, height, stride int
	if n, err := fmt.Sscanf(string(data), "%d %d %d %d %d", &format, &swap, &width, &height, &stride); err != nil || n != 5 || format != 8888 || swap != 1 || width < 120 || height < 120 || width > 8192 || height > 8192 || stride < width*4 {
		return 0, 0, errors.New("unsupported MiSTer framebuffer format")
	}
	return width, height, nil
}

// NeedsFramebufferScaling checks the current raster before any mapping. The
// supervised child skips this check. CRT modes never enter the scaler session.
func NeedsFramebufferScaling() (bool, error) {
	if os.Getenv(scaledEnv) == "1" {
		return false, nil
	}
	w, h, err := nativeFramebufferControl().read()
	return err == nil && !crtFramebuffer(w, h), err
}

func (f framebufferControl) setDivisor(ctx context.Context, div int) (int, int, error) {
	revision, err := os.ReadFile(f.revisionPath)
	if err != nil {
		return 0, 0, err
	}
	if err = f.send(ctx, fmt.Sprintf("fb_cmd0 8888 1 %d", div)); err != nil {
		return 0, 0, err
	}
	if err = f.waitChanged(ctx, revision); err != nil {
		return 0, 0, err
	}
	return f.read()
}

// waitChanged requires a kernel acknowledgement even when dimensions match.
func (f framebufferControl) waitChanged(ctx context.Context, revision []byte) error {
	for {
		current, e := os.ReadFile(f.revisionPath)
		if e != nil {
			return e
		}
		if strings.TrimSpace(string(current)) != strings.TrimSpace(string(revision)) {
			return nil
		}
		if err := delay(ctx); err != nil {
			return fmt.Errorf("waiting for MiSTer framebuffer acknowledgement: %w", err)
		}
	}
}

// RunScaled supervises a non-CRT framebuffer session. Main sets both kernel and
// FPGA dimensions without changing video timings or persistent configuration.
// All children stop before restoring Menu, including after a client failure.
// The diagnostic progressive test also pauses Main for exclusive SPI access.
func RunScaled(ctx context.Context, c Config, args []string) (err error) {
	lock, err := os.OpenFile("/tmp/mistervision-display.lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return errors.New("another client owns the display")
	}
	active, err := os.ReadFile("/tmp/CORENAME")
	if err != nil || strings.TrimSpace(string(active)) != "MENU" {
		return errors.New("start scaled playback from the normal MiSTer menu")
	}
	owner := strconv.Itoa(os.Getpid())
	var mainPID int
	defer func() {
		err = errors.Join(err, stopOrphans(owner))
		if mainPID > 0 {
			err = errors.Join(err, syscall.Kill(mainPID, syscall.SIGCONT))
		}
		if err == nil && os.Getenv(LauncherEnv) == "1" {
			return
		}
		restore, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		f := nativeFramebufferControl()
		before, readErr := os.ReadFile(f.revisionPath)
		sendErr := f.send(restore, "load_core /media/fat/menu.rbf")
		err = errors.Join(err, readErr, sendErr)
		if readErr == nil && sendErr == nil {
			err = errors.Join(err, f.waitChanged(restore, before))
		}

		err = errors.Join(err, waitCore(restore, "MENU", false))
	}()
	setup, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	test := ProgressiveTestRequested()
	f := nativeFramebufferControl()
	w, h, err := f.read()
	if err != nil {
		return err
	}
	if !test || !crtFramebuffer(w, h) {
		if test {
			c.FramebufferMaxWidth, c.FramebufferMaxHeight = 640, 480
		}
		if err = f.configure(setup, c); err != nil {
			return err
		}
	}
	environment := []string{scaledEnv + "=1", ownerEnv + "=" + owner}
	if test {
		mainPID, err = stopMain(setup)
		if err != nil {
			return err
		}
		environment = append(environment, "MISTERVISION_PROGRESSIVE_PAGEFLIP=1")
	}
	return runChild(ctx, args, environment...)
}

// configure waits for both Main and the kernel before a child can map pixels.
func (f framebufferControl) configure(ctx context.Context, c Config) error {
	// Probe at the smallest canvas first. Requesting full signal resolution
	// could exceed framebuffer memory on a high-resolution HDMI display.
	w, h, err := f.setDivisor(ctx, 4)
	if err != nil {
		return err
	}
	div, err := c.FramebufferDivisor(w*4, h*4)
	if err != nil {
		return err
	}
	if div != 4 {
		w, h, err = f.setDivisor(ctx, div)
		if err != nil {
			return err
		}
	}
	// The quarter-size probe rounds down. Validate the actual final canvas,
	// rather than trusting reconstructed signal dimensions or an ignored command.
	check, err := c.FramebufferDivisor(w, h)
	if err != nil || check != 1 {
		return errors.New("MiSTer did not apply framebuffer dimensions within the configured limits")
	}
	return nil
}
