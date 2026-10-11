package evdev

import (
	"time"

	"mistervision/internal/input/control"
)

// aboutHold distinguishes a tap from a two-second hold of a device's About
// binding. Taps fire on release. A long hold fires once and consumes its release.
// Each device owns its state, so disconnecting it discards an unfinished hold.
type aboutHold struct {
	started       time.Time
	active, fired bool
}

func (h *aboutHold) update(held, pressed bool, now time.Time) control.Action {
	if !h.active && (held || pressed) {
		h.started, h.active = now, true
	}
	if !h.active {
		return ""
	}
	if !h.fired && now.Sub(h.started) >= 2*time.Second {
		h.fired = true
		if !held {
			*h = aboutHold{}
		}
		return control.ConfirmExit
	}
	if !held {
		fired := h.fired
		*h = aboutHold{}
		if !fired {
			return control.About
		}
	}
	return ""
}
