//go:build linux

package evdev

import (
	"errors"
	"log/slog"
	"strings"
	"syscall"
	"unsafe"

	"mistervision/internal/diagnostics"
	"mistervision/internal/input/control"
)

var errUnavailableMapping = errors.New("saved map requires unavailable confirm or back inputs")

func (d *device) inheritedBindings(source MappingSource, caps deviceCapabilities, log *diagnostics.Log) Profile {
	if source == nil || d.name == "MiSTer virtual input" {
		return Profile{}
	}
	var id struct{ Bus, Vendor, Product, Version uint16 }
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(d.fd), 0x80084502, uintptr(unsafe.Pointer(&id))) // EVIOCGID
	if errno != 0 {
		log.Record("input.mapping", slog.String("status", "fallback"), slog.String("error_kind", diagnostics.ErrorKind(errno)))
		return Profile{}
	}
	info := DeviceInfo{Name: d.name, Vendor: id.Vendor, Product: id.Product,
		Physical: deviceString(d.fd, 0x80404507), Unique: deviceString(d.fd, 0x80404508)} // EVIOCGPHYS / EVIOCGUNIQ
	return sourceBindings(source, info, caps, log)
}

// sourceBindings isolates saved-map failure handling from Linux identity reads.
func sourceBindings(source MappingSource, info DeviceInfo, caps deviceCapabilities, log *diagnostics.Log) Profile {
	profile, err := source.Bindings(info)
	if err == nil && profile.Replace && (!caps.supports(profile, control.Open) || !caps.supports(profile, control.Back)) {
		err = errUnavailableMapping
	}
	status := "default"
	if err != nil {
		profile = Profile{}
		status = "fallback"
	} else if profile.Replace {
		status = "saved"
	}
	kind := diagnostics.ErrorKind(err)
	if errors.Is(err, ErrInvalidMapping) {
		kind = "invalid-mapping"
	} else if errors.Is(err, errUnavailableMapping) {
		kind = "unavailable-controls"
	}
	log.Record("input.mapping", slog.String("status", status), slog.Int("vendor", int(info.Vendor)), slog.Int("product", int(info.Product)), slog.String("error_kind", kind))
	return profile
}

func deviceString(fd int, request uintptr) string {
	var value [64]byte
	syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), request, uintptr(unsafe.Pointer(&value[0])))
	return strings.SplitN(string(value[:]), "\x00", 2)[0]
}
