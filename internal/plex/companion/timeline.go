package companion

import "mistervision/internal/remote"

// timeline uses decoder facts. A receipt means the application processed a
// command, not that a load or seek has finished. Stops are sent once per item so
// repeated idle polls cannot dismiss Plex Web's next resume/start-over dialog.
// c.mu must be held.
func (c *receiver) timeline(state remote.PlaybackState, entry *controller) timelineContainer {
	result := timelineContainer{Location: "navigation", CommandID: entry.commandID}
	if state.Audio || state.Live || state.ItemID == "" {
		return result
	}
	if state.Status == remote.Stopped {
		if !entry.sawPlayback {
			return result
		}
		entry.sawPlayback = false
	} else {
		entry.sawPlayback = true
	}
	base := timelineXML(state)
	result.Location = base.Location
	t := base.Timelines[0]
	t.MachineIdentifier, t.ProviderIdentifier = c.config.ServerID, "com.plexapp.plugins.library"
	t.RatingKey, t.Key = state.ItemID, "/library/metadata/"+state.ItemID
	if state.ItemID == c.itemID {
		t.ContainerKey = c.containerKey
	}
	t.Controllable = "playPause,stop,seekTo"
	result.Timelines = []timeline{t}
	return result
}
