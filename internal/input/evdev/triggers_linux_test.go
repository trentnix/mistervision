//go:build linux

package evdev

import (
	"testing"

	"mistervision/internal/input/control"
)

// TestReportedTriggerRestValues records issue #30's current failure mechanism.
// Change the default expectations once physical axis assignments are confirmed.
// The same reported values must remain idle with an explicit centered binding.
func TestReportedTriggerRestValues(t *testing.T) {
	for _, tc := range []struct {
		name      string
		code      uint16
		max, rest int32
		want      control.Action
	}{
		{"ABS_Z", 2, 65535, 32527, control.SeekBackward},
		{"ABS_RZ", 5, 65535, 32637, control.SeekForward},
		{"ABS_BRAKE", 10, 1023, 0, ""},
		{"ABS_GAS", 9, 1023, 0, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			axis := newMappedAxis(defaultTriggerBinding(tc.code), 0, tc.max)
			if got := axis.action(tc.rest); got != tc.want {
				t.Fatalf("default at rest: %q, want %q", got, tc.want)
			}
			if tc.code != 2 && tc.code != 5 {
				return
			}
			binding := Axis{Rest: "center", Negative: tc.want, Positive: tc.want}
			config := Config{Profiles: []Profile{{Match: "*Xbox Wireless*", Axes: map[uint16]Axis{tc.code: binding}}}}
			if err := config.Validate(); err != nil {
				t.Fatal(err)
			}
			d := device{
				name:     "Xbox Wireless Controller",
				bindings: config.bindings("Xbox Wireless Controller"),
				axes:     map[uint16]*mappedAxis{tc.code: newMappedAxis(binding, 0, tc.max)},
				triggers: map[uint16]*mappedAxis{tc.code: axis},
				held:     make(map[uint16]control.Action),
			}
			for _, value := range []int32{tc.rest, tc.rest - 100, tc.rest + 100} {
				if got := d.accept(event{Type: 3, Code: tc.code, Value: value}); got != "" || len(d.held) != 0 {
					t.Fatalf("neutral noise caused seek: value=%d action=%q held=%v", value, got, d.held)
				}
			}
			// Both directions are intentional in the reporter's override. This
			// verifies its behavior without claiming either is a physical trigger.
			for _, endpoint := range []int32{0, tc.max} {
				if got := d.accept(event{Type: 3, Code: tc.code, Value: endpoint}); got != tc.want {
					t.Fatalf("endpoint %d: %q, want %q", endpoint, got, tc.want)
				}
				if got := d.accept(event{Type: 3, Code: tc.code, Value: endpoint}); got != "" {
					t.Fatal("duplicate motion generated another press")
				}
				d.accept(event{Type: 3, Code: tc.code, Value: tc.rest})
				if len(d.held) != 0 {
					t.Fatal("returning to neutral left seeking held")
				}
			}
		})
	}
}
