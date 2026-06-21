package generator

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/phillip-england/ibot/internal/model"
)

var pngData = []byte("valid-enough-for-generation")

func TestPointVariationAndHeldKeys(t *testing.T) {
	source, err := Point(model.PointOptions{
		Name: "click_submit", Position: model.Position{X: 5, Y: 5},
		Variation: model.Variation{Pixels: 5}, Hold: []string{"shift", "ctrl"},
		IncludeImports: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"import random", "random.randint(0, 10)", "pyautogui.keyDown(key)",
		"for key in reversed(held_keys)", "pyautogui.keyUp(key)",
	} {
		if !strings.Contains(source, expected) {
			t.Errorf("generated point source missing %q", expected)
		}
	}
	compilePython(t, source)
}

func TestBoxModes(t *testing.T) {
	corners := [4]model.Position{{X: 10, Y: 20}, {X: 110, Y: 20}, {X: 110, Y: 80}, {X: 10, Y: 80}}
	center, err := Box(model.BoxOptions{Name: "click_box", Corners: corners, IncludeImports: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(center, "pyautogui.click(x=60, y=50)") {
		t.Fatalf("unexpected center source:\n%s", center)
	}
	all, err := Box(model.BoxOptions{Name: "click_box", Corners: corners, Variation: model.Variation{All: true}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(all, "random.randint(10, 110)") || !strings.Contains(all, "random.randint(20, 80)") {
		t.Fatalf("unexpected all source:\n%s", all)
	}
}

func TestMaximalImageCombination(t *testing.T) {
	source, err := Image(model.ImageOptions{
		Name: "click_targets", PNG: pngData, Variation: model.Variation{All: true},
		Confidence: 0.85, WaitFor: true, Timeout: 20,
		Stall: &model.SecondsRange{Min: 1, Max: 2}, ClickAll: true, Order: "random",
		Gap: &model.SecondsRange{Min: 0.5, Max: 3.8}, Hold: []string{"shift", "ctrl", "command"},
		IncludeImports: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"import random", "import time", "locateAll", "deadline = time.monotonic() + 20",
		"random.shuffle(matches)", "random.uniform(1, 2)", "random.uniform(0.5, 3.8)",
		"random.randint(left, right)", `("shift", "ctrl", "command")`, "finally:",
	} {
		if !strings.Contains(source, expected) {
			t.Errorf("generated image source missing %q", expected)
		}
	}
	compilePython(t, source)
}

func TestMeaningfulImageOptionMatrix(t *testing.T) {
	variations := []model.Variation{{}, {Pixels: 3}, {All: true}}
	stalls := []*model.SecondsRange{nil, {Min: 1, Max: 1}, {Min: 1, Max: 2}}
	holds := [][]string{nil, {"shift", "ctrl"}}
	type matchMode struct {
		all   bool
		order string
		gap   *model.SecondsRange
	}
	modes := []matchMode{{order: "linear"}}
	for _, order := range []string{"linear", "backwards", "random"} {
		for _, gap := range []*model.SecondsRange{nil, {Min: 1, Max: 1}, {Min: 0.5, Max: 3.8}} {
			modes = append(modes, matchMode{all: true, order: order, gap: gap})
		}
	}
	count := 0
	for _, variation := range variations {
		for _, wait := range []bool{false, true} {
			for _, stall := range stalls {
				for _, hold := range holds {
					for _, imports := range []bool{false, true} {
						for _, mode := range modes {
							_, err := Image(model.ImageOptions{
								Name: "click_icon", PNG: pngData, Variation: variation,
								Confidence: 0.8, WaitFor: wait, Timeout: 20, Stall: stall,
								ClickAll: mode.all, Order: mode.order, Gap: mode.gap,
								Hold: hold, IncludeImports: imports,
							})
							if err != nil {
								t.Fatalf("combination %d: %v", count, err)
							}
							count++
						}
					}
				}
			}
		}
	}
	if count != 720 {
		t.Fatalf("tested %d combinations, want 720", count)
	}
}

func TestRejectsInvalidImageDependencies(t *testing.T) {
	_, err := Image(model.ImageOptions{Name: "click_icon", PNG: pngData, Order: "backwards"})
	if err == nil || !strings.Contains(err.Error(), "require all") {
		t.Fatalf("expected all dependency error, got %v", err)
	}
}

func compilePython(t *testing.T, source string) {
	t.Helper()
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 unavailable for generated-source syntax check")
	}
	command := exec.Command(python, "-c", "import sys; compile(sys.stdin.read(), '<generated>', 'exec')")
	command.Stdin = strings.NewReader(source)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generated Python failed to compile: %v\n%s\n%s", err, output, source)
	}
}
