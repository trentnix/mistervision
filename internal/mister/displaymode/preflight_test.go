package displaymode

import (
	"strings"
	"testing"
)

func TestValidateMenuFramebuffer(t *testing.T) {
	for _, tc := range []struct {
		name, ini string
		reject    bool
	}{
		{"default", "[MiSTer]\nvga_scaler=0\n", false},
		{"HDMI", "[MiSTer]\nfb_terminal=1\nvga_scaler=0\n", false},
		{"analog", "[Menu]\nfb_terminal=1\nvga_scaler=1\n", false},
		{"disabled", "[MiSTer]\nfb_terminal=0\n", true},
		{"menu override", "[MiSTer]\nfb_terminal=1\n[menu]\nfb_terminal=0\n", true},
		{"menu enabled", "[MiSTer]\nfb_terminal=0\n[Menu]\nfb_terminal=1\n", false},
		{"unrelated core", "[MiSTer]\nfb_terminal=1\n[Other]\nfb_terminal=0\n", false},
		{"wildcard", "[MiSTer]\nfb_terminal=1\n[Men*]\nfb_terminal=0 ; disabled\n", true},
		{"group", "[MiSTer]\nfb_terminal=1\n[Other]\n+Menu\nfb_terminal=0\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateMenuFramebuffer([]byte(tc.ini), "/media/fat/MiSTER_RGsb.ini")
			if (err != nil) != tc.reject {
				t.Fatalf("unexpected result: %v", err)
			}
			if err != nil && (!strings.Contains(err.Error(), "MiSTER_RGsb.ini") || !strings.Contains(err.Error(), "fb_terminal=1")) {
				t.Fatalf("missing corrective guidance: %v", err)
			}
		})
	}
}
