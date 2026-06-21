package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/atotto/clipboard"

	"github.com/phillip-england/ibot/internal/app"
	"github.com/phillip-england/ibot/internal/capture"
	"github.com/phillip-england/ibot/internal/cli"
	ibotweb "github.com/phillip-england/ibot/internal/web"
)

type systemClipboard struct{}

func (systemClipboard) WriteAll(text string) error { return clipboard.WriteAll(text) }

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	desktop := capture.NewDesktop()
	service := app.Service{Capture: desktop}
	serve := func(ctx context.Context, options cli.ServeOptions) error {
		webServer := ibotweb.Server{Service: service, ProjectDir: options.Project, ExportDir: options.Export, PointsDir: options.Points, BoxesDir: options.Boxes, ImagesDir: options.Images}
		return webServer.Serve(ctx, ibotweb.Options{
			Address: options.Address,
			Open:    options.Open,
			Logf: func(format string, args ...any) {
				fmt.Fprintf(os.Stderr, format+"\n", args...)
			},
		})
	}
	command := cli.NewRoot(service, systemClipboard{}, os.Stdout, os.Stderr, serve)
	if err := command.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "ibot: %v\n", err)
		os.Exit(1)
	}
}
