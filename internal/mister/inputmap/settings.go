package inputmap

import (
	"bufio"
	"errors"
	"io"
	"strconv"
	"strings"

	"mistervision/internal/mister/displaymode"
)

// ErrVideoMapping means controller identity rules depend on a video section.
// The reader cannot resolve the host's active video-section selection and must
// fall back instead of looking up a potentially different controller map.
var ErrVideoMapping = errors.New("controller_unique_mapping in a video-dependent section is unsupported")

// readUniqueRules reads only controller identity settings. Core-specific input
// maps do not apply to the menu. MiSTer's array values replace earlier sections
// when the first occurrence in a matching section is encountered.
func readUniqueRules(r io.Reader, core string) ([]uint32, error) {
	limited := &io.LimitedReader{R: r, N: (128 << 10) + 1}
	scan := bufio.NewScanner(limited)
	matched, first, video := false, true, false
	var rules []uint32
	for scan.Scan() {
		line := strings.TrimSpace(strings.SplitN(scan.Text(), ";", 2)[0])
		if strings.HasPrefix(line, "[") {
			matched = displaymode.MatchMenuSection(line[1:], core) != ""
			video = isVideoSection(line[1:])
			first = true
			continue
		}
		if strings.HasPrefix(line, "+") {
			matched = matched || displaymode.MatchMenuSection(line[1:], core) != ""
			video = video || isVideoSection(line[1:])
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || !strings.EqualFold(strings.TrimSpace(key), "controller_unique_mapping") {
			continue
		}
		if !matched {
			if video {
				return nil, ErrVideoMapping
			}
			continue
		}
		n, err := strconv.ParseUint(strings.TrimSpace(value), 0, 32)
		if err != nil {
			return nil, errors.New("invalid controller_unique_mapping")
		}
		if first {
			rules = nil
			first = false
		}
		rules = append(rules, uint32(n))
		if len(rules) > 256 {
			return nil, errors.New("too many controller_unique_mapping entries")
		}
	}
	if err := scan.Err(); err != nil {
		return nil, err
	}
	if limited.N == 0 {
		return nil, errors.New("MiSTer INI exceeds controller settings limit")
	}
	return rules, nil
}

// isVideoSection identifies conditional groups, including +video= inclusions.
// Other settings within those groups do not affect controller map selection.
func isVideoSection(section string) bool {
	name, _, found := strings.Cut(section, "=")
	return found && strings.EqualFold(name, "video")
}
