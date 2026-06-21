package project

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitIncludesManagedUtilities(t *testing.T) {
	layout, err := Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(layout.Root, "utilities.py"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(contents)
	for _, expected := range []string{
		"def random_wait(min_seconds, max_seconds):",
		"wait_random = random_wait",
		"def wait_until(predicate, timeout=30, interval=0.25):",
		"def retry(action, attempts=3, delay=0):",
		"def roll(chance, out_of=100):",
		"def perform_random_actions(min_actions, max_actions, actions):",
		"amount = random_number(min_actions, max_actions)",
		"for action in random.sample(actions, amount):",
		"def click_saved_point(*path, variation=0, delay=None):",
		"def click_saved_box(*path, variation=0, delay=None):",
	} {
		if !strings.Contains(source, expected) {
			t.Errorf("utilities.py missing %q", expected)
		}
	}
}

func TestRollUsesChanceOutOfTotal(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is not installed")
	}
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "utilities.py"), []byte(UtilitiesSource), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "pyautogui.py"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	script := `import utilities

utilities.random.random = lambda: 0.009
assert utilities.roll(1) is True
utilities.random.random = lambda: 0.01
assert utilities.roll(1) is False
utilities.random.random = lambda: 0.49
assert utilities.roll(1, 2) is True
assert utilities.roll(0) is False
assert utilities.roll(100) is True

for args in [(-1,), (101,), (1, 0)]:
    try:
        utilities.roll(*args)
    except ValueError:
        pass
    else:
        raise AssertionError(f"invalid roll arguments were accepted: {args}")

try:
    utilities.roll(True)
except TypeError:
    pass
else:
    raise AssertionError("boolean chance was accepted")
`
	command := exec.Command(python, "-c", script)
	command.Dir = directory
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("roll failed: %v\n%s", err, output)
	}
}

func TestPerformRandomActionsSelectsWithinBounds(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is not installed")
	}
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "utilities.py"), []byte(UtilitiesSource), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "pyautogui.py"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	script := `import utilities

executed = []
actions = [lambda index=index: executed.append(index) for index in range(20)]
utilities.random_number = lambda minimum, maximum: maximum
utilities.perform_random_actions(0, 4, actions)
assert len(executed) == 4, executed
assert len(set(executed)) == 4, executed

utilities.random_number = lambda minimum, maximum: minimum
utilities.perform_random_actions(0, 4, actions)
assert len(executed) == 4, executed
`
	command := exec.Command(python, "-c", script)
	command.Dir = directory
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("perform_random_actions failed: %v\n%s", err, output)
	}
}

func TestSavedTargetClicksMoveWaitThenClick(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is not installed")
	}
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "utilities.py"), []byte(UtilitiesSource), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "pyautogui.py"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "point.json"), []byte(`{"x": 10, "y": 20}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "box.json"), []byte(`{"corners":[{"x":0,"y":0},{"x":10,"y":0},{"x":10,"y":20},{"x":0,"y":20}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	script := `import utilities

events = []
utilities.pyautogui.moveTo = lambda **position: events.append(("move", position["x"], position["y"]))
utilities.pyautogui.click = lambda: events.append(("click",))
utilities.random_wait = lambda minimum, maximum: events.append(("wait", minimum, maximum))

utilities.click_saved_point("point.json", delay=(0.5, 1))
assert events == [("move", 10, 20), ("wait", 0.5, 1), ("click",)], events

events.clear()
utilities.click_saved_box("box.json", delay=0.25)
assert events == [("move", 5, 10), ("wait", 0.25, 0.25), ("click",)], events

try:
    utilities.click_saved_point("point.json", delay=(1, 0.5))
except ValueError:
    pass
else:
    raise AssertionError("reversed delay range was accepted")
assert events == [("move", 5, 10), ("wait", 0.25, 0.25), ("click",)], events
`
	command := exec.Command(python, "-c", script)
	command.Dir = directory
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("saved target click failed: %v\n%s", err, output)
	}
}
