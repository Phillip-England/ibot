package model

import "testing"

func TestParseOptions(t *testing.T) {
	variation, err := ParseVariation("all")
	if err != nil || !variation.All {
		t.Fatalf("ParseVariation(all) = %#v, %v", variation, err)
	}
	duration, err := ParseSecondsRange("0.5-3.8")
	if err != nil || duration.Min != 0.5 || duration.Max != 3.8 {
		t.Fatalf("ParseSecondsRange = %#v, %v", duration, err)
	}
	keys, err := ParseHold("shift+control+cmd+shift")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"shift", "ctrl", "command"}
	for index := range want {
		if keys[index] != want[index] {
			t.Fatalf("ParseHold = %#v, want %#v", keys, want)
		}
	}
}

func TestRejectsInvalidRanges(t *testing.T) {
	for _, value := range []string{"-1", "2-1", "nan", "1-inf", "1-2-3"} {
		if _, err := ParseSecondsRange(value); err == nil {
			t.Errorf("ParseSecondsRange(%q) unexpectedly succeeded", value)
		}
	}
}

func TestGridTargetUsesZeroBasedCellIndex(t *testing.T) {
	grid, err := NewGridTarget(4, 4, 3)
	if err != nil {
		t.Fatal(err)
	}
	if grid.Rows != 4 || grid.Columns != 4 || grid.Cell != 3 {
		t.Fatalf("NewGridTarget = %#v", grid)
	}
	for _, cell := range []int{-1, 16} {
		if _, err := NewGridTarget(4, 4, cell); err == nil {
			t.Errorf("NewGridTarget accepted cell %d", cell)
		}
	}
}
