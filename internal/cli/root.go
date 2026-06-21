package cli

import (
	"context"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/spf13/cobra"

	"github.com/phillip-england/ibot/internal/app"
	"github.com/phillip-england/ibot/internal/generator"
	"github.com/phillip-england/ibot/internal/model"
)

type Clipboard interface {
	WriteAll(text string) error
}

type ServeOptions struct {
	Address string
	Open    bool
}

type ServeFunc func(ctx context.Context, options ServeOptions) error

func NewRoot(service app.Service, clipboard Clipboard, stdout, stderr io.Writer, serve ServeFunc) *cobra.Command {
	root := &cobra.Command{
		Use:           "ibot",
		Short:         "Generate portable PyAutoGUI functions",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       "0.2.0",
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.AddCommand(newClick(service, clipboard, stdout, stderr))
	root.AddCommand(newBox(service, clipboard, stdout, stderr))
	root.AddCommand(newClickImage(service, clipboard, stdout, stderr))
	if serve != nil {
		root.AddCommand(newServe(serve))
	}
	return root
}

func newClick(service app.Service, clipboard Clipboard, stdout, stderr io.Writer) *cobra.Command {
	var vary, hold string
	var noImports bool
	command := &cobra.Command{
		Use:   "click [function_name]",
		Short: "Capture a position and generate a click function",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			name := optionalName(args, "click_position")
			if err := generator.ValidateFunctionName(name); err != nil {
				return err
			}
			variation, err := model.ParseVariation(vary)
			if err != nil {
				return err
			}
			if variation.All {
				return fmt.Errorf("click --vary does not support all")
			}
			keys, err := model.ParseHold(hold)
			if err != nil {
				return err
			}
			source, err := service.Point(command.Context(), model.PointOptions{
				Name: name, Variation: variation, Hold: keys, IncludeImports: !noImports,
			}, prompt(stderr))
			if err != nil {
				return err
			}
			return deliver(source, name, clipboard, stdout, stderr)
		},
	}
	command.Flags().StringVar(&vary, "vary", "0", "vary each click axis by PIXELS")
	command.Flags().StringVar(&hold, "hold", "", "hold KEY[+KEY...] while clicking")
	command.Flags().BoolVar(&noImports, "no-imports", false, "omit imports from generated source")
	command.Example = "  ibot click click_submit --vary=5 --hold=shift+control+cmd"
	return command
}

func newBox(service app.Service, clipboard Clipboard, stdout, stderr io.Writer) *cobra.Command {
	var vary string
	var noImports bool
	command := &cobra.Command{
		Use:   "box function_name",
		Short: "Capture a box and generate a click function",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if err := generator.ValidateFunctionName(args[0]); err != nil {
				return err
			}
			variation, err := model.ParseVariation(vary)
			if err != nil {
				return err
			}
			source, err := service.Box(command.Context(), model.BoxOptions{
				Name: args[0], Variation: variation, IncludeImports: !noImports,
			}, prompt(stderr))
			if err != nil {
				return err
			}
			return deliver(source, args[0], clipboard, stdout, stderr)
		},
	}
	command.Flags().StringVar(&vary, "vary", "0", "vary around center by PIXELS, or use all")
	command.Flags().BoolVar(&noImports, "no-imports", false, "omit imports from generated source")
	command.Example = "  ibot box click_inventory --vary=all"
	return command
}

func newClickImage(service app.Service, clipboard Clipboard, stdout, stderr io.Writer) *cobra.Command {
	var vary, stall, order, gap, hold string
	var confidence, timeout float64
	var waitFor, clickAll, noImports bool
	command := &cobra.Command{
		Use:   "click_image [function_name]",
		Short: "Capture an image and generate a function that locates and clicks it",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			name := optionalName(args, "click_image")
			if err := generator.ValidateFunctionName(name); err != nil {
				return err
			}
			variation, err := model.ParseVariation(vary)
			if err != nil {
				return err
			}
			stallRange, err := model.ParseSecondsRange(stall)
			if err != nil {
				return fmt.Errorf("stall: %w", err)
			}
			gapRange, err := model.ParseSecondsRange(gap)
			if err != nil {
				return fmt.Errorf("gap: %w", err)
			}
			keys, err := model.ParseHold(hold)
			if err != nil {
				return err
			}
			if !finite(confidence) || confidence <= 0 || confidence > 1 {
				return fmt.Errorf("confidence must be greater than zero and at most one")
			}
			if !finite(timeout) || timeout <= 0 {
				return fmt.Errorf("timeout must be greater than zero")
			}
			order = strings.ToLower(order)
			if order != "linear" && order != "backwards" && order != "random" {
				return fmt.Errorf("order must be linear, backwards, or random")
			}
			if !clickAll && (order != "linear" || gapRange != nil) {
				return fmt.Errorf("--order and --gap require --all")
			}
			source, err := service.Image(command.Context(), model.ImageOptions{
				Name: name, Variation: variation, Confidence: confidence, WaitFor: waitFor,
				Timeout: timeout, Stall: stallRange, ClickAll: clickAll, Order: order,
				Gap: gapRange, Hold: keys, IncludeImports: !noImports,
			}, prompt(stderr))
			if err != nil {
				return err
			}
			return deliver(source, name, clipboard, stdout, stderr)
		},
	}
	flags := command.Flags()
	flags.StringVar(&vary, "vary", "0", "vary around center by PIXELS, or use all")
	flags.Float64Var(&confidence, "confidence", 0.9, "minimum image similarity from greater than 0 through 1")
	flags.BoolVar(&waitFor, "wait-for", false, "poll until an image appears or timeout expires")
	flags.Float64Var(&timeout, "timeout", 30, "maximum wait in seconds with --wait-for")
	flags.StringVar(&stall, "stall", "", "delay after matching: SECONDS or MIN-MAX")
	flags.BoolVar(&clickAll, "all", false, "click every matching image")
	flags.StringVar(&order, "order", "linear", "match order for --all: linear, backwards, or random")
	flags.StringVar(&gap, "gap", "", "delay between --all clicks: SECONDS or MIN-MAX")
	flags.StringVar(&hold, "hold", "", "hold KEY[+KEY...] during the click sequence")
	flags.BoolVar(&noImports, "no-imports", false, "omit imports from generated source")
	command.Example = "  ibot click_image click_targets --wait-for --all --order=random --gap=0.5-3.8 --vary=all --hold=shift+control+cmd"
	return command
}

func newServe(serve ServeFunc) *cobra.Command {
	var address string
	var open bool
	command := &cobra.Command{
		Use:   "serve",
		Short: "Serve the local ibot web application",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return serve(command.Context(), ServeOptions{Address: address, Open: open})
		},
	}
	command.Flags().StringVar(&address, "addr", "127.0.0.1:8787", "loopback address for the web application")
	command.Flags().BoolVar(&open, "open", true, "open the application in the default browser")
	return command
}

func optionalName(args []string, fallback string) string {
	if len(args) == 0 {
		return fallback
	}
	return args[0]
}

func prompt(writer io.Writer) func(string) {
	return func(message string) { fmt.Fprintln(writer, message) }
}

func deliver(source, name string, clipboard Clipboard, stdout, stderr io.Writer) error {
	if clipboard != nil {
		if err := clipboard.WriteAll(source); err != nil {
			return fmt.Errorf("copy generated source: %w", err)
		}
	}
	fmt.Fprint(stdout, source)
	fmt.Fprintf(stderr, "\nCopied %s() to the clipboard.\n", name)
	return nil
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }
