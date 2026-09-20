package media

import (
	"testing"
	"time"
)

func TestCurrentNextBoundariesAndMalformedPrograms(t *testing.T) {
	now := time.Unix(1000, 0)
	programs := []Program{
		{Title: "Next", Start: now.Add(time.Hour), End: now.Add(2 * time.Hour)},
		{Title: "Ended", Start: now.Add(-time.Hour), End: now},
		{Title: "Current", Start: now, End: now.Add(time.Hour)},
		{Title: "Invalid", Start: now, End: now.Add(-time.Second)},
		{Title: "Overlapping", Start: now.Add(30 * time.Minute), End: now.Add(90 * time.Minute)},
	}
	current, next := CurrentNext(programs, now)
	if current == nil || current.Title != "Current" || next == nil || next.Title != "Next" {
		t.Fatalf("got %v / %v", current, next)
	}
	current, next = CurrentNext(programs, now.Add(time.Hour))
	if current == nil || current.Title != "Next" || next != nil {
		t.Fatalf("boundary: %v / %v", current, next)
	}
	current, next = CurrentNext(programs, now.Add(3*time.Hour))
	if current != nil || next != nil {
		t.Fatal("expired listings remained current")
	}
}
func TestGuideRequestBounds(t *testing.T) {
	now := time.Now()
	for _, window := range []time.Duration{0, -time.Second, 25 * time.Hour} {
		if ValidateGuideRequest([]string{"channel"}, now, now.Add(window)) == nil {
			t.Fatal("accepted invalid window")
		}
	}
	if ValidateGuideRequest(nil, now, now.Add(time.Hour)) == nil {
		t.Fatal("accepted unbounded channel query")
	}
}
