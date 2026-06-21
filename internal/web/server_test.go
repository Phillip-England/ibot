package web

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/phillip-england/ibot/internal/app"
	"github.com/phillip-england/ibot/internal/capture"
	"github.com/phillip-england/ibot/internal/model"
	"github.com/phillip-england/ibot/internal/project"
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
	for path, expected := range map[string]string{
		"/": "ibot studio", "/app.css": "--accent", "/app.js": "readEvents",
		"/utilities.html": "perform_random_actions", "/utilities.js": "utility-query",
	} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.RemoteAddr = "127.0.0.1:12345"
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), expected) {
			t.Errorf("GET %s = %d, body missing %q", path, response.Code, expected)
		}
	}
	utilitiesRequest := httptest.NewRequest(http.MethodGet, "/utilities.html", nil)
	utilitiesRequest.RemoteAddr = "127.0.0.1:12345"
	utilitiesResponse := httptest.NewRecorder()
	handler.ServeHTTP(utilitiesResponse, utilitiesRequest)
	for _, expected := range []string{
		"random_number", "random.sample(actions, amount)",
		"9 utilities", "random_wait", "wait_until", "retry", "roll(chance, out_of=100)",
		"click_saved_point", "click_saved_box",
	} {
		if !strings.Contains(utilitiesResponse.Body.String(), expected) {
			t.Errorf("utilities page missing %q", expected)
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

func TestImageCaptureAcceptsIntegerAndDecimalTimeouts(t *testing.T) {
	for _, timeout := range []float64{10, 10.5} {
		request := captureRequest{Mode: "click_image", WaitFor: true, Timeout: timeout}
		parsed, err := parseRequest(request)
		if err != nil {
			t.Fatalf("timeout %v was rejected: %v", timeout, err)
		}
		if parsed.image.Timeout != request.Timeout {
			t.Errorf("timeout = %v, want %v", parsed.image.Timeout, request.Timeout)
		}
	}
}

func TestImageCaptureParsesWaitUntilGone(t *testing.T) {
	parsed, err := parseRequest(captureRequest{
		Mode: "click_image", WaitUntilGone: true, Timeout: 15,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !parsed.image.WaitUntilGone {
		t.Fatal("wait-until-gone was not passed to image options")
	}
}

func TestImageCaptureParsesNoClickWait(t *testing.T) {
	parsed, err := parseRequest(captureRequest{
		Mode: "click_image", WaitFor: true, NoClick: true, Timeout: 15,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !parsed.image.WaitFor || !parsed.image.NoClick {
		t.Fatal("no-click wait was not passed to image options")
	}
}

func TestImageNoClickWaitCaptureStream(t *testing.T) {
	handler := testHandler(t)
	request := httptest.NewRequest(http.MethodPost, "/api/capture", strings.NewReader(
		`{"mode":"click_image","name":"wait_for_dialog","waitFor":true,"noClick":true,"timeout":15}`,
	))
	request.RemoteAddr = "127.0.0.1:12345"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	output := response.Body.String()
	for _, expected := range []string{"def wait_for_dialog", "wait_for=True", "click=False", "timeout=15", "while not matches:"} {
		if response.Code != http.StatusOK || !strings.Contains(output, expected) {
			t.Fatalf("no-click wait response missing %q: %d %s", expected, response.Code, output)
		}
	}
}

func TestImageExistsCaptureStream(t *testing.T) {
	handler := testHandler(t)
	request := httptest.NewRequest(http.MethodPost, "/api/capture", strings.NewReader(
		`{"mode":"image_exists","name":"is_submit_visible","confidence":0.88}`,
	))
	request.RemoteAddr = "127.0.0.1:12345"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	output := response.Body.String()
	for _, expected := range []string{
		`def is_submit_visible(confidence=0.88) -\u003e bool:`,
		"return True", "return False",
		`"installCommand":"uv add pyautogui pillow opencv-python-headless"`,
	} {
		if response.Code != http.StatusOK || !strings.Contains(output, expected) {
			t.Fatalf("image existence response missing %q: %d %s", expected, response.Code, output)
		}
	}
	if strings.Contains(output, "pyautogui.click(") {
		t.Fatalf("image existence response contains a click: %s", output)
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
		`delay=(1, 2)`, `gap=(0.5, 3.8)`, "random.uniform(*delay_range)",
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

func TestPointCaptureAcceptsDelayDefault(t *testing.T) {
	handler := testHandler(t)
	request := httptest.NewRequest(http.MethodPost, "/api/capture", strings.NewReader(`{"mode":"click","name":"click_later","delay":"3-5"}`))
	request.RemoteAddr = "127.0.0.1:12345"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "delay=(3, 5)") {
		t.Fatalf("point delay response = %d %s", response.Code, response.Body.String())
	}
}

func TestSavedTargetsCanBeListedAndRecaptured(t *testing.T) {
	points := t.TempDir()
	if err := os.WriteFile(filepath.Join(points, "submit.json"), []byte("{\"x\":1,\"y\":2}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	server := Server{Service: app.Service{Capture: fakeCapture{}}, PointsDir: points}
	handler, err := server.Handler()
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/targets", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"x":1`) {
		t.Fatalf("list = %d %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "/api/targets/point/submit.json/capture", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"type":"target"`) {
		t.Fatalf("capture = %d %s", response.Code, response.Body.String())
	}
	data, err := os.ReadFile(filepath.Join(points, "submit.json"))
	if err != nil || !strings.Contains(string(data), `"x": 10`) {
		t.Fatalf("updated JSON = %s, %v", data, err)
	}
}

func TestProjectCapturesJSONAndImageAssets(t *testing.T) {
	root := t.TempDir()
	if _, err := project.Init(root); err != nil {
		t.Fatal(err)
	}
	server := Server{Service: app.Service{Capture: fakeCapture{}}, ProjectDir: root}
	handler, err := server.Handler()
	if err != nil {
		t.Fatal(err)
	}
	requests := []struct{ body, path, contains string }{
		{`{"mode":"click","output":"json","name":"submit"}`, "points/submit.json", `"x": 10`},
		{`{"mode":"box","output":"json","name":"panel"}`, "boxes/panel.json", `"corners"`},
		{`{"mode":"click_image","output":"image","name":"button"}`, "images/button.png", "png bytes"},
	}
	for _, item := range requests {
		request := httptest.NewRequest(http.MethodPost, "/api/capture", strings.NewReader(item.body))
		request.RemoteAddr = "127.0.0.1:12345"
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"type":"asset"`) {
			t.Fatalf("capture %s = %d %s", item.path, response.Code, response.Body.String())
		}
		data, readErr := os.ReadFile(filepath.Join(root, item.path))
		if readErr != nil || !strings.Contains(string(data), item.contains) {
			t.Fatalf("asset %s = %q, %v", item.path, data, readErr)
		}
	}
}

func TestCaptureExportsFunctionToPythonModule(t *testing.T) {
	directory := t.TempDir()
	initPath := filepath.Join(directory, "__init__.py")
	if err := os.WriteFile(initPath, []byte("# helpers\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	server := Server{Service: app.Service{Capture: fakeCapture{}}, ExportDir: directory}
	handler, err := server.Handler()
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		request := httptest.NewRequest(http.MethodPost, "/api/capture", strings.NewReader(`{"mode":"click","name":"foo"}`))
		request.RemoteAddr = "127.0.0.1:12345"
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Generated and exported") {
			t.Fatalf("export response = %d %s", response.Code, response.Body.String())
		}
	}
	functionSource, err := os.ReadFile(filepath.Join(directory, "foo.py"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(functionSource), "def foo") {
		t.Fatalf("foo.py = %s", functionSource)
	}
	initSource, err := os.ReadFile(initPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(initSource), "from .foo import *") != 1 {
		t.Fatalf("__init__.py = %s", initSource)
	}
}

func TestCaptureExportsDefaultFunctionName(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "__init__.py"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	server := Server{Service: app.Service{Capture: fakeCapture{}}, ExportDir: directory}
	handler, err := server.Handler()
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/capture", strings.NewReader(`{"mode":"click"}`))
	request.RemoteAddr = "127.0.0.1:12345"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if _, err := os.Stat(filepath.Join(directory, "click_position.py")); err != nil {
		t.Fatalf("default function was not exported: %v; response %s", err, response.Body.String())
	}
}

func TestValidatePythonModuleRequiresInit(t *testing.T) {
	err := validatePythonModule(t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "__init__.py") {
		t.Fatalf("expected missing __init__.py error, got %v", err)
	}
}

func TestBoxCaptureCanClickAnywhereInside(t *testing.T) {
	handler := testHandler(t)
	request := httptest.NewRequest(http.MethodPost, "/api/capture", strings.NewReader(`{"mode":"box","vary":"all"}`))
	request.RemoteAddr = "127.0.0.1:12345"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	output := response.Body.String()
	for _, expected := range []string{`variation=\"all\"`, "random.randint(left, right)"} {
		if response.Code != http.StatusOK || !strings.Contains(output, expected) {
			t.Fatalf("box capture response missing %q: %d %s", expected, response.Code, output)
		}
	}
}

func TestBoxCaptureCanTargetGridCell(t *testing.T) {
	handler := testHandler(t)
	request := httptest.NewRequest(http.MethodPost, "/api/capture", strings.NewReader(
		`{"mode":"box","name":"click_top_right","gridRows":4,"gridColumns":4,"gridCell":3}`,
	))
	request.RemoteAddr = "127.0.0.1:12345"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	output := response.Body.String()
	for _, expected := range []string{"grid=(4, 4, 3)", "row, column = divmod(cell, columns)"} {
		if response.Code != http.StatusOK || !strings.Contains(output, expected) {
			t.Fatalf("box grid response missing %q: %d %s", expected, response.Code, output)
		}
	}
}

func TestBoxCaptureRejectsOutOfRangeGridCellBeforeCapture(t *testing.T) {
	handler := testHandler(t)
	request := httptest.NewRequest(http.MethodPost, "/api/capture", strings.NewReader(
		`{"mode":"box","gridRows":4,"gridColumns":4,"gridCell":16}`,
	))
	request.RemoteAddr = "127.0.0.1:12345"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "between 0 and 15") {
		t.Fatalf("invalid box grid response = %d %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "top-left") {
		t.Fatal("invalid grid started screen capture")
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
