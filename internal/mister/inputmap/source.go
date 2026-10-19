package inputmap

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"mistervision/internal/input/evdev"
	"mistervision/internal/mister/displaymode"
)

// Source looks up saved menu mappings. The active INI's unique-mapping rules
// are captured once at startup and reused for hotplugged devices.
type Source struct {
	directory string
	unique    []uint32
}

var _ evdev.MappingSource = (*Source)(nil)

// New reads the active MiSTer INI profile to resolve controller identities.
// If selection or parsing fails, callers must retain application defaults.
func New() (*Source, error) {
	path, err := displaymode.ActiveINIPath()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	coreFile, err := os.Open("/tmp/CORENAME")
	if err != nil {
		return nil, err
	}
	core, err := io.ReadAll(io.LimitReader(coreFile, 129))
	coreFile.Close()
	if err != nil {
		return nil, err
	}
	if len(core) > 128 {
		return nil, errors.New("invalid active core name")
	}
	unique, err := readUniqueRules(f, strings.TrimSpace(string(core)))
	if err != nil {
		return nil, err
	}
	return &Source{directory: "/media/fat/config/inputs", unique: unique}, nil
}

// Bindings reads only the matching v3 menu map. Missing files return an empty
// profile. Invalid files return an error so the input reader can log a fallback.
// Core-specific and transient mode-switch maps are not guessed from filenames.
func (s *Source) Bindings(d evdev.DeviceInfo) (evdev.Profile, error) {
	name := mapName(d, s.unique)
	f, err := os.Open(filepath.Join(s.directory, name))
	if errors.Is(err, os.ErrNotExist) {
		return evdev.Profile{}, nil
	}
	if err != nil {
		return evdev.Profile{}, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, mapSize+1))
	if err != nil {
		return evdev.Profile{}, err
	}
	profile, err := parse(data)
	if err != nil {
		return evdev.Profile{}, fmt.Errorf("%w: %v", evdev.ErrInvalidMapping, err)
	}
	return profile, nil
}

// mapName follows Main_MiSTer's adapter IDs and controller_unique_mapping.
// Never glob for a similar map: two identical pads can have different bindings.
func mapName(d evdev.DeviceInfo, unique []uint32) string {
	id := fmt.Sprintf("%04x_%04x", d.Vendor, d.Product)
	adapter := d.Vendor == 0x2341 || (d.Vendor == 0x16c0 && d.Product>>8 == 4) ||
		(d.Vendor == 0x16d0 && (d.Product == 0x127e || d.Product == 0x1460)) || (d.Vendor == 0x1209 && d.Product == 0x595a)
	if adapter && d.Unique != "" {
		// MiSTer's adapter identity uses EVIOCGUNIQ with a 32-byte buffer.
		u := truncate(d.Unique, 31)
		id += "_" + strings.NewReplacer("/", "_", " ", "_", "*", "_", ":", "_").Replace(u)
	} else if d.Vendor == 0x1209 && (d.Product == 0xface || d.Product == 0xfaca) {
		sum := 0
		for _, b := range []byte(truncate(d.Name, 127)) {
			sum += int(b)
		}
		id += fmt.Sprintf("_%d", sum)
	}
	for _, rule := range unique {
		if rule == 0 {
			break
		}
		if rule != 1 && rule != uint32(d.Vendor)<<16|uint32(d.Product) {
			continue
		}
		physical, uniq := truncate(d.Physical, 63), truncate(d.Unique, 63)
		identity := physical
		if physical != "" && uniq != "" {
			identity += "/" + uniq
		} else if physical == "" {
			identity = uniq
		}
		var hash uint32
		if identity != "" {
			hash = hashString(uniq, hashString(truncate(identity, 63), 5381))
		}
		id += fmt.Sprintf("_%08x", hash)
		break
	}
	return "input_" + id + "_v3.map"
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func hashString(s string, hash uint32) uint32 {
	for _, b := range []byte(s) {
		hash = hash*33 + uint32(b)
	}
	return hash
}
