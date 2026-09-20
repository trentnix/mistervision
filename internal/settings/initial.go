package settings

import "encoding/json"

// InitializeDisplay saves the installer's display preset only when no current
// or legacy settings exist. Writing exclusively prevents replacement of settings
// created concurrently. Later updates cannot change this saved preference.
func (f *File) InitializeDisplay(interlaced bool) error {
	if !f.legacy {
		return nil
	}
	for _, section := range f.sources {
		if section.Data != nil || section.Err != nil {
			return nil
		}
	}
	data, err := json.MarshalIndent(map[string]any{"display": map[string]bool{"interlaced": interlaced}}, "", "  ")
	if err != nil {
		return err
	}
	return writeNewSettings(f.Path, append(data, '\n'))
}
