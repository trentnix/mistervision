package update

import (
	"bufio"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const downloaderDatabase = "multidatabases/mister-vision"

// DownloaderRegistered reports whether the standard Downloader configuration
// registers MiSTerVision's database. Root is normally /media/fat. This observes
// update ownership, not whether a download succeeded or filters include every
// file. Custom launcher paths and environment-only registrations are not read.
// The first database section wins, matching Downloader's documented precedence.
func DownloaderRegistered(root string) (bool, error) {
	paths := []string{filepath.Join(root, "downloader.ini")}
	for _, pattern := range []string{"downloader/*.ini", "downloader_*.ini"} {
		matches, err := filepath.Glob(filepath.Join(root, pattern))
		if err != nil {
			return false, err
		}
		for _, path := range matches {
			if !strings.HasPrefix(filepath.Base(path), ".") {
				paths = append(paths, path)
			}
		}
	}
	if len(paths) > 128 {
		return false, errors.New("too many Downloader configuration files")
	}
	for _, path := range paths {
		found, err := registeredFile(path)
		if err != nil || found {
			return found, err
		}
	}
	return false, nil
}

// registeredFile reads only regular, bounded configuration files. It never reads
// Downloader's private cache format or treats a filename alone as registration.
func registeredFile(path string) (bool, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, nil
	}
	if info.Size() > 256<<10 {
		return false, errors.New("Downloader configuration is too large")
	}
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()
	return registeredDatabase(io.LimitReader(file, (256<<10)+1))
}

// registeredDatabase recognizes the database section and its URL without trying
// to interpret Downloader's filters or other application-specific options.
func registeredDatabase(source io.Reader) (bool, error) {
	scan := bufio.NewScanner(source)
	scan.Buffer(make([]byte, 4096), 256<<10)
	selected, seenURL := false, false
	url := ""
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())
		for i, c := range line {
			if (c == '#' || c == ';') && (i == 0 || line[i-1] == ' ' || line[i-1] == '\t') {
				line = strings.TrimSpace(line[:i])
				break
			}
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			if selected {
				break
			}
			selected = strings.EqualFold(strings.TrimSpace(line[1:len(line)-1]), downloaderDatabase)
			continue
		}
		if !selected {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			key, value, ok = strings.Cut(line, ":")
		}
		if !ok || !strings.EqualFold(strings.TrimSpace(key), "db_url") {
			continue
		}
		if seenURL {
			return false, errors.New("duplicate MiSTerVision Downloader URL")
		}
		seenURL = true
		url = strings.Trim(strings.TrimSpace(value), "\"'")
	}
	if err := scan.Err(); err != nil {
		return false, err
	}
	if selected && url == "" {
		return false, errors.New("MiSTerVision Downloader registration has no URL")
	}
	return selected, nil
}
