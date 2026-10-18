//go:build linux

package evdev

import (
	"syscall"
	"unsafe"

	"mistervision/internal/diagnostics"
	"mistervision/internal/input/control"
)

// mappedAxis translates a configured axis into held actions. Each device owns
// its state, so disconnecting one controller cannot leave another held down.
type mappedAxis struct {
	binding        Axis
	min, rest, max int64
	held           control.Action
	waitForNeutral bool
}

func newMappedAxis(binding Axis, min, max int32) *mappedAxis {
	a := &mappedAxis{binding: binding, min: int64(min), max: int64(max)}
	a.rest = a.min + (a.max-a.min)/2
	switch binding.Rest {
	case "minimum":
		a.rest = a.min
	case "maximum":
		a.rest = a.max
	}
	return a
}

func (a *mappedAxis) action(value int32) control.Action {
	key, travel, span := a.binding.Positive, int64(value)-a.rest, a.max-a.rest
	if int64(value) < a.rest {
		key, travel, span = a.binding.Negative, a.rest-int64(value), a.rest-a.min
	}
	press, release := a.binding.thresholds()
	threshold := press
	if a.held == key {
		threshold = release
	}
	a.held = ""
	if key != "" && span > 0 && travel*100 >= span*int64(threshold) {
		a.held = key
	}
	if a.waitForNeutral {
		a.waitForNeutral = a.held != ""
		return ""
	}
	return a.held
}

type absoluteInfo struct{ Value, Minimum, Maximum, Fuzz, Flat, Resolution int32 }

func readAxis(fd int, code uint16) (info absoluteInfo, ok bool) {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), uintptr(0x80184540)+uintptr(code), uintptr(unsafe.Pointer(&info))) // EVIOCGABS
	return info, errno == 0 && info.Maximum > info.Minimum
}

// deviceCapabilities snapshots advertised inputs and usable axis ranges once
// at discovery. The same snapshot validates saved maps and initializes axes.
type deviceCapabilities struct {
	keys [96]byte
	axes map[uint16]absoluteInfo
}

func readCapabilities(fd int) deviceCapabilities {
	caps := deviceCapabilities{keys: keyCapabilities(fd), axes: make(map[uint16]absoluteInfo)}
	var bits [8]byte
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), 0x80084523, uintptr(unsafe.Pointer(&bits[0]))) // EVIOCGBIT(EV_ABS)
	if errno != 0 {
		return caps
	}
	for code := uint16(0); code < 64; code++ {
		if bits[code/8]&(1<<(code%8)) == 0 {
			continue
		}
		if info, ok := readAxis(fd, code); ok {
			caps.axes[code] = info
		}
	}
	return caps
}

// supports checks whether a saved action can actually be produced. Missing
// optional buttons do not invalidate a map, but confirm and back must work.
func (c deviceCapabilities) supports(p Profile, action control.Action) bool {
	for code, key := range p.Buttons {
		if key == action && code < 768 && c.keys[code/8]&(1<<(code%8)) != 0 {
			return true
		}
	}
	for code, binding := range p.Axes {
		info, ok := c.axes[code]
		if !ok || info.Maximum <= info.Minimum {
			continue
		}
		axis := newMappedAxis(binding, info.Minimum, info.Maximum)
		if axis.action(info.Minimum) == action || axis.action(info.Maximum) == action {
			return true
		}
	}
	return false
}

// axisBindings adds gamepad left-stick defaults before applying explicit
// bindings. Keep defaults out of Profile so labels can prefer configured inputs.
func (d *device) axisBindings(keys [96]byte) map[uint16]Axis {
	bindings := make(map[uint16]Axis)
	// BTN_GAMEPAD (BTN_SOUTH) identifies a gamepad rather than a touchscreen
	// or pointer with ABS_X/ABS_Y. MiSTer's virtual node supplies arrows only.
	if !d.bindings.Replace && d.name != "MiSTer virtual input" && keys[304/8]&(1<<(304%8)) != 0 {
		bindings[0] = Axis{Negative: control.Previous, Positive: control.Next, Press: 40, Release: 25}
		bindings[1] = Axis{Negative: control.Up, Positive: control.Down, Press: 40, Release: 25}
	}
	for code, binding := range d.bindings.Axes {
		bindings[code] = binding
	}
	return bindings
}

func (d *device) configure(config Config, source MappingSource, log *diagnostics.Log) {
	caps := readCapabilities(d.fd)
	d.overrides = config.bindings(d.name)
	var base Profile
	if !d.overrides.Replace {
		base = d.inheritedBindings(source, caps, log)
	}
	d.bindings = config.bindingsOn(d.name, base)
	_, d.hats[0] = caps.axes[16]
	_, d.hats[1] = caps.axes[17]
	d.axes = make(map[uint16]*mappedAxis)
	for code, binding := range d.axisBindings(caps.keys) {
		if info, ok := caps.axes[code]; ok {
			a := newMappedAxis(binding, info.Minimum, info.Maximum)
			// Wait for release if an inherited axis is already held at discovery.
			if base.Replace {
				a.waitForNeutral = a.action(info.Value) != ""
			}
			d.axes[code] = a
		}
	}
	d.legend = d.labels(caps.keys)
}

func (d *device) mappedAction(e event) control.Action {
	if e.Type == 1 {
		if e.Value != 1 {
			return ""
		}
		if key, ok := d.bindings.Buttons[e.Code]; ok {
			return key
		}
	} else if axis := d.axes[e.Code]; axis != nil {
		return axis.action(e.Value)
	} else if _, configured := d.bindings.Axes[e.Code]; configured {
		return ""
	}
	if d.bindings.Replace {
		return ""
	}
	return action(d.name, e.Type, e.Code, e.Value)
}
