package web

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/phillip-england/ibot/internal/app"
	"github.com/phillip-england/ibot/internal/generator"
	"github.com/phillip-england/ibot/internal/model"
	"github.com/phillip-england/ibot/internal/project"
	"github.com/phillip-england/ibot/internal/target"
)

//go:embed static/*
var assets embed.FS

type Server struct {
	Service    app.Service
	ProjectDir string
	ExportDir  string
	PointsDir  string
	BoxesDir   string
	ImagesDir  string
}

type Options struct {
	Address string
	Open    bool
	Logf    func(format string, args ...any)
}

type captureRequest struct {
	Mode          string  `json:"mode"`
	Output        string  `json:"output"`
	Name          string  `json:"name"`
	Vary          string  `json:"vary"`
	GridRows      int     `json:"gridRows"`
	GridColumns   int     `json:"gridColumns"`
	GridCell      int     `json:"gridCell"`
	Confidence    float64 `json:"confidence"`
	WaitFor       bool    `json:"waitFor"`
	NoClick       bool    `json:"noClick"`
	WaitUntilGone bool    `json:"waitUntilGone"`
	Timeout       float64 `json:"timeout"`
	Delay         string  `json:"delay"`
	Stall         string  `json:"stall"`
	ClickAll      bool    `json:"all"`
	Order         string  `json:"order"`
	Gap           string  `json:"gap"`
	Hold          string  `json:"hold"`
	NoImports     bool    `json:"noImports"`
}

type streamEvent struct {
	Type           string `json:"type"`
	Message        string `json:"message,omitempty"`
	Source         string `json:"source,omitempty"`
	InstallCommand string `json:"installCommand,omitempty"`
}

func (server Server) Handler() (http.Handler, error) {
	var err error
	server, err = server.withProject()
	if err != nil {
		return nil, err
	}
	static, err := fs.Sub(assets, "static")
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.Handle("GET /", http.FileServer(http.FS(static)))
	mux.HandleFunc("GET /api/health", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /api/capture", server.capture)
	mux.HandleFunc("GET /api/targets", server.targets)
	mux.HandleFunc("POST /api/targets/{kind}/{name}/capture", server.captureTarget)
	return secureLocal(mux), nil
}

func (server Server) Serve(ctx context.Context, options Options) error {
	if options.Address == "" {
		options.Address = "127.0.0.1:8787"
	}
	if err := validateLoopback(options.Address); err != nil {
		return err
	}
	var err error
	server, err = server.withProject()
	if err != nil {
		return err
	}
	if server.ExportDir != "" {
		if err := validatePythonModule(server.ExportDir); err != nil {
			return err
		}
	}
	for label, dir := range map[string]string{"points": server.PointsDir, "boxes": server.BoxesDir, "images": server.ImagesDir} {
		if dir == "" {
			continue
		}
		info, err := os.Stat(dir)
		if err != nil {
			return fmt.Errorf("%s directory: %w", label, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("%s path is not a directory", label)
		}
	}
	handler, err := server.Handler()
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", options.Address)
	if err != nil {
		return err
	}
	defer listener.Close()
	httpServer := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdown)
	}()
	applicationURL := "http://" + listener.Addr().String()
	if options.Logf != nil {
		options.Logf("ibot web application: %s", applicationURL)
	}
	if options.Open {
		go func() { _ = openBrowser(applicationURL) }()
	}
	err = httpServer.Serve(listener)
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func (server Server) withProject() (Server, error) {
	if server.ProjectDir == "" {
		return server, nil
	}
	layout, err := project.Open(server.ProjectDir)
	if err != nil {
		return Server{}, err
	}
	server.ExportDir = layout.Root
	server.PointsDir = layout.Points
	server.BoxesDir = layout.Boxes
	server.ImagesDir = layout.Images
	return server, nil
}

func (server Server) targets(writer http.ResponseWriter, _ *http.Request) {
	entries, err := target.List(server.PointsDir, server.BoxesDir)
	if err != nil {
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"targets":          entries,
		"pointsConfigured": server.PointsDir != "",
		"boxesConfigured":  server.BoxesDir != "",
	})
}

func (server Server) captureTarget(writer http.ResponseWriter, request *http.Request) {
	kind, name := request.PathValue("kind"), request.PathValue("name")
	dir := server.PointsDir
	if kind == "box" {
		dir = server.BoxesDir
	} else if kind != "point" {
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "target kind must be point or box"})
		return
	}
	path, err := target.Path(dir, name)
	if err != nil {
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if _, err := os.Stat(path); err != nil {
		writeJSON(writer, http.StatusNotFound, map[string]string{"error": "target file does not exist"})
		return
	}
	flusher, ok := writer.(http.Flusher)
	if !ok {
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "streaming is unavailable"})
		return
	}
	writer.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(writer)
	emit := func(event streamEvent) { _ = encoder.Encode(event); flusher.Flush() }
	prompt := func(message string) { emit(streamEvent{Type: "prompt", Message: message}) }
	emit(streamEvent{Type: "status", Message: "Editing " + name})
	if kind == "point" {
		point, captureErr := server.Service.Capture.Point(request.Context(), prompt)
		if captureErr == nil {
			captureErr = target.WritePoint(path, point)
		}
		err = captureErr
	} else {
		corners, captureErr := server.Service.Capture.Corners(request.Context(), prompt)
		if captureErr == nil {
			captureErr = target.WriteBox(path, corners)
		}
		err = captureErr
	}
	if err != nil {
		emit(streamEvent{Type: "error", Message: err.Error()})
		return
	}
	emit(streamEvent{Type: "target", Message: "Updated " + filepath.Base(path)})
}

func (server Server) capture(writer http.ResponseWriter, request *http.Request) {
	request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
	var input captureRequest
	if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "invalid JSON request"})
		return
	}
	if input.Output == "json" || input.Output == "image" {
		server.captureAsset(writer, request, input)
		return
	}
	parsed, err := parseRequest(input)
	if err != nil {
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	flusher, ok := writer.(http.Flusher)
	if !ok {
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "streaming is unavailable"})
		return
	}
	writer.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(writer)
	emit := func(event streamEvent) {
		_ = encoder.Encode(event)
		flusher.Flush()
	}
	prompt := func(message string) { emit(streamEvent{Type: "prompt", Message: message}) }
	emit(streamEvent{Type: "status", Message: "Capture started"})

	var source string
	switch parsed.mode {
	case "click":
		source, err = server.Service.Point(request.Context(), parsed.point, prompt)
	case "box":
		source, err = server.Service.Box(request.Context(), parsed.box, prompt)
	case "click_image", "image_exists":
		source, err = server.Service.Image(request.Context(), parsed.image, prompt)
	}
	if err != nil {
		emit(streamEvent{Type: "error", Message: err.Error()})
		return
	}
	message := "Generated successfully"
	if server.ExportDir != "" {
		if err := exportPythonFunction(server.ExportDir, parsed.name, source); err != nil {
			emit(streamEvent{Type: "error", Message: fmt.Sprintf("export generated function: %v", err)})
			return
		}
		message = fmt.Sprintf("Generated and exported to %s", server.ExportDir)
	}
	emit(streamEvent{
		Type:           "source",
		Source:         source,
		InstallCommand: installCommand(parsed.mode),
		Message:        message,
	})
}

func (server Server) captureAsset(writer http.ResponseWriter, request *http.Request, input captureRequest) {
	mode := strings.ToLower(strings.TrimSpace(input.Mode))
	name := strings.TrimSpace(input.Name)
	var dir, extension string
	if input.Output == "json" && mode == "click" {
		dir, extension = server.PointsDir, ".json"
	} else if input.Output == "json" && mode == "box" {
		dir, extension = server.BoxesDir, ".json"
	} else if input.Output == "image" && (mode == "click_image" || mode == "image_exists") {
		dir, extension = server.ImagesDir, ".png"
	} else {
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "selected output is not available for this capture mode"})
		return
	}
	path, err := assetPath(dir, name, extension)
	if err != nil {
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	flusher, ok := writer.(http.Flusher)
	if !ok {
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "streaming is unavailable"})
		return
	}
	writer.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(writer)
	emit := func(event streamEvent) { _ = encoder.Encode(event); flusher.Flush() }
	prompt := func(message string) { emit(streamEvent{Type: "prompt", Message: message}) }
	emit(streamEvent{Type: "status", Message: "Capture started"})
	if input.Output == "json" && mode == "click" {
		point, captureErr := server.Service.Capture.Point(request.Context(), prompt)
		if captureErr == nil {
			captureErr = target.WritePoint(path, point)
		}
		err = captureErr
	} else {
		corners, captureErr := server.Service.Capture.Corners(request.Context(), prompt)
		if captureErr == nil && input.Output == "json" {
			captureErr = target.WriteBox(path, corners)
		}
		if captureErr == nil && input.Output == "image" {
			var png []byte
			png, captureErr = server.Service.Capture.PNG(request.Context(), corners)
			if captureErr == nil {
				captureErr = target.WriteImage(path, png)
			}
		}
		err = captureErr
	}
	if err != nil {
		emit(streamEvent{Type: "error", Message: err.Error()})
		return
	}
	emit(streamEvent{Type: "asset", Message: fmt.Sprintf("Saved %s", path)})
}

func assetPath(dir, name, extension string) (string, error) {
	if dir == "" {
		return "", fmt.Errorf("this output requires an initialized project")
	}
	name = strings.TrimSpace(name)
	if strings.HasSuffix(strings.ToLower(name), extension) {
		name = name[:len(name)-len(extension)]
	}
	if name == "" || name != filepath.Base(name) || strings.ContainsAny(name, `/\\`) {
		return "", fmt.Errorf("file name must be a simple non-empty name")
	}
	for _, char := range name {
		if !(char == '_' || char == '-' || char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9') {
			return "", fmt.Errorf("file name may contain only letters, numbers, underscores, and hyphens")
		}
	}
	return filepath.Join(dir, name+extension), nil
}

func installCommand(mode string) string {
	if mode == "click_image" || mode == "image_exists" {
		return "uv add pyautogui pillow opencv-python-headless"
	}
	return "uv add pyautogui"
}

type parsedRequest struct {
	mode  string
	name  string
	point model.PointOptions
	box   model.BoxOptions
	image model.ImageOptions
}

func parseRequest(input captureRequest) (parsedRequest, error) {
	input.Mode = strings.ToLower(strings.TrimSpace(input.Mode))
	if input.Mode != "click" && input.Mode != "box" && input.Mode != "click_image" && input.Mode != "image_exists" {
		return parsedRequest{}, fmt.Errorf("mode must be click, box, click_image, or image_exists")
	}
	if input.Name == "" {
		switch input.Mode {
		case "click":
			input.Name = "click_position"
		case "click_image":
			input.Name = "click_image"
		case "image_exists":
			input.Name = "image_exists"
		default:
			input.Name = "click_box"
		}
	}
	if err := generator.ValidateFunctionName(input.Name); err != nil {
		return parsedRequest{}, err
	}
	variation, err := model.ParseVariation(input.Vary)
	if err != nil {
		return parsedRequest{}, err
	}
	if input.Mode == "click" && variation.All {
		return parsedRequest{}, fmt.Errorf("click variation does not support all")
	}
	hold, err := model.ParseHold(input.Hold)
	if err != nil {
		return parsedRequest{}, err
	}
	delayValue := input.Delay
	if delayValue == "" {
		delayValue = input.Stall
	}
	delay, err := model.ParseSecondsRange(delayValue)
	if err != nil {
		return parsedRequest{}, fmt.Errorf("delay: %w", err)
	}
	result := parsedRequest{mode: input.Mode, name: input.Name}
	result.point = model.PointOptions{Name: input.Name, Variation: variation, Delay: delay, Hold: hold, IncludeImports: !input.NoImports}
	var grid *model.GridTarget
	if input.Mode == "box" && (input.GridRows != 0 || input.GridColumns != 0 || input.GridCell != 0) {
		grid, err = model.NewGridTarget(input.GridRows, input.GridColumns, input.GridCell)
		if err != nil {
			return parsedRequest{}, err
		}
	}
	result.box = model.BoxOptions{Name: input.Name, Variation: variation, Grid: grid, Delay: delay, IncludeImports: !input.NoImports}
	if input.Mode != "click_image" && input.Mode != "image_exists" {
		return result, nil
	}
	if input.Confidence == 0 {
		input.Confidence = 0.9
	}
	if input.Timeout == 0 {
		input.Timeout = 30
	}
	if input.Order == "" {
		input.Order = "linear"
	}
	if !finite(input.Confidence) || input.Confidence <= 0 || input.Confidence > 1 {
		return parsedRequest{}, fmt.Errorf("confidence must be greater than zero and at most one")
	}
	if !finite(input.Timeout) || input.Timeout <= 0 {
		return parsedRequest{}, fmt.Errorf("timeout must be greater than zero")
	}
	stall := delay
	gap, err := model.ParseSecondsRange(input.Gap)
	if err != nil {
		return parsedRequest{}, fmt.Errorf("gap: %w", err)
	}
	input.Order = strings.ToLower(input.Order)
	if input.Order != "linear" && input.Order != "backwards" && input.Order != "random" {
		return parsedRequest{}, fmt.Errorf("order must be linear, backwards, or random")
	}
	if !input.ClickAll && (input.Order != "linear" || gap != nil) {
		return parsedRequest{}, fmt.Errorf("order and gap require all")
	}
	if input.WaitFor && input.WaitUntilGone {
		return parsedRequest{}, fmt.Errorf("wait for and wait until gone cannot be used together")
	}
	if input.NoClick && !input.WaitFor {
		return parsedRequest{}, fmt.Errorf("no click requires wait for")
	}
	if input.NoClick && (variation.All || variation.Pixels > 0 || stall != nil || input.ClickAll || input.Order != "linear" || gap != nil || len(hold) > 0) {
		return parsedRequest{}, fmt.Errorf("no click cannot be combined with click options")
	}
	if input.Mode == "image_exists" && (input.WaitFor || input.NoClick || input.WaitUntilGone || variation.All || variation.Pixels > 0 || stall != nil || input.ClickAll || input.Order != "linear" || gap != nil || len(hold) > 0) {
		return parsedRequest{}, fmt.Errorf("image existence checks cannot be combined with click or wait options")
	}
	if input.WaitUntilGone && (variation.All || variation.Pixels > 0 || stall != nil || input.ClickAll || input.Order != "linear" || gap != nil || len(hold) > 0) {
		return parsedRequest{}, fmt.Errorf("wait until gone cannot be combined with click options")
	}
	result.image = model.ImageOptions{
		Name: input.Name, Variation: variation, Confidence: input.Confidence, ReturnExists: input.Mode == "image_exists",
		WaitFor: input.WaitFor, NoClick: input.NoClick, WaitUntilGone: input.WaitUntilGone, Timeout: input.Timeout, Delay: stall,
		ClickAll: input.ClickAll, Order: input.Order, Gap: gap, Hold: hold,
		IncludeImports: !input.NoImports,
	}
	return result, nil
}

func secureLocal(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; connect-src 'self'; img-src 'self' data:")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("Referrer-Policy", "no-referrer")
		host, _, err := net.SplitHostPort(request.RemoteAddr)
		if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
			http.Error(writer, "loopback access only", http.StatusForbidden)
			return
		}
		if origin := request.Header.Get("Origin"); origin != "" {
			parsed, err := url.Parse(origin)
			if err != nil || !strings.EqualFold(parsed.Host, request.Host) {
				http.Error(writer, "invalid origin", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(writer, request)
	})
}

func validateLoopback(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("invalid serve address: %w", err)
	}
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("serve address must use a loopback host")
	}
	return nil
}

func openBrowser(address string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", address)
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", address)
	default:
		command = exec.Command("xdg-open", address)
	}
	return command.Start()
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }
