package capture

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"sync"
	"time"

	"github.com/go-vgo/robotgo"
	"github.com/phillip-england/ibot/internal/model"
	"github.com/phillip-england/ibot/internal/sound"
	hook "github.com/robotn/gohook"
	"golang.org/x/image/draw"
)

type Prompt func(message string)

type Service interface {
	Point(ctx context.Context, prompt Prompt) (model.Position, error)
	Corners(ctx context.Context, prompt Prompt) ([4]model.Position, error)
	PNG(ctx context.Context, corners [4]model.Position) ([]byte, error)
}

type Desktop struct {
	mu sync.Mutex
}

const screenshotDelay = time.Second

func NewDesktop() *Desktop { return &Desktop{} }

func (desktop *Desktop) Point(ctx context.Context, prompt Prompt) (model.Position, error) {
	desktop.mu.Lock()
	defer desktop.mu.Unlock()
	return waitPoint(ctx, prompt, "Move the pointer to the target position, then press 0.")
}

func (desktop *Desktop) Corners(ctx context.Context, prompt Prompt) ([4]model.Position, error) {
	desktop.mu.Lock()
	defer desktop.mu.Unlock()
	labels := []string{"top-left", "top-right", "bottom-right", "bottom-left"}
	var corners [4]model.Position
	for index, label := range labels {
		position, err := waitPoint(ctx, prompt, fmt.Sprintf("Move the pointer to the %s corner, then press 0.", label))
		if err != nil {
			return corners, err
		}
		corners[index] = position
	}
	return corners, nil
}

func waitPoint(ctx context.Context, prompt Prompt, message string) (model.Position, error) {
	if err := ctx.Err(); err != nil {
		return model.Position{}, err
	}
	if prompt != nil {
		prompt(message)
	}
	if !hook.AddEvent("0") {
		return model.Position{}, fmt.Errorf("global keyboard listener stopped before 0 was captured")
	}
	sound.Blip()
	if err := ctx.Err(); err != nil {
		return model.Position{}, err
	}
	x, y := robotgo.Location()
	return model.Position{X: x, Y: y}, nil
}

func (desktop *Desktop) PNG(ctx context.Context, corners [4]model.Position) ([]byte, error) {
	if err := waitForScreenshot(ctx, screenshotDelay); err != nil {
		return nil, err
	}
	screenshot, err := robotgo.CaptureImg()
	if err != nil {
		return nil, fmt.Errorf("capture screen: %w", err)
	}
	sound.Done()
	logicalWidth, logicalHeight := robotgo.GetScreenSize()
	if logicalWidth <= 0 || logicalHeight <= 0 {
		return nil, fmt.Errorf("capture screen: invalid logical screen size")
	}
	screenshot = resize(screenshot, logicalWidth, logicalHeight)
	left, right, top, bottom := cornerBounds(corners)
	left, right = clamp(left, 0, logicalWidth-1), clamp(right, 0, logicalWidth-1)
	top, bottom = clamp(top, 0, logicalHeight-1), clamp(bottom, 0, logicalHeight-1)
	if right < left || bottom < top {
		return nil, fmt.Errorf("captured image bounds are invalid")
	}
	region := image.Rect(0, 0, right-left+1, bottom-top+1)
	cropped := image.NewRGBA(region)
	draw.Draw(cropped, region, screenshot, image.Pt(left, top), draw.Src)
	var output bytes.Buffer
	if err := png.Encode(&output, cropped); err != nil {
		return nil, fmt.Errorf("encode captured PNG: %w", err)
	}
	return output.Bytes(), nil
}

func waitForScreenshot(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func resize(source image.Image, width, height int) image.Image {
	if source.Bounds().Dx() == width && source.Bounds().Dy() == height {
		return source
	}
	target := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(target, target.Bounds(), source, source.Bounds(), draw.Over, nil)
	return target
}

func cornerBounds(corners [4]model.Position) (left, right, top, bottom int) {
	left, right = corners[0].X, corners[0].X
	top, bottom = corners[0].Y, corners[0].Y
	for _, corner := range corners[1:] {
		left, right = min(left, corner.X), max(right, corner.X)
		top, bottom = min(top, corner.Y), max(bottom, corner.Y)
	}
	return
}

func clamp(value, minimum, maximum int) int {
	return min(max(value, minimum), maximum)
}
