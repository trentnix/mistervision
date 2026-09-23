package displaymode

import "strings"

// MatchMenuSection returns the known global or core name matched by an INI
// section, or an empty string for an unrelated section. The caller removes the
// opening [ or +. Matching follows Main_MiSTer's case-insensitive prefix rules.
// Menu always participates. Named MGL cores can be supplied as additional names.
// Returning a known name lets diagnostics include wildcard/group settings
// without exposing arbitrary section text.
func MatchMenuSection(section string, names ...string) string {
	section, _, _ = strings.Cut(section, "]")
	section = iniNameKey(section)
	if section == "mister" {
		return "mister"
	}
	for _, name := range append([]string{"menu"}, names...) {
		name = iniNameKey(name)
		if name == "" {
			continue
		}
		if wildcard := strings.LastIndexByte(section, '*'); wildcard >= 0 {
			if strings.HasPrefix(name, section[:wildcard]) {
				return name
			}
		} else if section == name {
			return name
		}
	}
	return ""
}
