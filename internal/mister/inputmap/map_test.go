package inputmap

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"mistervision/internal/input/control"
	"mistervision/internal/input/evdev"
)

func mapBytes(slots map[int]uint32) []byte {
	data := make([]byte, mapSize)
	for slot, value := range slots {
		binary.LittleEndian.PutUint32(data[slot*4:], value)
	}
	return data
}

// These assignments reproduce the M30 case documented in MiSTerFin PR #41:
// D-pad on stick axes, R on ABS_Z, and menu Back on ABS_RZ.
func m30Slots() map[int]uint32 {
	return map[int]uint32{0: 769, 1: 768, 2: 771, 3: 770, 4: 779, 5: 305,
		6: 311, 7: 308, 8: 310, 9: 773, 10: 314, 11: 315,
		23: 305 | 779<<16, 24: 0x20000, 25: 0x20001}
}

func TestMenuBindingsAndAxisEdges(t *testing.T) {
	p, err := parse(mapBytes(m30Slots()))
	if err != nil {
		t.Fatal(err)
	}
	if !p.Replace || p.Buttons[305] != control.Open || p.Buttons[311] != control.Options || p.Buttons[310] != control.TrackPrevious || p.Buttons[314] != control.Select || p.Buttons[315] != control.About {
		t.Fatal(p)
	}
	if p.Axes[0].Negative != control.Previous || p.Axes[0].Positive != control.Next || p.Axes[1].Negative != control.Up || p.Axes[1].Positive != control.Down {
		t.Fatal(p.Axes)
	}
	if p.Axes[2].Positive != control.TrackNext || p.Axes[2].Negative != "" || p.Axes[5].Positive != control.Back {
		t.Fatal(p.Axes)
	}
	if p.AxisLabels[2].Positive != "R" {
		t.Fatal("axis code was mistaken for an LT label", p.AxisLabels)
	}
	if _, ok := p.Buttons[308]; ok {
		t.Fatal("unassigned Y gained a binding")
	}
	if err := (evdev.Config{Profiles: []evdev.Profile{{Match: "*", Replace: p.Replace, Buttons: p.Buttons, Axes: p.Axes, AxisLabels: p.AxisLabels}}}).Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestMenuFallbackAndMappedNavigationStick(t *testing.T) {
	slots := map[int]uint32{4: 305, 5: 304, 28: 0x20003, 29: 0x20004}
	p, err := parse(mapBytes(slots))
	if err != nil {
		t.Fatal(err)
	}
	if p.Buttons[305] != control.Open || p.Buttons[304] != control.Back || p.Axes[3].Positive != control.Next || p.Axes[4].Negative != control.Up {
		t.Fatal(p)
	}
	if _, exists := p.Axes[0]; exists {
		t.Fatal("assumed ABS_X despite saved stick mapping")
	}
	// A menu action wins over a duplicate core button assignment.
	slots[6] = 305
	p, err = parse(mapBytes(slots))
	if err != nil || p.Buttons[305] != control.Open {
		t.Fatal(p, err)
	}
}

func TestRejectInvalidMaps(t *testing.T) {
	for _, data := range [][]byte{nil, make([]byte, 127), make([]byte, 129), make([]byte, 128),
		mapBytes(map[int]uint32{4: 305, 5: 305}),
		mapBytes(map[int]uint32{4: 305, 5: 304, 9: 0xffffffff}),
		mapBytes(map[int]uint32{4: 305, 5: 304, 28: 0x20040})} {
		if _, err := parse(data); err == nil {
			t.Fatal("invalid map accepted")
		}
	}
}

func TestSourceMissingInvalidAndIndependentReads(t *testing.T) {
	dir := t.TempDir()
	s := &Source{directory: dir}
	d := evdev.DeviceInfo{Vendor: 0x45e, Product: 0x28e}
	if p, err := s.Bindings(d); err != nil || p.Replace {
		t.Fatal(p, err)
	}
	path := filepath.Join(dir, "input_045e_028e_v3.map")
	for _, data := range [][]byte{make([]byte, 127), append(mapBytes(m30Slots()), 0), make([]byte, 128)} {
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		if p, err := s.Bindings(d); !errors.Is(err, evdev.ErrInvalidMapping) || p.Replace {
			t.Fatal(p, err)
		}
	}
	if err := os.WriteFile(path, mapBytes(m30Slots()), 0600); err != nil {
		t.Fatal(err)
	}
	p, err := s.Bindings(d)
	if err != nil {
		t.Fatal(err)
	}
	p.Buttons[305] = control.Quit
	again, err := s.Bindings(d)
	if err != nil || again.Buttons[305] != control.Open {
		t.Fatal("reconnected device reused mutable bindings", again, err)
	}
	// A unique controller must not silently inherit another controller's map.
	s.unique = []uint32{1}
	if p, err := s.Bindings(d); err != nil || p.Replace {
		t.Fatal(p, err)
	}
}

func TestMapNames(t *testing.T) {
	for _, tc := range []struct {
		name   string
		device evdev.DeviceInfo
		rules  []uint32
		want   string
	}{
		{"standard", evdev.DeviceInfo{Vendor: 0x45e, Product: 0x28e}, nil, "input_045e_028e_v3.map"},
		{"adapter", evdev.DeviceInfo{Vendor: 0x16d0, Product: 0x127e, Unique: "SNES: pad/1*"}, nil, "input_16d0_127e_SNES__pad_1__v3.map"},
		{"name checksum", evdev.DeviceInfo{Vendor: 0x1209, Product: 0xface, Name: "ABC"}, nil, "input_1209_face_198_v3.map"},
		{"unique", evdev.DeviceInfo{Vendor: 0x45e, Product: 0x28e, Physical: "a", Unique: "b"}, []uint32{1}, "input_045e_028e_7c931539_v3.map"},
		{"other rule", evdev.DeviceInfo{Vendor: 0x45e, Product: 0x28e}, []uint32{0x00790006}, "input_045e_028e_v3.map"},
		{"stop at zero", evdev.DeviceInfo{Vendor: 0x45e, Product: 0x28e}, []uint32{0, 1}, "input_045e_028e_v3.map"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := mapName(tc.device, tc.rules); got != tc.want {
				t.Fatalf("%q, want %q", got, tc.want)
			}
		})
	}
	d := evdev.DeviceInfo{Vendor: 0x45e, Product: 0x28e, Physical: "usb1", Unique: "abc"}
	if mapName(d, []uint32{0x045e028e}) != mapName(d, []uint32{1}) {
		t.Fatal("VID/PID rule not applied")
	}
}

func TestUniqueRulesRespectSections(t *testing.T) {
	text := `controller_unique_mapping=1
[MiSTer]
controller_unique_mapping=0x045e028e
controller_unique_mapping=0x00790006
[OtherCore]
controller_unique_mapping=invalid
[MiSTerVisionInterlaced]
controller_unique_mapping=0
[Other]
+Menu
controller_unique_mapping=1
`
	got, err := readUniqueRules(strings.NewReader(text), "MiSTerVisionFramebuffer")
	if err != nil || !reflect.DeepEqual(got, []uint32{1}) {
		t.Fatal(got, err)
	}
	text = strings.Split(text, "[Other]\n")[0]
	got, err = readUniqueRules(strings.NewReader(text), "MiSTerVisionFramebuffer")
	if err != nil || !reflect.DeepEqual(got, []uint32{0x045e028e, 0x00790006}) {
		t.Fatal(got, err)
	}
	got, err = readUniqueRules(strings.NewReader(text), "MiSTerVisionInterlaced")
	if err != nil || !reflect.DeepEqual(got, []uint32{0}) {
		t.Fatal(got, err)
	}
	if _, err := readUniqueRules(strings.NewReader("[MiSTer]\ncontroller_unique_mapping=garbage"), "MENU"); err == nil {
		t.Fatal("invalid rule accepted")
	}
	if _, err := readUniqueRules(strings.NewReader(strings.Repeat("; padding\n", 20000)), "MENU"); err == nil {
		t.Fatal("unbounded INI accepted")
	}
}

func TestVideoDependentIdentityRulesFallBack(t *testing.T) {
	for _, text := range []string{
		"[MiSTer]\ncontroller_unique_mapping=0\n[video=1280x720]\ncontroller_unique_mapping=1\n",
		"[VIDEO=640x480@60]\nCONTROLLER_UNIQUE_MAPPING=0x045e028e\n",
		"[OtherCore]\n+video=640x480\ncontroller_unique_mapping=1\n",
		"[video=1280x720]\n+OtherCore\ncontroller_unique_mapping=1\n",
		"[video=1280x720]\ncontroller_unique_mapping=garbage\n",
	} {
		if rules, err := readUniqueRules(strings.NewReader(text), "MENU"); !errors.Is(err, ErrVideoMapping) || rules != nil {
			t.Fatalf("video rule was guessed: rules=%v err=%v for %q", rules, err, text)
		}
	}
}

func TestVideoSettingsWithoutConditionalIdentityRemainUsable(t *testing.T) {
	for _, tc := range []struct {
		text string
		want []uint32
	}{
		{"[MiSTer]\ncontroller_unique_mapping=1\n[video=1280x720]\nvideo_mode=8\nvga_scaler=1\n", []uint32{1}},
		{"[video=1280x720]\n; controller_unique_mapping=1\n[Menu]\ncontroller_unique_mapping=0\n", []uint32{0}},
		// +Menu makes the group unconditional for this application, regardless
		// of whether the video condition matched first in MiSTer's parser.
		{"[video=1280x720]\n+Menu\ncontroller_unique_mapping=1\n", []uint32{1}},
		{"[Menu]\n+video=1280x720\ncontroller_unique_mapping=1\n", []uint32{1}},
		{"[video=1280x720]\nvideo_mode=8\n[OtherCore]\ncontroller_unique_mapping=1\n", nil},
	} {
		got, err := readUniqueRules(strings.NewReader(tc.text), "MENU")
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("rules=%v err=%v, want %v for %q", got, err, tc.want, tc.text)
		}
	}
}
