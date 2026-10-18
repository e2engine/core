package http

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"

	e2enginehttp "github.com/e2engine/instrumentation-go/http"
)

func TestRequestCapturer(t *testing.T) {
	const body = `{"value":42}`

	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"http://example.com/path?foo=bar&foo=baz",
		strings.NewReader(body),
	)
	if err != nil {
		t.Fatalf("cannot create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test", "test")
	req.Header = e2enginehttp.WithTestExecutionID(
		req.Header,
		"execution-1",
	)

	capturer := newRequestCapturer(req, 1024)

	// The request body is captured as it is read for forwarding.
	forwarded, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("cannot read forwarded body: %v", err)
	}

	if string(forwarded) != body {
		t.Errorf(
			"forwarded body = %q, want %q",
			forwarded,
			body,
		)
	}

	got := capturer.GetRequest()

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

	if id := e2enginehttp.TestExecutionID(got.Headers); id != "execution-1" {
		t.Errorf(
			"expected execution ID execution-1, got %q",
			id,
		)
	}

	if string(got.Body) != body {
		t.Errorf(
			"unexpected captured body %q",
			string(got.Body),
		)
	}

	if got.BodyTruncated {
		t.Error("expected body not to be truncated")
	}

	if !got.BodyComplete {
		t.Error("expected body to be complete")
	}
}

func TestRequestCapturerBodyLimit(t *testing.T) {
	const limit int64 = 5

	tests := []struct {
		name          string
		chunks        []string
		wantBody      string
		wantTruncated bool
	}{
		{
			name:     "below limit",
			chunks:   []string{"abcd"},
			wantBody: "abcd",
		},
		{
			name:     "exactly at limit",
			chunks:   []string{"abcde"},
			wantBody: "abcde",
		},
		{
			name:          "exceeds in single read",
			chunks:        []string{"abcdef"},
			wantBody:      "abcde",
			wantTruncated: true,
		},
		{
			name:          "exceeds across reads",
			chunks:        []string{"ab", "cdef"},
			wantBody:      "abcde",
			wantTruncated: true,
		},
		{
			name: "empty body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := strings.Join(tt.chunks, "")

			req, err := http.NewRequestWithContext(
				t.Context(),
				http.MethodPost,
				"http://example.com",
				strings.NewReader(payload),
			)
			if err != nil {
				t.Fatalf("cannot create request: %v", err)
			}

			capturer := newRequestCapturer(req, limit)

			var forwarded bytes.Buffer

			// Read the request body in the specified chunks.
			for _, chunk := range tt.chunks {
				p := make([]byte, len(chunk))

				n, err := io.ReadFull(req.Body, p)
				if err != nil {
					t.Fatalf("cannot read request body: %v", err)
				}

				forwarded.Write(p[:n])
			}

			// Forwarding must preserve the complete body.
			if forwarded.String() != payload {
				t.Errorf(
					"forwarded body = %q, want %q",
					forwarded.String(),
					payload,
				)
			}

			got := capturer.GetRequest()

			if string(got.Body) != tt.wantBody {
				t.Errorf(
					"captured body = %q, want %q",
					string(got.Body),
					tt.wantBody,
				)
			}

			if got.BodyTruncated != tt.wantTruncated {
				t.Errorf(
					"BodyTruncated = %t, want %t",
					got.BodyTruncated,
					tt.wantTruncated,
				)
			}

			if !got.BodyComplete {
				t.Error("expected fully consumed body to be complete")
			}
		})
	}
}

func TestRequestCapturerBodyCompletion(t *testing.T) {
	tests := []struct {
		name          string
		body          string
		contentLength int64
		readBytes     int
		readEOF       bool
		wantBody      string
		wantComplete  bool
	}{
		{
			name:          "known content length fully consumed",
			body:          "abcdef",
			contentLength: 6,
			readBytes:     6,
			wantBody:      "abcdef",
			wantComplete:  true,
		},
		{
			name:          "known content length partially consumed",
			body:          "abcdef",
			contentLength: 6,
			readBytes:     3,
			wantBody:      "abc",
		},
		{
			name:          "unknown content length EOF observed",
			body:          "abcdef",
			contentLength: -1,
			readBytes:     6,
			readEOF:       true,
			wantBody:      "abcdef",
			wantComplete:  true,
		},
		{
			name:          "unknown content length EOF not observed",
			body:          "abcdef",
			contentLength: -1,
			readBytes:     6,
			wantBody:      "abcdef",
		},
		{
			name:          "unknown content length partially consumed",
			body:          "abcdef",
			contentLength: -1,
			readBytes:     3,
			wantBody:      "abc",
		},
		{
			name:          "empty body",
			contentLength: 0,
			wantComplete:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(
				t.Context(),
				http.MethodPost,
				"http://example.com",
				io.NopCloser(strings.NewReader(tt.body)),
			)
			if err != nil {
				t.Fatal(err)
			}

			req.ContentLength = tt.contentLength

			capturer := newRequestCapturer(req, 1024)

			if tt.readBytes > 0 {
				buf := make([]byte, tt.readBytes)

				if _, err := io.ReadFull(req.Body, buf); err != nil {
					t.Fatalf("ReadFull() error = %v", err)
				}
			}

			if tt.readEOF {
				buf := make([]byte, 1)

				n, err := req.Body.Read(buf)
				if n != 0 || err != io.EOF {
					t.Fatalf(
						"expected EOF, got n=%d, err=%v",
						n,
						err,
					)
				}
			}

			got := capturer.GetRequest()

			if string(got.Body) != tt.wantBody {
				t.Errorf(
					"captured body = %q, want %q",
					got.Body,
					tt.wantBody,
				)
			}

			if got.BodyComplete != tt.wantComplete {
				t.Errorf(
					"BodyComplete = %t, want %t",
					got.BodyComplete,
					tt.wantComplete,
				)
			}

			if got.BodyTruncated {
				t.Error("unexpected body truncation")
			}
		})
	}
}

func TestRequestCapturerConcurrentAccess(t *testing.T) {
	const body = "abcdefghij"

	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"http://example.com",
		strings.NewReader(body),
	)
	if err != nil {
		t.Fatal(err)
	}

	capturer := newRequestCapturer(req, 5)

	done := make(chan error, 1)

	go func() {
		_, err := io.Copy(io.Discard, req.Body)
		done <- err
	}()

	for i := 0; i < 100; i++ {
		got := capturer.GetRequest()

		if len(got.Body) > 5 {
			t.Errorf(
				"captured body exceeds limit: %d",
				len(got.Body),
			)
		}
	}

	if err := <-done; err != nil {
		t.Fatalf("reading request body failed: %v", err)
	}

	got := capturer.GetRequest()

	if string(got.Body) != "abcde" {
		t.Errorf(
			"captured body = %q, want %q",
			got.Body,
			"abcde",
		)
	}

	if !got.BodyTruncated {
		t.Error("expected body to be truncated")
	}

	if !got.BodyComplete {
		t.Error("expected body to be complete")
	}
}

func TestRequestCapturerTruncatedAndIncomplete(t *testing.T) {
	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"http://example.com",
		strings.NewReader("abcdefghij"),
	)
	if err != nil {
		t.Fatal(err)
	}

	capturer := newRequestCapturer(req, 3)

	buf := make([]byte, 5)

	if _, err := io.ReadFull(req.Body, buf); err != nil {
		t.Fatal(err)
	}

	got := capturer.GetRequest()

	if string(got.Body) != "abc" {
		t.Errorf(
			"captured body = %q, want %q",
			got.Body,
			"abc",
		)
	}

	if !got.BodyTruncated {
		t.Error("expected body to be truncated")
	}

	if got.BodyComplete {
		t.Error("expected body to be incomplete")
	}
}
