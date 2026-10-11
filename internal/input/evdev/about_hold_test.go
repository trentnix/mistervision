package evdev

import (
	"testing"
	"time"

	"mistervision/internal/input/control"
)

func TestAboutTapAndHold(t *testing.T) {
	var h aboutHold
	start := time.Unix(100, 0)
	check := func(ms int, held, pressed bool, want control.Action) {
		t.Helper()
		if got := h.update(held, pressed, start.Add(time.Duration(ms)*time.Millisecond)); got != want {
			t.Fatalf("at %d ms: got %q, want %q", ms, got, want)
		}
	}
	check(0, true, true, "")
	check(150, false, false, control.About)
	check(200, false, false, "")
	check(500, true, true, "")
	check(2499, true, false, "")
	check(2500, true, false, control.ConfirmExit)
	check(6000, true, false, "")
	check(6500, false, false, "")
	check(7000, false, true, control.About) // Tap completed within one poll.
	check(7100, true, true, "")
	check(9100, false, false, control.ConfirmExit)
	check(9200, false, false, "")
}

func TestAboutHoldIsPerDevice(t *testing.T) {
	start := time.Unix(100, 0)
	var first, second aboutHold
	first.update(true, true, start)
	second.update(true, true, start.Add(2*time.Second))
	if got := second.update(false, false, start.Add(2500*time.Millisecond)); got != control.About {
		t.Fatal("second device inherited another device's hold", got)
	}
	// Disconnect discards the owning device instead of manufacturing a release.
	first = aboutHold{}
	if got := first.update(false, false, start.Add(4*time.Second)); got != "" {
		t.Fatal("disconnect produced an action", got)
	}
}
