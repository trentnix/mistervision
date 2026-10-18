package evdev

import "errors"

// ErrInvalidMapping marks saved bindings that cannot safely be applied.
var ErrInvalidMapping = errors.New("invalid saved input mapping")

// DeviceInfo identifies a Linux input device for an optional mapping source.
// Physical and Unique are private lookup keys, not diagnostic fields.
type DeviceInfo struct {
	Name             string
	Vendor, Product  uint16
	Physical, Unique string
}

// MappingSource supplies inherited bindings before application profiles apply.
// An empty profile means no saved mapping. Errors fall back to built-in bindings.
// Implementations must return independent maps and must not modify device state.
type MappingSource interface {
	Bindings(DeviceInfo) (Profile, error)
}
