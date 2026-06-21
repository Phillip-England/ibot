package capture

import (
	"context"
	"errors"
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/phillip-england/ibot/internal/model"
)

func TestResizeAndCornerBounds(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 4, 4))
	source.Set(0, 0, color.RGBA{R: 255, A: 255})
	normalized := resize(source, 2, 2)
	if normalized.Bounds().Dx() != 2 || normalized.Bounds().Dy() != 2 {
		t.Fatalf("normalized bounds = %v", normalized.Bounds())
	}
	corners := [4]model.Position{
		{X: 10, Y: 20}, {X: 110, Y: 18}, {X: 108, Y: 80}, {X: 8, Y: 78},
	}
	left, right, top, bottom := cornerBounds(corners)
	if left != 8 || right != 110 || top != 18 || bottom != 80 {
		t.Fatalf("bounds = %d %d %d %d", left, right, top, bottom)
	}
}

func TestClamp(t *testing.T) {
	for _, test := range []struct{ value, want int }{{-2, 0}, {5, 5}, {12, 10}} {
		if got := clamp(test.value, 0, 10); got != test.want {
			t.Errorf("clamp(%d) = %d, want %d", test.value, got, test.want)
		}
	}
}

func TestWaitForScreenshot(t *testing.T) {
	started := time.Now()
	if err := waitForScreenshot(context.Background(), 10*time.Millisecond); err != nil {
		t.Fatalf("waitForScreenshot() error = %v", err)
	}
	if elapsed := time.Since(started); elapsed < 10*time.Millisecond {
		t.Fatalf("waitForScreenshot() returned after %v", elapsed)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitForScreenshot(ctx, time.Second); !errors.Is(err, context.Canceled) {
		t.Fatalf("waitForScreenshot() error = %v, want context.Canceled", err)
	}
}
