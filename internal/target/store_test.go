package target

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/phillip-england/ibot/internal/model"
)

func TestWriteAndListTargets(t *testing.T) {
	points, boxes := filepath.Join(t.TempDir(), "points"), filepath.Join(t.TempDir(), "boxes")
	if err := WritePoint(filepath.Join(points, "submit.json"), model.Position{X: 12, Y: 34}); err != nil {
		t.Fatal(err)
	}
	corners := [4]model.Position{{X: 1, Y: 2}, {X: 9, Y: 2}, {X: 9, Y: 8}, {X: 1, Y: 8}}
	if err := WriteBox(filepath.Join(boxes, "menu.json"), corners); err != nil {
		t.Fatal(err)
	}
	entries, err := List(points, boxes)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Kind != "box" || entries[1].Point.X != 12 {
		t.Fatalf("unexpected entries: %#v", entries)
	}
	data, _ := os.ReadFile(filepath.Join(points, "submit.json"))
	if !strings.Contains(string(data), "\n  \"x\": 12") || data[len(data)-1] != '\n' {
		t.Fatalf("point JSON = %q", data)
	}
}

func TestPathRejectsTraversal(t *testing.T) {
	for _, name := range []string{"../target.json", "target.txt", ".json"} {
		if _, err := Path(t.TempDir(), name); err == nil {
			t.Errorf("Path accepted %q", name)
		}
	}
}
