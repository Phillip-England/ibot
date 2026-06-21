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
