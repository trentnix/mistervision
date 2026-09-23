//go:build !linux

package displaymode

import "errors"

func activeINIIndex() (int, error) {
	return 0, errors.New("MiSTer INI selection requires Linux")
}
