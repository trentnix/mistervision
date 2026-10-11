package media

import (
	"reflect"
	"testing"
)

func TestValidSegments(t *testing.T) {
	input := []Segment{
		{SegmentCredits, 80, 110}, {SegmentIntro, 10, 30}, {SegmentIntro, 10, 30},
		{SegmentRecap, 0, 20}, {SegmentCommercial, 15, 40},
		{SegmentIntro, -1, 10}, {SegmentIntro, 20, 20}, {SegmentIntro, 30, 20},
		{SegmentIntro, 101, 120}, {"unknown", 0, 5},
	}
	original := append([]Segment(nil), input...)
	got := ValidSegments(input, 100)
	want := []Segment{{SegmentRecap, 0, 20}, {SegmentIntro, 10, 30}, {SegmentCommercial, 15, 40}, {SegmentCredits, 80, 100}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("segments = %+v", got)
	}
	if !reflect.DeepEqual(input, original) {
		t.Fatal("mutated provider data")
	}
	if got := ValidSegments([]Segment{{SegmentIntro, 0, 10}}, 0); len(got) != 1 {
		t.Fatal("unknown duration discarded valid segment")
	}
}
