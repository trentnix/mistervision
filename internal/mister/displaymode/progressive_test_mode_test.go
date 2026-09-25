package displaymode

import "testing"

func TestProgressiveTestRequiresExactOptInAndOneSupervisor(t *testing.T) {
	for _, value := range []string{"", "0", "true", "1 "} {
		t.Setenv("MISTERVISION_PROGRESSIVE_TEST", value)
		t.Setenv(scaledEnv, "")
		if ProgressiveTestRequested() {
			t.Fatalf("accepted %q", value)
		}
	}
	t.Setenv("MISTERVISION_PROGRESSIVE_TEST", "1")
	if !ProgressiveTestRequested() {
		t.Fatal("explicit test was not enabled")
	}
	t.Setenv(scaledEnv, "1")
	if ProgressiveTestRequested() {
		t.Fatal("supervised child tried to supervise again")
	}
}

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
