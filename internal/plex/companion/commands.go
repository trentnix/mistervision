package companion

import (
	"errors"
	"math"
	"mistervision/internal/remote"
	"net/url"
	"strconv"
	"strings"
)

// command accepts identifiers, never stream URLs or alternate server credentials.
func (c *receiver) command(path string, q map[string]string) (remote.Command, string, error) {
	invalid := errors.New(messageVideo)
	if q["type"] != "" && q["type"] != "video" {
		return remote.Command{}, "", invalid
	}
	cmd := remote.Command{}
	switch path {
	case "/player/playback/playMedia":
		if q["machineIdentifier"] != c.config.ServerID || (q["providerIdentifier"] != "" && q["providerIdentifier"] != "com.plexapp.plugins.library") {
			return cmd, "", invalid
		}
		id, ok := strings.CutPrefix(q["key"], "/library/metadata/")
		if !decimalID(id) || !ok {
			return cmd, "", invalid
		}
		container := q["containerKey"]
		if container != "" {
			u, err := url.Parse(container)
			if err != nil || u.IsAbs() || u.Host != "" || u.Fragment != "" || u.RawPath != "" {
				return cmd, "", invalid
			}
			queueID, ok := strings.CutPrefix(u.Path, "/playQueues/")
			values, err := url.ParseQuery(u.RawQuery)
			if !ok || !decimalID(queueID) || err != nil || len(values) > 1 || (len(values) == 1 && (len(values["own"]) != 1 || values.Get("own") != "1")) {
				return cmd, "", invalid
			}
		}
		position, err := offset(q["offset"], true)
		if err != nil {
			return cmd, "", err
		}
		cmd.Kind, cmd.PlayMode, cmd.IDs, cmd.Position = remote.Play, remote.PlayNow, []string{id}, &position
		return cmd, container, nil
	case "/player/playback/pause":
		cmd.Kind = remote.Pause
	case "/player/playback/play":
		cmd.Kind = remote.Resume
	case "/player/playback/stop":
		cmd.Kind = remote.Stop
	case "/player/playback/seekTo":
		position, err := offset(q["offset"], false)
		if err != nil {
			return cmd, "", err
		}
		cmd.Kind, cmd.Position = remote.Seek, &position
	default:
		return cmd, "", invalid
	}
	return cmd, "", nil
}
func decimalID(value string) bool {
	if len(value) == 0 || len(value) > 20 {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
func offset(value string, optional bool) (int64, error) {
	if value == "" && optional {
		return 0, nil
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil || n < 0 || n > math.MaxInt64/10000 {
		return 0, errors.New(messageParameters)
	}
	return n * 10000, nil
}
