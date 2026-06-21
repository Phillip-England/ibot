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
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/phillip-england/ibot/internal/app"
	"github.com/phillip-england/ibot/internal/generator"
	"github.com/phillip-england/ibot/internal/model"
)

//go:embed static/*
var assets embed.FS

type Server struct {
	Service app.Service
}

type Options struct {
	Address string
	Open    bool
	Logf    func(format string, args ...any)
}

type captureRequest struct {
	Mode       string  `json:"mode"`
	Name       string  `json:"name"`
	Vary       string  `json:"vary"`
	Confidence float64 `json:"confidence"`
	WaitFor    bool    `json:"waitFor"`
	Timeout    float64 `json:"timeout"`
	Stall      string  `json:"stall"`
	ClickAll   bool    `json:"all"`
	Order      string  `json:"order"`
	Gap        string  `json:"gap"`
	Hold       string  `json:"hold"`
	NoImports  bool    `json:"noImports"`
}

type streamEvent struct {
	Type           string `json:"type"`
	Message        string `json:"message,omitempty"`
	Source         string `json:"source,omitempty"`
	InstallCommand string `json:"installCommand,omitempty"`
}

func (server Server) Handler() (http.Handler, error) {
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
	return secureLocal(mux), nil
}

func (server Server) Serve(ctx context.Context, options Options) error {
	if options.Address == "" {
		options.Address = "127.0.0.1:8787"
	}
	if err := validateLoopback(options.Address); err != nil {
		return err
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

func (server Server) capture(writer http.ResponseWriter, request *http.Request) {
	request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
	var input captureRequest
	if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "invalid JSON request"})
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
	case "click_image":
		source, err = server.Service.Image(request.Context(), parsed.image, prompt)
	}
	if err != nil {
		emit(streamEvent{Type: "error", Message: err.Error()})
		return
	}
	emit(streamEvent{
		Type:           "source",
		Source:         source,
		InstallCommand: installCommand(parsed.mode),
		Message:        "Generated successfully",
	})
}

func installCommand(mode string) string {
	if mode == "click_image" {
		return "uv add pyautogui pillow opencv-python-headless"
	}
	return "uv add pyautogui"
}

type parsedRequest struct {
	mode  string
	point model.PointOptions
	box   model.BoxOptions
	image model.ImageOptions
}

func parseRequest(input captureRequest) (parsedRequest, error) {
	input.Mode = strings.ToLower(strings.TrimSpace(input.Mode))
	if input.Mode != "click" && input.Mode != "box" && input.Mode != "click_image" {
		return parsedRequest{}, fmt.Errorf("mode must be click, box, or click_image")
	}
	if input.Name == "" {
		switch input.Mode {
		case "click":
			input.Name = "click_position"
		case "click_image":
			input.Name = "click_image"
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
	result := parsedRequest{mode: input.Mode}
	result.point = model.PointOptions{Name: input.Name, Variation: variation, Hold: hold, IncludeImports: !input.NoImports}
	result.box = model.BoxOptions{Name: input.Name, Variation: variation, IncludeImports: !input.NoImports}
	if input.Mode != "click_image" {
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
	stall, err := model.ParseSecondsRange(input.Stall)
	if err != nil {
		return parsedRequest{}, fmt.Errorf("stall: %w", err)
	}
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
	result.image = model.ImageOptions{
		Name: input.Name, Variation: variation, Confidence: input.Confidence,
		WaitFor: input.WaitFor, Timeout: input.Timeout, Stall: stall,
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
