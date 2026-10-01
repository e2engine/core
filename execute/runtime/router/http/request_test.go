package http

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/e2engine/core/pkg/keys"
)

func TestCaptureRequest(t *testing.T) {
	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"http://example.com/path?foo=bar&foo=baz",
		strings.NewReader(`{"value":42}`),
	)
	if err != nil {
		t.Fatalf("cannot create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test", "test")
	req.Header.Set(
		string(keys.E2EngineTestExecutionID),
		"execution-1",
	)

	got, err := captureRequest(req)
	if err != nil {
		t.Fatalf("captureRequest() error = %v", err)
	}

	if got.Method != http.MethodPost {
		t.Errorf("expected method POST, got %s", got.Method)
	}

	if got.Path != "/path" {
		t.Errorf("expected path /path, got %s", got.Path)
	}

	values := got.Query["foo"]
	if len(values) != 2 ||
		values[0] != "bar" ||
		values[1] != "baz" {
		t.Errorf("unexpected query: %#v", got.Query)
	}

	if got.Headers.Get("Content-Type") != "application/json" {
		t.Errorf(
			"expected Content-Type application/json, got %q",
			got.Headers.Get("Content-Type"),
		)
	}

	if got.Headers.Get("X-Test") != "test" {
		t.Errorf(
			"expected X-Test header, got %q",
			got.Headers.Get("X-Test"),
		)
	}

	if got.Headers.Get(
		string(keys.E2EngineTestExecutionID),
	) != "" {
		t.Fatal(
			"expected execution ID header to be removed",
		)
	}

	if string(got.Body) != `{"value":42}` {
		t.Errorf(
			"unexpected body %q",
			string(got.Body),
		)
	}

	// captureRequest must restore the body because the reverse proxy
	// still needs to read it.
	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("ReadAll(request body) error = %v", err)
	}

	if string(body) != `{"value":42}` {
		t.Errorf(
			"expected restored body, got %q",
			string(body),
		)
	}
}
