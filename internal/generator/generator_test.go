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
		"import random", "def click_submit(variation=5, delay=None", "random.randint(5 - variation, 5 + variation)", "pyautogui.keyDown(key)",
		"for key in reversed(held_keys)", "pyautogui.keyUp(key)",
	} {
		if !strings.Contains(source, expected) {
			t.Errorf("generated point source missing %q", expected)
		}
	}
	compilePython(t, source)
}

func TestPointRuntimeDelayAndHoldOverride(t *testing.T) {
	source, err := Point(model.PointOptions{
		Name: "click_submit", Position: model.Position{X: 10, Y: 20},
	})
	if err != nil {
		t.Fatal(err)
	}
	output := runPython(t, `
import random
events = []
class FakePyAutoGUI:
    KEYBOARD_KEYS = ("shift",)
    def keyDown(self, key): events.append(("down", key))
    def keyUp(self, key): events.append(("up", key))
    def moveTo(self, **point): events.append(("move", point["x"], point["y"]))
    def click(self, **point): events.append(("click", point))
pyautogui = FakePyAutoGUI()
class FakeTime:
    def sleep(self, seconds): events.append(("sleep", seconds))
time = FakeTime()
`+source+`
click_submit(delay=0.25, hold="shift")
print(events)
`)
	for _, expected := range []string{"('down', 'shift')", "('move', 10, 20)", "('sleep', 0.25)", "('click', {})", "('up', 'shift')"} {
		if !strings.Contains(output, expected) {
			t.Errorf("runtime output missing %q: %s", expected, output)
		}
	}
}

func TestGeneratedPointAndBoxDelayDefaults(t *testing.T) {
	delay := &model.SecondsRange{Min: 3, Max: 5}
	point, err := Point(model.PointOptions{Name: "click_point", Delay: delay})
	if err != nil {
		t.Fatal(err)
	}
	box, err := Box(model.BoxOptions{Name: "click_box", Delay: delay})
	if err != nil {
		t.Fatal(err)
	}
	for name, source := range map[string]string{"point": point, "box": box} {
		if !strings.Contains(source, "delay=(3, 5)") {
			t.Errorf("generated %s source does not set the delay default", name)
		}
	}
}

func TestBoxRuntimeGridOverride(t *testing.T) {
	source, err := Box(model.BoxOptions{
		Name:    "click_box",
		Corners: [4]model.Position{{X: 10, Y: 20}, {X: 110, Y: 20}, {X: 110, Y: 80}, {X: 10, Y: 80}},
	})
	if err != nil {
		t.Fatal(err)
	}
	output := runPython(t, `
import random
class FakePyAutoGUI:
    KEYBOARD_KEYS = ()
    def click(self, **point): print(point["x"], point["y"])
pyautogui = FakePyAutoGUI()
`+source+`
click_box(grid=(4, 4, 3))
`)
	if strings.TrimSpace(output) != "97 27" {
		t.Fatalf("runtime grid click = %q, want 97 27", output)
	}
}

func TestBoxModes(t *testing.T) {
	corners := [4]model.Position{{X: 10, Y: 20}, {X: 110, Y: 20}, {X: 110, Y: 80}, {X: 10, Y: 80}}
	center, err := Box(model.BoxOptions{Name: "click_box", Corners: corners, IncludeImports: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(center, "def click_box(variation=0, grid=None, delay=None, hold=())") || !strings.Contains(center, "left, right, top, bottom = 10, 110, 20, 80") {
		t.Fatalf("unexpected center source:\n%s", center)
	}
	all, err := Box(model.BoxOptions{Name: "click_box", Corners: corners, Variation: model.Variation{All: true}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(all, `variation="all"`) || !strings.Contains(all, "random.randint(left, right)") {
		t.Fatalf("unexpected all source:\n%s", all)
	}
}

func TestBoxGridTargetsCellsInRowMajorOrder(t *testing.T) {
	corners := [4]model.Position{{X: 10, Y: 20}, {X: 110, Y: 20}, {X: 110, Y: 80}, {X: 10, Y: 80}}
	grid, err := model.NewGridTarget(4, 4, 3)
	if err != nil {
		t.Fatal(err)
	}
	source, err := Box(model.BoxOptions{Name: "click_top_right", Corners: corners, Grid: grid, IncludeImports: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"grid=(4, 4, 3)", "row, column = divmod(cell, columns)"} {
		if !strings.Contains(source, expected) {
			t.Errorf("generated grid source missing %q:\n%s", expected, source)
		}
	}
	compilePython(t, source)
}

func TestBoxGridVariationStaysInsideSelectedCell(t *testing.T) {
	corners := [4]model.Position{{X: 10, Y: 20}, {X: 110, Y: 20}, {X: 110, Y: 80}, {X: 10, Y: 80}}
	grid, _ := model.NewGridTarget(4, 4, 4)
	source, err := Box(model.BoxOptions{
		Name: "click_cell", Corners: corners, Grid: grid,
		Variation: model.Variation{All: true}, IncludeImports: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`variation="all"`, "grid=(4, 4, 4)", "random.randint(left, right)"} {
		if !strings.Contains(source, expected) {
			t.Errorf("generated grid source missing %q:\n%s", expected, source)
		}
	}
}

func TestBoxGridRejectsCellsLargerThanCapturedPixels(t *testing.T) {
	grid, _ := model.NewGridTarget(20, 20, 0)
	_, err := Box(model.BoxOptions{
		Name: "click_cell", Corners: [4]model.Position{{X: 0, Y: 0}, {X: 9, Y: 0}, {X: 9, Y: 9}, {X: 0, Y: 9}}, Grid: grid,
	})
	if err == nil || !strings.Contains(err.Error(), "pixels") {
		t.Fatalf("expected grid dimensions error, got %v", err)
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
		"import random", "import time", "locateAll", "timeout=20",
		"random.shuffle(matches)", `delay=(1, 2)`, `gap=(0.5, 3.8)`, "random.uniform(*delay_range)",
		"random.randint(left, right)", `("shift", "ctrl", "command")`, "finally:",
		`.convert("RGB")`, "Image.Resampling.LANCZOS", "Image.Resampling.BICUBIC",
		"Image.Resampling.BILINEAR", "if matches:",
	} {
		if !strings.Contains(source, expected) {
			t.Errorf("generated image source missing %q", expected)
		}
	}
	for _, expected := range []string{
		`position="center"`, `"bottom_right": (1.0, 1.0)`,
		"position ratios must be numbers from 0 through 1", "anchor_x = round(",
	} {
		if !strings.Contains(source, expected) {
			t.Errorf("generated image runtime contract missing %q", expected)
		}
	}
	compilePython(t, source)
}

func TestImageDelayMovesBeforeWaitingAndClicking(t *testing.T) {
	source, err := Image(model.ImageOptions{Name: "click_image", PNG: pngData, Stall: &model.SecondsRange{Min: 3, Max: 5}})
	if err != nil {
		t.Fatal(err)
	}
	loop := strings.Index(source, "for index, match in enumerate(matches):")
	if loop < 0 {
		t.Fatalf("generated image source has no match loop:\n%s", source)
	}
	move := strings.Index(source[loop:], "pyautogui.moveTo(x=click_x, y=click_y)")
	wait := strings.Index(source[loop:], "time.sleep(random.uniform(*delay_range))")
	click := strings.Index(source[loop:], "pyautogui.click()")
	if move < 0 || wait < move || click < wait {
		t.Fatalf("image delay does not move, wait, then click:\n%s", source)
	}
}

func TestImageMatchingUsesOriginalScreenshotAtLogicalResolution(t *testing.T) {
	source, err := Image(model.ImageOptions{
		Name: "click_icon", PNG: pngData, Confidence: 0.9, IncludeImports: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"screens = [screenshot]",
		"if screenshot.size != screen_size:",
		"for screen in screens:",
		"return matches",
		"return []",
	} {
		if !strings.Contains(source, expected) {
			t.Errorf("generated image source missing %q", expected)
		}
	}
	compilePython(t, source)
}

func TestImageWaitUntilGone(t *testing.T) {
	source, err := Image(model.ImageOptions{
		Name: "wait_for_spinner", PNG: pngData, Confidence: 0.85,
		WaitUntilGone: true, Timeout: 12, IncludeImports: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"import time", "wait_until_gone=True", "timeout=12",
		"deadline = time.monotonic() + timeout", "while matches:",
		"embedded image was still visible", "    return",
	} {
		if !strings.Contains(source, expected) {
			t.Errorf("generated wait-until-gone source missing %q", expected)
		}
	}
	compilePython(t, source)
}

func TestImageWaitForWithoutClicking(t *testing.T) {
	source, err := Image(model.ImageOptions{
		Name: "wait_for_dialog", PNG: pngData, Confidence: 0.85,
		WaitFor: true, NoClick: true, Timeout: 12, IncludeImports: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"import time", "wait_for=True", "click=False", "timeout=12",
		"deadline = time.monotonic() + timeout", "while not matches:", "    return",
	} {
		if !strings.Contains(source, expected) {
			t.Errorf("generated no-click wait source missing %q", expected)
		}
	}
	compilePython(t, source)
}

func TestImageNoClickRequiresWaitFor(t *testing.T) {
	_, err := Image(model.ImageOptions{Name: "wait_for_dialog", PNG: pngData, NoClick: true})
	if err == nil || !strings.Contains(err.Error(), "requires wait for") {
		t.Fatalf("expected no-click dependency error, got %v", err)
	}
}

func TestImageExistsReturnsBooleanWithoutClicking(t *testing.T) {
	source, err := Image(model.ImageOptions{
		Name: "image_exists", PNG: pngData, Confidence: 0.92,
		ReturnExists: true, IncludeImports: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"def image_exists(confidence=0.92) -> bool:",
		"Return whether the captured image is visible at runtime confidence.",
		"return True", "return False",
	} {
		if !strings.Contains(source, expected) {
			t.Errorf("generated existence source missing %q", expected)
		}
	}
	for _, unexpected := range []string{"pyautogui.click(", "raise RuntimeError", "import time"} {
		if strings.Contains(source, unexpected) {
			t.Errorf("generated existence source unexpectedly contains %q", unexpected)
		}
	}
	compilePython(t, source)
}

func TestImageExistsRejectsClickOptions(t *testing.T) {
	_, err := Image(model.ImageOptions{
		Name: "image_exists", PNG: pngData, ReturnExists: true,
		Variation: model.Variation{Pixels: 2},
	})
	if err == nil || !strings.Contains(err.Error(), "existence checks") {
		t.Fatalf("expected existence option error, got %v", err)
	}
}

func TestWaitUntilGoneRejectsClickOptions(t *testing.T) {
	_, err := Image(model.ImageOptions{
		Name: "wait_for_spinner", PNG: pngData, WaitUntilGone: true,
		Variation: model.Variation{Pixels: 2},
	})
	if err == nil || !strings.Contains(err.Error(), "click options") {
		t.Fatalf("expected click option error, got %v", err)
	}
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

func runPython(t *testing.T, source string) string {
	t.Helper()
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 unavailable for generated-source runtime check")
	}
	command := exec.Command(python, "-")
	command.Stdin = strings.NewReader(source)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("generated Python failed: %v\n%s\n%s", err, output, source)
	}
	return string(output)
}
