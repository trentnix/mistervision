package displaymode

import "testing"

func TestMainPhysicalDiscRecognition(t *testing.T) {
	for _, tc := range []struct {
		name, path string
		want       bool
	}{
		{"MiSTer", "/media/fat/MiSTer", true},
		{"MiSTer_Physical", "/media/fat/MiSTer_Physical-CD", true},
		{"MiSTer_Physical", "/tmp/unrelated", false},
		{"MiSTer_Physical", "/media/fat/MiSTer_Physical-CD-other", false},
	} {
		if got := isMainProcess(tc.name, tc.path); got != tc.want {
			t.Fatalf("%q %q: %v", tc.name, tc.path, got)
		}
	}
}
