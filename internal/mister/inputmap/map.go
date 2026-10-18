// Package inputmap reads MiSTer's saved menu controller mappings without
// changing them. File layout and slots follow Main_MiSTer's input.h/input.cpp.
package inputmap

import (
	"encoding/binary"
	"errors"

	"mistervision/internal/input/control"
	"mistervision/internal/input/evdev"
)

// Slot numbers and KEY_EMU match Main_MiSTer/input.h. The unused slots
// describe mouse emulation, OSD chords, and secondary analog inputs.
const (
	mapSize         = 32 * 4
	keyEmu          = 768 // KEY_MAX + 1: synthetic axis-edge key codes.
	slotRight       = 0
	slotLeft        = 1
	slotDown        = 2
	slotUp          = 3
	slotA           = 4
	slotB           = 5
	slotX           = 6
	slotL           = 8
	slotR           = 9
	slotSelect      = 10
	slotStart       = 11
	slotMenu        = 23
	slotStickX      = 24
	slotNavigationX = 28
)

// parse decodes v3 maps into a complete base profile. Unbound controls stay
// unbound. A malformed map falls back as a whole instead of mixing layouts.
func parse(data []byte) (evdev.Profile, error) {
	if len(data) != mapSize {
		return evdev.Profile{}, errors.New("controller map must contain 128 bytes")
	}
	var slots [32]uint32
	for i := range slots {
		slots[i] = binary.LittleEndian.Uint32(data[i*4:])
	}
	p := evdev.Profile{Replace: true, Buttons: make(map[uint16]control.Action), Axes: make(map[uint16]evdev.Axis), AxisLabels: make(map[uint16]evdev.AxisLabels)}
	// MiSTer's menu-specific pair overrides its core-facing A/B assignments.
	ok, back := slots[slotMenu]&0xffff, slots[slotMenu]>>16
	if ok == 0 {
		ok = slots[slotA]
	}
	if back == 0 {
		back = slots[slotB]
	}
	if ok == 0 || back == 0 || ok == back {
		return evdev.Profile{}, errors.New("controller map needs distinct confirm and back inputs")
	}
	bindings := []struct {
		raw    uint32
		action control.Action
		label  string
	}{
		{ok, control.Open, "Confirm"}, {back, control.Back, "Back"},
		{slots[slotX], control.Options, "X"},
		{slots[slotL], control.TrackPrevious, "L"}, {slots[slotR], control.TrackNext, "R"},
		{slots[slotSelect], control.Select, "Select"}, {slots[slotStart], control.About, "Start"},
		{slots[slotRight], control.Next, "Right"}, {slots[slotLeft], control.Previous, "Left"},
		{slots[slotDown], control.Down, "Down"}, {slots[slotUp], control.Up, "Up"},
	}
	for _, b := range bindings {
		if b.raw == 0 {
			continue
		}
		if b.raw < keyEmu {
			code := uint16(b.raw)
			// Follow MiSTer's first-match order when one input has two roles.
			if _, exists := p.Buttons[code]; !exists {
				p.Buttons[code] = b.action
			}
			continue
		}
		code := (b.raw - keyEmu) / 2
		if code > 0x3f {
			return evdev.Profile{}, errors.New("controller map has an invalid axis edge")
		}
		a, labels := p.Axes[uint16(code)], p.AxisLabels[uint16(code)]
		a.Press, a.Release = 40, 25
		if (b.raw-keyEmu)&1 == 1 {
			if a.Positive == "" {
				a.Positive, labels.Positive = b.action, b.label
			}
		} else {
			if a.Negative == "" {
				a.Negative, labels.Negative = b.action, b.label
			}
		}
		p.Axes[uint16(code)], p.AxisLabels[uint16(code)] = a, labels
	}
	// Use the mapped navigation stick, not assumed ABS_X/ABS_Y. MiSTer falls
	// back to Stick 1 when the separate navigation-stick slots were skipped.
	for i, actions := range [][2]control.Action{{control.Previous, control.Next}, {control.Up, control.Down}} {
		raw := slots[slotNavigationX+i]
		if raw == 0 {
			raw = slots[slotStickX+i]
		}
		if raw == 0 || raw == 0xffffffff {
			continue
		}
		if raw>>16 != 2 || raw&0xffff > 0x3f {
			return evdev.Profile{}, errors.New("controller map has an invalid navigation axis")
		}
		code := uint16(raw)
		a := p.Axes[code]
		a.Press, a.Release = 40, 25
		if a.Negative == "" {
			a.Negative = actions[0]
		}
		if a.Positive == "" {
			a.Positive = actions[1]
		}
		p.Axes[code] = a
		labels := p.AxisLabels[code]
		if labels.Negative == "" {
			labels.Negative = []string{"Left", "Up"}[i]
		}
		if labels.Positive == "" {
			labels.Positive = []string{"Right", "Down"}[i]
		}
		p.AxisLabels[code] = labels
	}
	return p, nil
}
