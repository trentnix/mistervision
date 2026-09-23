//go:build linux

package displaymode

import (
	"errors"
	"os"
	"runtime"
	"syscall"
)

// activeINIIndex reads only the four-byte selection record used by altcfg in
// https://github.com/MiSTer-devel/Main_MiSTer/blob/master/user_io.cpp.
// The mapping is read-only and never changes the profile.
func activeINIIndex() (int, error) {
	if runtime.GOARCH != "arm" {
		return 0, errors.New("MiSTer INI selection requires ARM hardware")
	}
	f, err := os.OpenFile("/dev/mem", os.O_RDONLY|syscall.O_SYNC, 0)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	data, err := syscall.Mmap(int(f.Fd()), 0x1ffff000, 4096, syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		return 0, err
	}
	index, readErr := iniProfileIndex(data[0xf04:0xf08])
	return index, errors.Join(readErr, syscall.Munmap(data))
}
