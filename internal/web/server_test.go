package web

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
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
	for _, message := range []string{"top-left", "top-right", "bottom-right", "bottom-left"} {
		prompt(message)
	}
	return [4]model.Position{{X: 10, Y: 20}, {X: 110, Y: 20}, {X: 110, Y: 80}, {X: 10, Y: 80}}, nil
}

func (fakeCapture) PNG(context.Context, [4]model.Position) ([]byte, error) {
	return []byte("png bytes"), nil
}

func TestEmbeddedApplicationAndHealth(t *testing.T) {
	handler := testHandler(t)
	for path, expected := range map[string]string{"/": "ibot studio", "/app.css": "--accent", "/app.js": "readEvents"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.RemoteAddr = "127.0.0.1:12345"
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), expected) {
			t.Errorf("GET %s = %d, body missing %q", path, response.Code, expected)
		}
	}
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"status":"ok"`) {
		t.Fatalf("health response = %d %s", response.Code, response.Body.String())
	}
}

func TestMaximalImageCaptureStream(t *testing.T) {
	handler := testHandler(t)
	body := `{
        "mode":"click_image", "name":"click_targets", "vary":"all",
        "confidence":0.85, "waitFor":true, "timeout":20, "stall":"1-2",
        "all":true, "order":"random", "gap":"0.5-3.8",
        "hold":"shift+control+cmd", "noImports":true
    }`
	request := httptest.NewRequest(http.MethodPost, "/api/capture", strings.NewReader(body))
	request.RemoteAddr = "127.0.0.1:12345"
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "http://example.com")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	output := response.Body.String()
	if response.Code != http.StatusOK {
		t.Fatalf("capture response = %d %s", response.Code, output)
	}
	if strings.Count(output, `"type":"prompt"`) != 4 {
		t.Fatalf("expected four prompt events, got %s", output)
	}
	for _, expected := range []string{
		`"type":"source"`, "def click_targets", "locateAll", "random.shuffle(matches)",
		"random.uniform(1, 2)", "random.uniform(0.5, 3.8)",
		`(\"shift\", \"ctrl\", \"command\")`, "random.randint(left, right)",
		`"installCommand":"uv add pyautogui pillow opencv-python-headless"`,
	} {
		if !strings.Contains(output, expected) {
			t.Errorf("capture stream missing %q", expected)
		}
	}
}

func TestPointCaptureIncludesUvInstallCommand(t *testing.T) {
	handler := testHandler(t)
	request := httptest.NewRequest(http.MethodPost, "/api/capture", strings.NewReader(`{"mode":"click"}`))
	request.RemoteAddr = "127.0.0.1:12345"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"installCommand":"uv add pyautogui"`) {
		t.Fatalf("point capture response = %d %s", response.Code, response.Body.String())
	}
}

func TestBoxCaptureCanClickAnywhereInside(t *testing.T) {
	handler := testHandler(t)
	request := httptest.NewRequest(http.MethodPost, "/api/capture", strings.NewReader(`{"mode":"box","vary":"all"}`))
	request.RemoteAddr = "127.0.0.1:12345"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	output := response.Body.String()
	for _, expected := range []string{"random.randint(10, 110)", "random.randint(20, 80)"} {
		if response.Code != http.StatusOK || !strings.Contains(output, expected) {
			t.Fatalf("box capture response missing %q: %d %s", expected, response.Code, output)
		}
	}
}

func TestLocalSecurity(t *testing.T) {
	handler := testHandler(t)
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "192.0.2.1:12345"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("non-loopback response = %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/capture", strings.NewReader(`{"mode":"click"}`))
	request.RemoteAddr = "127.0.0.1:12345"
	request.Header.Set("Origin", "https://attacker.example")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("cross-origin response = %d", response.Code)
	}
}

func TestRejectsInvalidCaptureBeforePrompting(t *testing.T) {
	handler := testHandler(t)
	request := httptest.NewRequest(http.MethodPost, "/api/capture", strings.NewReader(`{"mode":"click_image","gap":"1"}`))
	request.RemoteAddr = "127.0.0.1:12345"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	data, _ := io.ReadAll(response.Result().Body)
	if response.Code != http.StatusBadRequest || !strings.Contains(string(data), "require all") {
		t.Fatalf("invalid capture response = %d %s", response.Code, data)
	}
}

func TestServeAddressMustBeLoopback(t *testing.T) {
	for _, address := range []string{"0.0.0.0:8787", "192.0.2.10:8787"} {
		if err := validateLoopback(address); err == nil {
			t.Errorf("validateLoopback(%q) unexpectedly succeeded", address)
		}
	}
	for _, address := range []string{"127.0.0.1:8787", "[::1]:8787", "localhost:8787"} {
		if err := validateLoopback(address); err != nil {
			t.Errorf("validateLoopback(%q) = %v", address, err)
		}
	}
}

func testHandler(t *testing.T) http.Handler {
	t.Helper()
	handler, err := (Server{Service: app.Service{Capture: fakeCapture{}}}).Handler()
	if err != nil {
		t.Fatal(err)
	}
	return handler
}
