package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/phillip-england/ibot/internal/app"
	"github.com/phillip-england/ibot/internal/capture"
	"github.com/phillip-england/ibot/internal/model"
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
	root.SetArgs([]string{"click", "click_submit", "--vary=5", "--hold=shift+control+cmd"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"def click_submit", "random.randint(5, 15)", `("shift", "ctrl", "command")`} {
		if !strings.Contains(stdout.String(), expected) {
			t.Errorf("stdout missing %q", expected)
		}
	}
	if clipboard.value != stdout.String() {
		t.Fatal("clipboard did not receive generated source")
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
		"random.uniform(1, 2)", "random.uniform(0.5, 3.8)",
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

func TestRejectsGapWithoutAllBeforeCapture(t *testing.T) {
	var stdout, stderr bytes.Buffer
	root := NewRoot(app.Service{Capture: fakeCapture{}}, &fakeClipboard{}, &stdout, &stderr, nil)
	root.SetArgs([]string{"click_image", "click_icon", "--gap=1"})
	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "require --all") {
		t.Fatalf("expected dependency error, got %v", err)
	}
}
