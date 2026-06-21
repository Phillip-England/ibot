package app

import (
	"context"

	"github.com/phillip-england/ibot/internal/capture"
	"github.com/phillip-england/ibot/internal/generator"
	"github.com/phillip-england/ibot/internal/model"
)

type Service struct {
	Capture capture.Service
}

func (service Service) Point(ctx context.Context, options model.PointOptions, prompt capture.Prompt) (string, error) {
	position, err := service.Capture.Point(ctx, prompt)
	if err != nil {
		return "", err
	}
	options.Position = position
	return generator.Point(options)
}

func (service Service) Box(ctx context.Context, options model.BoxOptions, prompt capture.Prompt) (string, error) {
	corners, err := service.Capture.Corners(ctx, prompt)
	if err != nil {
		return "", err
	}
	options.Corners = corners
	return generator.Box(options)
}

func (service Service) Image(ctx context.Context, options model.ImageOptions, prompt capture.Prompt) (string, error) {
	corners, err := service.Capture.Corners(ctx, prompt)
	if err != nil {
		return "", err
	}
	data, err := service.Capture.PNG(ctx, corners)
	if err != nil {
		return "", err
	}
	options.PNG = data
	return generator.Image(options)
}
