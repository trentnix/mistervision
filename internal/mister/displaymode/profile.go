package displaymode

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ActiveINIPath resolves the host's selected INI profile without changing it.
// Failure is explicit: editing Main when another profile is active is unsafe.
func ActiveINIPath() (string, error) {
	index, err := activeINIIndex()
	if err != nil {
		return "", fmt.Errorf("cannot determine active MiSTer INI profile: %w", err)
	}
	return iniProfilePath("/media/fat", index)
}

// iniProfileIndex follows Main_MiSTer's altcfg shared-memory record. An unset
// signature means Main, as it does in the host. Unknown indices fail closed.
func iniProfileIndex(record []byte) (int, error) {
	if len(record) != 4 {
		return 0, errors.New("incomplete MiSTer INI selection record")
	}
	if record[0] != 0x34 || record[1] != 0x99 || record[2] != 0xba {
		return 0, nil
	}
	if record[3] > 3 {
		return 0, errors.New("unsupported MiSTer INI profile index")
	}
	return int(record[3]), nil
}

// iniProfilePath matches cfg_get_name's case-insensitive ordering. More than
// three alternate files is ambiguous because the host caches only the first
// three directory entries. Refuse that layout instead of editing a guessed file.
func iniProfilePath(root string, index int) (string, error) {
	if index < 0 || index > 3 {
		return "", errors.New("unsupported MiSTer INI profile index")
	}
	name := "MiSTer.ini"
	if index != 0 {
		entries, err := os.ReadDir(root)
		if err != nil {
			return "", err
		}
		var names []string
		for _, entry := range entries {
			lower := iniNameKey(entry.Name())
			if strings.HasPrefix(lower, "mister_") && strings.HasSuffix(lower, ".ini") {
				if len(entry.Name()) > 63 {
					return "", errors.New("alternate MiSTer INI filename exceeds host limit")
				}
				names = append(names, entry.Name())
			}
		}
		if len(names) > 3 {
			return "", errors.New("cannot resolve active profile with more than three alternate MiSTer INI files")
		}
		sort.Slice(names, func(i, j int) bool { return iniNameKey(names[i]) < iniNameKey(names[j]) })
		for i := 1; i < len(names); i++ {
			if iniNameKey(names[i-1]) == iniNameKey(names[i]) {
				return "", errors.New("ambiguous alternate MiSTer INI filenames")
			}
		}
		if index > len(names) {
			return "", errors.New("selected alternate MiSTer INI file is missing")
		}
		name = names[index-1]
	}
	path := filepath.Join(root, name)
	info, err := os.Lstat(path)
	if err != nil {
		return "", fmt.Errorf("active MiSTer INI profile: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("active MiSTer INI profile must be a regular file")
	}
	return path, nil
}

// iniNameKey matches the host's ASCII case folding without reordering UTF-8 names.
func iniNameKey(name string) string {
	b := []byte(name)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}

// configureINI preserves a per-profile original before publishing a managed
// section. Existing backups are retained, including backups from Main-only builds.
func configureINI(path, directory, purpose string, transform func([]byte) ([]byte, error)) error {
	original, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	configured, err := transform(original)
	if err != nil {
		return err
	}
	if string(configured) == string(original) {
		return nil
	}
	backup := filepath.Join(directory, filepath.Base(path)+".before-"+purpose)
	// Display supervisors hold the display lock across this check and publish.
	// Write and sync a temporary file before exposing the final backup name.
	// An interrupted temporary write cannot become the next launch's backup.
	info, err := os.Lstat(backup)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if err := replace(backup, original); err != nil {
			return err
		}
	case err != nil:
		return err
	case !info.Mode().IsRegular():
		return errors.New("MiSTer INI backup must be a regular file")
	}
	return replace(path, configured)
}
