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
	"github.com/phillip-england/ibot/internal/project"
	"github.com/phillip-england/ibot/internal/target"
)

type Clipboard interface {
	WriteAll(text string) error
}

type ServeOptions struct {
	Address string
	Open    bool
	Project string
	Export  string
	Points  string
	Boxes   string
	Images  string
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
	root.AddCommand(newInit(stdout))
	root.AddCommand(newPatchUtilities(stdout))
	if serve != nil {
		root.AddCommand(newServe(serve))
	}
	return root
}

func newPatchUtilities(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "patch-utilities PROJECT",
		Short: "Update an ibot project's utility functions",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			path, err := project.PatchUtilities(args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, "Updated ibot utilities at %s\n", path)
			return nil
		},
	}
}

func newInit(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "init PROJECT",
		Short: "Initialize an ibot Python automation project",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			layout, err := project.Init(args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, "Initialized ibot project at %s\n", layout.Root)
			return nil
		},
	}
}

func newClick(service app.Service, clipboard Clipboard, stdout, stderr io.Writer) *cobra.Command {
	var vary, delay, hold, saveJSON string
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
			delayRange, err := model.ParseSecondsRange(delay)
			if err != nil {
				return fmt.Errorf("delay: %w", err)
			}
			keys, err := model.ParseHold(hold)
			if err != nil {
				return err
			}
			if saveJSON != "" {
				point, err := service.Capture.Point(command.Context(), prompt(stderr))
				if err != nil {
					return err
				}
				if err := target.WritePoint(saveJSON, point); err != nil {
					return err
				}
				fmt.Fprintf(stderr, "Saved point to %s.\n", saveJSON)
				return nil
			}
			source, err := service.Point(command.Context(), model.PointOptions{
				Name: name, Variation: variation, Delay: delayRange, Hold: keys, IncludeImports: !noImports,
			}, prompt(stderr))
			if err != nil {
				return err
			}
			return deliver(source, name, clipboard, stdout, stderr)
		},
	}
	command.Flags().StringVar(&vary, "vary", "0", "vary each click axis by PIXELS")
	command.Flags().StringVar(&delay, "delay", "", "move, then wait SECONDS or MIN-MAX before clicking")
	command.Flags().StringVar(&hold, "hold", "", "hold KEY[+KEY...] while clicking")
	command.Flags().BoolVar(&noImports, "no-imports", false, "omit imports from generated source")
	command.Flags().StringVar(&saveJSON, "save-json", "", "save the captured point to PATH instead of generating source")
	command.Example = "  ibot click click_submit --vary=5 --hold=shift+control+cmd"
	return command
}

func newBox(service app.Service, clipboard Clipboard, stdout, stderr io.Writer) *cobra.Command {
	var vary, delay, saveJSON string
	var gridRows, gridColumns, gridCell int
	var noImports bool
	command := &cobra.Command{
		Use:   "box [function_name]",
		Short: "Capture a box and generate a click function",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			name := optionalName(args, "click_box")
			if err := generator.ValidateFunctionName(name); err != nil {
				return err
			}
			variation, err := model.ParseVariation(vary)
			if err != nil {
				return err
			}
			delayRange, err := model.ParseSecondsRange(delay)
			if err != nil {
				return fmt.Errorf("delay: %w", err)
			}
			var grid *model.GridTarget
			if gridRows != 0 || gridColumns != 0 || gridCell != -1 {
				grid, err = model.NewGridTarget(gridRows, gridColumns, gridCell)
				if err != nil {
					return err
				}
			}
			if saveJSON != "" {
				corners, err := service.Capture.Corners(command.Context(), prompt(stderr))
				if err != nil {
					return err
				}
				if err := target.WriteBox(saveJSON, corners); err != nil {
					return err
				}
				fmt.Fprintf(stderr, "Saved box to %s.\n", saveJSON)
				return nil
			}
			source, err := service.Box(command.Context(), model.BoxOptions{
				Name: name, Variation: variation, Grid: grid, Delay: delayRange, IncludeImports: !noImports,
			}, prompt(stderr))
			if err != nil {
				return err
			}
			return deliver(source, name, clipboard, stdout, stderr)
		},
	}
	command.Flags().StringVar(&vary, "vary", "0", "vary around center by PIXELS, or use all")
	command.Flags().StringVar(&delay, "delay", "", "move, then wait SECONDS or MIN-MAX before clicking")
	command.Flags().IntVar(&gridRows, "grid-rows", 0, "number of rows in the box grid")
	command.Flags().IntVar(&gridColumns, "grid-columns", 0, "number of columns in the box grid")
	command.Flags().IntVar(&gridCell, "grid-cell", -1, "zero-based cell to click, ordered left-to-right then top-to-bottom")
	command.Flags().BoolVar(&noImports, "no-imports", false, "omit imports from generated source")
	command.Flags().StringVar(&saveJSON, "save-json", "", "save the captured box to PATH instead of generating source")
	command.Example = "  ibot box click_inventory --grid-rows=4 --grid-columns=4 --grid-cell=3"
	return command
}

func newClickImage(service app.Service, clipboard Clipboard, stdout, stderr io.Writer) *cobra.Command {
	var vary, stall, order, gap, hold string
	var confidence, timeout float64
	var waitFor, noClick, waitUntilGone, clickAll, noImports bool
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
			if waitFor && waitUntilGone {
				return fmt.Errorf("--wait-for and --wait-until-gone cannot be used together")
			}
			if noClick && !waitFor {
				return fmt.Errorf("--no-click requires --wait-for")
			}
			if noClick && (variation.All || variation.Pixels > 0 || stallRange != nil || clickAll || order != "linear" || gapRange != nil || len(keys) > 0) {
				return fmt.Errorf("--no-click cannot be combined with click options")
			}
			if waitUntilGone && (variation.All || variation.Pixels > 0 || stallRange != nil || clickAll || order != "linear" || gapRange != nil || len(keys) > 0) {
				return fmt.Errorf("--wait-until-gone cannot be combined with click options")
			}
			source, err := service.Image(command.Context(), model.ImageOptions{
				Name: name, Variation: variation, Confidence: confidence, WaitFor: waitFor, NoClick: noClick, WaitUntilGone: waitUntilGone,
				Timeout: timeout, Delay: stallRange, ClickAll: clickAll, Order: order,
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
	flags.BoolVar(&noClick, "no-click", false, "return without clicking after --wait-for finds an image")
	flags.BoolVar(&waitUntilGone, "wait-until-gone", false, "poll until an image disappears or timeout expires")
	flags.Float64Var(&timeout, "timeout", 30, "maximum wait in seconds with an image wait option")
	flags.StringVar(&stall, "stall", "", "delay after matching: SECONDS or MIN-MAX")
	flags.StringVar(&stall, "delay", "", "move, then wait SECONDS or MIN-MAX before clicking")
	flags.BoolVar(&clickAll, "all", false, "click every matching image")
	flags.StringVar(&order, "order", "linear", "match order for --all: linear, backwards, or random")
	flags.StringVar(&gap, "gap", "", "delay between --all clicks: SECONDS or MIN-MAX")
	flags.StringVar(&hold, "hold", "", "hold KEY[+KEY...] during the click sequence")
	flags.BoolVar(&noImports, "no-imports", false, "omit imports from generated source")
	command.Example = "  ibot click_image click_targets --wait-for --all --order=random --gap=0.5-3.8 --vary=all --hold=shift+control+cmd"
	return command
}

func newServe(serve ServeFunc) *cobra.Command {
	var address, export, points, boxes, images string
	var open bool
	command := &cobra.Command{
		Use:     "serve [PROJECT]",
		Aliases: []string{"server"},
		Short:   "Serve the local ibot web application",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			projectDir := ""
			if len(args) == 1 {
				if strings.HasPrefix(args[0], "export=") {
					if export != "" || strings.TrimPrefix(args[0], "export=") == "" {
						return fmt.Errorf("invalid duplicate export path")
					}
					export = strings.TrimPrefix(args[0], "export=")
				} else {
					projectDir = args[0]
				}
			}
			if projectDir != "" && (export != "" || points != "" || boxes != "" || images != "") {
				return fmt.Errorf("PROJECT cannot be combined with explicit directory flags")
			}
			return serve(command.Context(), ServeOptions{Address: address, Open: open, Project: projectDir, Export: export, Points: points, Boxes: boxes, Images: images})
		},
	}
	command.Flags().StringVar(&address, "addr", "127.0.0.1:8787", "loopback address for the web application")
	command.Flags().BoolVar(&open, "open", true, "open the application in the default browser")
	command.Flags().StringVar(&export, "export", "", "export generated functions to a Python module directory")
	command.Flags().StringVar(&points, "points", "", "directory containing saved point JSON files")
	command.Flags().StringVar(&boxes, "boxes", "", "directory containing saved box JSON files")
	command.Flags().StringVar(&images, "images", "", "directory for saved PNG image files")
	command.Example = "  ibot init ./automation\n  ibot serve ./automation"
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
