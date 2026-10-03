package plex

import "testing"

func TestInheritedMusicBackdrop(t *testing.T) {
	for _, tc := range []struct{ art, parent, artist, want string }{
		{"track", "album", "artist", "track"}, {"", "album", "artist", "album"}, {"", "", "artist", "artist"}, {"", "", "", ""},
	} {
		item := (metadata{Type: "track", Art: tc.art, ParentArt: tc.parent, GrandparentArt: tc.artist}).item()
		if tc.want == "" {
			if len(item.BackdropImageTags) != 0 {
				t.Fatal("invented backdrop")
			}
			continue
		}
		if len(item.BackdropImageTags) != 1 || item.BackdropImageTags[0] != tc.want {
			t.Fatalf("got %v, want %s", item.BackdropImageTags, tc.want)
		}
	}
}
