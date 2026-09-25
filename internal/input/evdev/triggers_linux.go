//go:build linux

package evdev

import (
	"syscall"
	"unsafe"

	"mistervision/internal/input/control"
)

// defaultTriggerBinding preserves the minimum-rest trigger convention. Default
// binding policy lives here, independently of travel and hysteresis in mappedAxis.
// A numeric range alone cannot identify a trigger or its resting position.
func defaultTriggerBinding(code uint16) Axis {
	return Axis{Rest: "minimum", Positive: triggerKey(code)}
}

func triggerKey(code uint16) control.Action {
	switch code {
	case 2, 10: // ABS_Z, ABS_BRAKE
		return control.SeekBackward
	case 5, 9: // ABS_RZ, ABS_GAS
		return control.SeekForward
	}
	return ""
}

// discoverTriggers finds candidate trigger axes on devices with shoulder buttons.
// Axis codes are driver conventions, not proof of physical control identity.
// Explicit axis bindings take precedence when processing input.
func discoverTriggers(fd int) map[uint16]*mappedAxis {
	var keys [96]byte
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), 0x80604521, uintptr(unsafe.Pointer(&keys[0]))) // EVIOCGBIT(EV_KEY)
	if errno != 0 || keys[310/8]&(1<<(310%8)) == 0 || keys[311/8]&(1<<(311%8)) == 0 {
		return nil
	}
	axes := make(map[uint16]*mappedAxis)
	for _, code := range []uint16{2, 5, 9, 10} {
		if min, max, ok := axisRange(fd, code); ok {
			axes[code] = newMappedAxis(defaultTriggerBinding(code), min, max)
		}
	}
	return axes
}
