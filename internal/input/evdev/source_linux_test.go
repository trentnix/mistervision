//go:build linux

package evdev

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mistervision/internal/diagnostics"
	"mistervision/internal/input/control"
)

func inheritedProfile() Profile {
	return Profile{Replace: true, Buttons: map[uint16]control.Action{305: control.Open, 304: control.Back, 310: control.TrackPrevious},
		Axes:       map[uint16]Axis{2: {Positive: control.TrackNext}, 16: {Negative: control.Previous, Positive: control.Next}},
		AxisLabels: map[uint16]AxisLabels{2: {Positive: "R"}}}
}

func TestExplicitProfilesOverrideSavedBindings(t *testing.T) {
	base := inheritedProfile()
	config := Config{Profiles: []Profile{{Match: "Pad", Buttons: map[uint16]control.Action{305: "", 307: control.Open}, Axes: map[uint16]Axis{16: {}}, AxisLabels: map[uint16]AxisLabels{2: {Positive: "Shoulder"}}}}}
	p := config.bindingsOn("Pad", base)
	d := configuredTestAxes("Pad", advertised(304, 305, 307, 310), p, map[uint16][2]int32{2: {0, 255}, 16: {-1, 1}})
	d.overrides = config.bindings("Pad")
	if got := d.accept(event{Type: 1, Code: 305, Value: 1}); got != "" {
		t.Fatal("disabled saved binding fired", got)
	}
	if got := d.accept(event{Type: 1, Code: 307, Value: 1}); got != control.Open {
		t.Fatal(got)
	}
	if got := d.accept(event{Type: 3, Code: 16, Value: 1}); got != "" {
		t.Fatal("disabled saved axis fired", got)
	}
	labels := d.labels(advertised(304, 305, 307, 310))
	if labels.Name(control.Open) != "X" || labels.Name(control.TrackNext) != "Shoulder" {
		t.Fatal(labels)
	}
	if base.Buttons[305] != control.Open || base.Axes[16].Positive != control.Next {
		t.Fatal("profile merge mutated source")
	}
	config.Profiles = append(config.Profiles, Profile{Match: "Pad", Replace: true, Buttons: map[uint16]control.Action{28: control.Open}})
	p = config.bindingsOn("Pad", base)
	if len(p.Buttons) != 1 || len(p.Axes) != 0 || len(p.AxisLabels) != 0 {
		t.Fatal("replace retained saved bindings", p)
	}
}

func TestExplicitLabelWinsOverSavedAlias(t *testing.T) {
	config := Config{Profiles: []Profile{{Match: "Pad", Buttons: map[uint16]control.Action{307: control.Open}}}}
	d := device{name: "Pad", bindings: config.bindingsOn("Pad", inheritedProfile()), overrides: config.bindings("Pad")}
	if got := d.labels(advertised(305, 307)).Name(control.Open); got != "X" {
		t.Fatal(got)
	}
}

func TestSavedHatCenterAndTriggerRelease(t *testing.T) {
	d := configuredTestAxes("Pad", advertised(304, 305), inheritedProfile(), map[uint16][2]int32{2: {0, 255}, 16: {-1, 1}})
	for _, step := range []struct {
		code  uint16
		value int32
		want  control.Action
	}{
		{16, 0, ""}, {16, 1, control.Next}, {16, 0, ""}, {16, -1, control.Previous}, {16, 0, ""},
		{2, 0, ""}, {2, 255, control.TrackNext}, {2, 240, ""}, {2, 0, ""}, {2, 255, control.TrackNext}, {2, 0, ""},
	} {
		if got := d.accept(event{Type: 3, Code: step.code, Value: step.value}); got != step.want {
			t.Fatal(step, got)
		}
	}
	if len(d.held) != 0 {
		t.Fatal("release stuck", d.held)
	}
	for _, code := range []uint16{312, 313} {
		if got := d.accept(event{Type: 1, Code: code, Value: 1}); got != "" {
			t.Fatal("unmapped trigger gained action", got)
		}
	}
}

func TestMappedAxisWaitsForInitialRelease(t *testing.T) {
	a := newMappedAxis(Axis{Positive: control.TrackNext}, 0, 255)
	a.waitForNeutral = a.action(255) != ""
	if got := a.action(250); got != "" {
		t.Fatal("startup hold fired", got)
	}
	if got := a.action(0); got != "" {
		t.Fatal(got)
	}
	if got := a.action(255); got != control.TrackNext {
		t.Fatal("new press lost", got)
	}
}

func TestMappedAndUnmappedControllersKeepVirtualNavigation(t *testing.T) {
	mapped := configuredTestAxes("Pad", advertised(304, 305), inheritedProfile(), map[uint16][2]int32{16: {-1, 1}})
	virtual := device{name: "MiSTer virtual input", held: make(map[uint16]control.Action)}
	unmapped := device{name: "Other Pad", held: make(map[uint16]control.Action)}
	var nav navigation
	now := time.Unix(0, 0)
	check := func(d *device, e event, want control.Action) {
		t.Helper()
		pressed := map[control.Action]bool{}
		if key := d.accept(e); key != "" {
			pressed[key] = true
		}
		held := map[control.Action]bool{}
		for _, pad := range []*device{&mapped, &virtual, &unmapped} {
			for _, key := range pad.held {
				held[key] = true
			}
		}
		got := nav.update(held, pressed, now)
		now = now.Add(time.Millisecond)
		if want == "" && len(got) == 0 {
			return
		}
		if len(got) != 1 || got[0] != want {
			t.Fatalf("got %v, want %s", got, want)
		}
	}
	check(&mapped, event{Type: 3, Code: 16, Value: 1}, control.Next)
	check(&virtual, event{Type: 1, Code: 106, Value: 1}, "")
	check(&mapped, event{Type: 3, Code: 16, Value: 0}, "")
	check(&virtual, event{Type: 1, Code: 106, Value: 0}, "")
	// An unmapped pad may supply directions only through MiSTer's virtual node.
	check(&virtual, event{Type: 1, Code: 108, Value: 1}, control.Down)
	check(&virtual, event{Type: 1, Code: 108, Value: 0}, "")
	if got := mapped.accept(event{Type: 1, Code: 305, Value: 1}); got != control.Open {
		t.Fatal(got)
	}
	if got := virtual.accept(event{Type: 1, Code: 28, Value: 1}); got != "" {
		t.Fatal("confirm echo duplicated", got)
	}
	if got := unmapped.accept(event{Type: 1, Code: 305, Value: 1}); got != control.Open {
		t.Fatal("unmapped pad disabled", got)
	}
}

// mappingStub models saved-map success, absence, and corruption without devices.
type mappingStub struct {
	profile Profile
	err     error
}

func (s mappingStub) Bindings(DeviceInfo) (Profile, error) { return s.profile, s.err }

func TestMappingFallbackIsLoggedAndStillAcceptsInput(t *testing.T) {
	for _, tc := range []struct {
		name         string
		source       mappingStub
		status, kind string
	}{
		{"saved", mappingStub{profile: inheritedProfile()}, "saved", "none"},
		{"missing", mappingStub{}, "default", "none"},
		{"invalid", mappingStub{profile: inheritedProfile(), err: ErrInvalidMapping}, "fallback", "invalid-mapping"},
		{"unreadable", mappingStub{err: os.ErrPermission}, "fallback", "permission"},
		{"unavailable controls", mappingStub{profile: Profile{Replace: true, Axes: map[uint16]Axis{2: {Positive: control.Open}, 5: {Positive: control.Back}}}}, "fallback", "unavailable-controls"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "input.jsonl")
			log, err := diagnostics.Open(diagnostics.Config{Enabled: true, Path: path, MaxBytes: 4096})
			if err != nil {
				t.Fatal(err)
			}
			base := sourceBindings(tc.source, DeviceInfo{Vendor: 0x45e, Product: 0x28e, Physical: "private-path", Unique: "private-serial"}, deviceCapabilities{keys: advertised(304, 305)}, log)
			if err := log.Close(); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var entry map[string]any
			if err := json.Unmarshal(data, &entry); err != nil {
				t.Fatal(err)
			}
			if entry["status"] != tc.status || entry["error_kind"] != tc.kind {
				t.Fatal(entry)
			}
			if strings.Contains(string(data), "private-") {
				t.Fatal("private identity logged")
			}
			if base.Replace != (tc.name == "saved") {
				t.Fatal("bad fallback", base)
			}
			config := Config{Profiles: []Profile{{Match: "Pad", Buttons: map[uint16]control.Action{307: control.Select}}}}
			d := device{name: "Pad", held: make(map[uint16]control.Action), bindings: config.bindingsOn("Pad", base)}
			if got := d.accept(event{Type: 1, Code: 305, Value: 1}); got != control.Open {
				t.Fatal("confirm unavailable", got)
			}
			if got := d.accept(event{Type: 1, Code: 307, Value: 1}); got != control.Select {
				t.Fatal("override lost", got)
			}
		})
	}
}

func TestSavedMappingsRequireReachableConfirmAndBack(t *testing.T) {
	buttons := Profile{Replace: true, Buttons: map[uint16]control.Action{304: control.Back, 305: control.Open, 310: control.TrackPrevious}}
	axes := Profile{Replace: true, Axes: map[uint16]Axis{2: {Positive: control.Open}, 5: {Positive: control.Back}}}
	for _, tc := range []struct {
		name      string
		profile   Profile
		caps      deviceCapabilities
		wantSaved bool
	}{
		{"buttons present, optional shoulder absent", buttons, deviceCapabilities{keys: advertised(304, 305)}, true},
		{"confirm button absent", buttons, deviceCapabilities{keys: advertised(304)}, false},
		{"back button absent", buttons, deviceCapabilities{keys: advertised(305)}, false},
		{"both saved axes absent", axes, deviceCapabilities{keys: advertised(304, 305)}, false},
		{"both axes present", axes, deviceCapabilities{axes: map[uint16]absoluteInfo{2: {Minimum: 0, Maximum: 255}, 5: {Minimum: 0, Maximum: 255}}}, true},
		{"one axis missing", axes, deviceCapabilities{axes: map[uint16]absoluteInfo{2: {Minimum: 0, Maximum: 255}}}, false},
		{"degenerate axis", axes, deviceCapabilities{axes: map[uint16]absoluteInfo{2: {Minimum: 0, Maximum: 255}, 5: {Minimum: 0, Maximum: 0}}}, false},
		{"inverted range", axes, deviceCapabilities{axes: map[uint16]absoluteInfo{2: {Minimum: 0, Maximum: 255}, 5: {Minimum: 255, Maximum: 0}}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := sourceBindings(mappingStub{profile: tc.profile}, DeviceInfo{}, tc.caps, nil)
			if base.Replace != tc.wantSaved {
				t.Fatalf("saved=%v, want %v", base.Replace, tc.wantSaved)
			}
			if !tc.wantSaved {
				// Failed map validation must preserve defaults and still allow
				// application overrides, including intentional disabled actions.
				config := Config{Profiles: []Profile{{Match: "Pad", Buttons: map[uint16]control.Action{307: control.Select, 310: ""}}}}
				d := device{name: "Pad", held: make(map[uint16]control.Action), bindings: config.bindingsOn("Pad", base)}
				for code, want := range map[uint16]control.Action{304: control.Back, 305: control.Open, 307: control.Select, 310: ""} {
					if got := d.accept(event{Type: 1, Code: code, Value: 1}); got != want {
						t.Fatalf("code %d: got %s, want %s", code, got, want)
					}
				}
			}
		})
	}
}

func TestExplicitDisablingStillWinsOverUsableSavedMap(t *testing.T) {
	base := sourceBindings(mappingStub{profile: inheritedProfile()}, DeviceInfo{}, deviceCapabilities{keys: advertised(304, 305)}, nil)
	config := Config{Profiles: []Profile{{Match: "Pad", Buttons: map[uint16]control.Action{304: "", 305: ""}}}}
	d := device{name: "Pad", held: make(map[uint16]control.Action), bindings: config.bindingsOn("Pad", base)}
	for _, code := range []uint16{304, 305} {
		if got := d.accept(event{Type: 1, Code: code, Value: 1}); got != "" {
			t.Fatal("explicit disabling was replaced by fallback", got)
		}
	}
}
