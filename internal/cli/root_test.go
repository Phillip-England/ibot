package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/phillip-england/ibot/internal/app"
	"github.com/phillip-england/ibot/internal/capture"
	"github.com/phillip-england/ibot/internal/model"
	"github.com/phillip-england/ibot/internal/project"
)

type fakeCapture struct{}

func (fakeCapture) Point(_ context.Context, prompt capture.Prompt) (model.Position, error) {
	prompt("point prompt")
	return model.Position{X: 10, Y: 20}, nil
}

func (fakeCapture) Corners(_ context.Context, prompt capture.Prompt) ([4]model.Position, error) {
	for _, label := range []string{"top-left", "top-right", "bottom-right", "bottom-left"} {
		prompt(label)
	}
	return [4]model.Position{{X: 10, Y: 20}, {X: 110, Y: 20}, {X: 110, Y: 80}, {X: 10, Y: 80}}, nil
}

func (fakeCapture) PNG(context.Context, [4]model.Position) ([]byte, error) {
	return []byte("png bytes"), nil
}

type fakeClipboard struct{ value string }

func (clipboard *fakeClipboard) WriteAll(value string) error {
	clipboard.value = value
	return nil
}

func TestClickCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	clipboard := &fakeClipboard{}
	root := NewRoot(app.Service{Capture: fakeCapture{}}, clipboard, &stdout, &stderr, nil)
	root.SetArgs([]string{"click", "click_submit", "--vary=5", "--delay=3-5", "--hold=shift+control+cmd"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"def click_submit(variation=5, delay=(3, 5)", "random.randint(10 - variation, 10 + variation)", `("shift", "ctrl", "command")`} {
		if !strings.Contains(stdout.String(), expected) {
			t.Errorf("stdout missing %q", expected)
		}
	}
	if clipboard.value != stdout.String() {
		t.Fatal("clipboard did not receive generated source")
	}
}

func TestBoxGridCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	root := NewRoot(app.Service{Capture: fakeCapture{}}, &fakeClipboard{}, &stdout, &stderr, nil)
	root.SetArgs([]string{
		"box", "click_top_right", "--grid-rows=4", "--grid-columns=4", "--grid-cell=3", "--delay=2",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"grid=(4, 4, 3)", "delay=2", "row, column = divmod(cell, columns)"} {
		if !strings.Contains(stdout.String(), expected) {
			t.Errorf("stdout missing %q", expected)
		}
	}
}

func TestMaximalImageCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	clipboard := &fakeClipboard{}
	root := NewRoot(app.Service{Capture: fakeCapture{}}, clipboard, &stdout, &stderr, nil)
	root.SetArgs([]string{
		"click_image", "--confidence=0.85", "--wait-for", "--timeout=20",
		"--stall=1-2", "--all", "--order=random", "--gap=0.5-3.8",
		"--vary=all", "--hold=shift+control+cmd", "--no-imports", "click_targets",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"def click_targets", "locateAll", "random.shuffle(matches)",
		`delay=(1, 2)`, `gap=(0.5, 3.8)`, "random.uniform(*delay_range)",
		`("shift", "ctrl", "command")`, "random.randint(left, right)",
	} {
		if !strings.Contains(stdout.String(), expected) {
			t.Errorf("stdout missing %q", expected)
		}
	}
	if strings.HasPrefix(stdout.String(), "import ") {
		t.Error("--no-imports output contains imports")
	}
	if strings.Count(stderr.String(), "top-")+strings.Count(stderr.String(), "bottom-") != 4 {
		t.Fatalf("expected four corner prompts, got %q", stderr.String())
	}
}

func TestWaitUntilImageIsGoneCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	root := NewRoot(app.Service{Capture: fakeCapture{}}, &fakeClipboard{}, &stdout, &stderr, nil)
	root.SetArgs([]string{"click_image", "wait_for_spinner", "--wait-until-gone", "--timeout=12"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"def wait_for_spinner", "wait_until_gone=True", "timeout=12", "while matches:"} {
		if !strings.Contains(stdout.String(), expected) {
			t.Errorf("stdout missing %q", expected)
		}
	}
}

func TestWaitForImageWithoutClickingCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	root := NewRoot(app.Service{Capture: fakeCapture{}}, &fakeClipboard{}, &stdout, &stderr, nil)
	root.SetArgs([]string{"click_image", "wait_for_dialog", "--wait-for", "--no-click", "--timeout=12"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"def wait_for_dialog", "wait_for=True", "click=False", "timeout=12", "while not matches:"} {
		if !strings.Contains(stdout.String(), expected) {
			t.Errorf("stdout missing %q", expected)
		}
	}
}

func TestRejectsGapWithoutAllBeforeCapture(t *testing.T) {
	var stdout, stderr bytes.Buffer
	root := NewRoot(app.Service{Capture: fakeCapture{}}, &fakeClipboard{}, &stdout, &stderr, nil)
	root.SetArgs([]string{"click_image", "click_icon", "--gap=1"})
	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "require --all") {
		t.Fatalf("expected dependency error, got %v", err)
	}
}

func TestServeExportOption(t *testing.T) {
	for _, args := range [][]string{
		{"serve", "export=./helpers"},
		{"serve", "--export=./helpers"},
	} {
		var received ServeOptions
		root := NewRoot(app.Service{}, nil, &bytes.Buffer{}, &bytes.Buffer{}, func(_ context.Context, options ServeOptions) error {
			received = options
			return nil
		})
		root.SetArgs(args)
		if err := root.Execute(); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if received.Export != "./helpers" {
			t.Errorf("%v: export = %q", args, received.Export)
		}
	}
}

func TestClickCanSaveJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "points", "submit.json")
	var stdout, stderr bytes.Buffer
	clipboard := &fakeClipboard{}
	root := NewRoot(app.Service{Capture: fakeCapture{}}, clipboard, &stdout, &stderr, nil)
	root.SetArgs([]string{"click", "--save-json", path})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if stdout.Len() != 0 || clipboard.value != "" || !strings.Contains(string(data), `"x": 10`) {
		t.Fatalf("stdout=%q clipboard=%q JSON=%s", stdout.String(), clipboard.value, data)
	}
}

func TestServeTargetDirectories(t *testing.T) {
	var received ServeOptions
	root := NewRoot(app.Service{}, nil, &bytes.Buffer{}, &bytes.Buffer{}, func(_ context.Context, options ServeOptions) error { received = options; return nil })
	root.SetArgs([]string{"serve", "--points=./points", "--boxes=./boxes"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if received.Points != "./points" || received.Boxes != "./boxes" {
		t.Fatalf("options = %#v", received)
	}
}

func TestInitAndServeProject(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "automation")
	var stdout bytes.Buffer
	root := NewRoot(app.Service{}, nil, &stdout, &bytes.Buffer{}, nil)
	root.SetArgs([]string{"init", projectDir})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"__init__.py", "utilities.py", "points", "boxes", "images"} {
		if _, err := os.Stat(filepath.Join(projectDir, path)); err != nil {
			t.Fatalf("missing %s: %v", path, err)
		}
	}
	var received ServeOptions
	root = NewRoot(app.Service{}, nil, &bytes.Buffer{}, &bytes.Buffer{}, func(_ context.Context, options ServeOptions) error { received = options; return nil })
	root.SetArgs([]string{"serve", projectDir})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if received.Project != projectDir {
		t.Fatalf("project = %q", received.Project)
	}
}

func TestPatchUtilitiesCommand(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "automation")
	if _, err := project.Init(projectDir); err != nil {
		t.Fatal(err)
	}
	utilitiesPath := filepath.Join(projectDir, "utilities.py")
	if err := os.WriteFile(utilitiesPath, []byte("# stale\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(utilitiesPath, 0o600); err != nil {
		t.Fatal(err)
	}
	customPath := filepath.Join(projectDir, "custom.py")
	if err := os.WriteFile(customPath, []byte("custom = True\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	root := NewRoot(app.Service{}, nil, &stdout, &bytes.Buffer{}, nil)
	root.SetArgs([]string{"patch-utilities", projectDir})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	utilities, err := os.ReadFile(utilitiesPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(utilities), "def print_and_do(message, action):") {
		t.Fatal("patched utilities are missing print_and_do")
	}
	for _, expected := range []string{
		"def roll(chance, out_of=100):",
		"def click_saved_point(*path, variation=0, delay=None):",
		"def click_saved_box(*path, variation=0, delay=None):",
		"os.path.join(*path)",
		`variation.lower() == "all"`,
	} {
		if !strings.Contains(string(utilities), expected) {
			t.Fatalf("patched utilities are missing %q", expected)
		}
	}
	info, err := os.Stat(utilitiesPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("utilities permissions = %o", info.Mode().Perm())
	}
	custom, err := os.ReadFile(customPath)
	if err != nil || string(custom) != "custom = True\n" {
		t.Fatalf("custom file changed: contents=%q err=%v", custom, err)
	}
	if !strings.Contains(stdout.String(), utilitiesPath) {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestServeRejectsInvalidPositionalOption(t *testing.T) {
	root := NewRoot(app.Service{}, nil, &bytes.Buffer{}, &bytes.Buffer{}, func(context.Context, ServeOptions) error { return nil })
	root.SetArgs([]string{"serve", "./helpers"})
	if err := root.Execute(); err != nil {
		t.Fatalf("project positional argument rejected: %v", err)
	}
}
