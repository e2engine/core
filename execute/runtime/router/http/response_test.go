package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResponseRecorder(t *testing.T) {
	w := httptest.NewRecorder()
	recorder := newResponseRecorder(w, 1024)
	recorder.Header().Set("X-Test", "value")
	recorder.WriteHeader(http.StatusCreated)
	if _, err := recorder.Write([]byte("response")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	got := recorder.GetResponse()
	if got.StatusCode != http.StatusCreated {
		t.Errorf(
			"expected status %d, got %d",
			http.StatusCreated,
			got.StatusCode,
		)
	}
	if got.Headers.Get("X-Test") != "value" {
		t.Errorf(
			"expected X-Test header, got %q",
			got.Headers.Get("X-Test"),
		)
	}
	if string(got.Body) != "response" {
		t.Errorf(
			"expected body response, got %q",
			string(got.Body),
		)
	}
}

func TestResponseRecorderImplicitOK(t *testing.T) {
	w := httptest.NewRecorder()
	recorder := newResponseRecorder(w, 1024)
	if _, err := recorder.Write([]byte("response")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	got := recorder.GetResponse()
	if got.StatusCode != http.StatusOK {
		t.Errorf(
			"expected status %d, got %d",
			http.StatusOK,
			got.StatusCode,
		)
	}
	if string(got.Body) != "response" {
		t.Errorf(
			"expected body response, got %q",
			string(got.Body),
		)
	}
}

func TestResponseRecorderEmptyResponse(t *testing.T) {
	w := httptest.NewRecorder()
	recorder := newResponseRecorder(w, 1024)
	recorder.Header().Set("X-Test", "value")
	got := recorder.GetResponse()
	if got.StatusCode != http.StatusOK {
		t.Errorf(
			"expected status %d, got %d",
			http.StatusOK,
			got.StatusCode,
		)
	}
	if got.Headers.Get("X-Test") != "value" {
		t.Errorf(
			"expected X-Test header, got %q",
			got.Headers.Get("X-Test"),
		)
	}
	if len(got.Body) != 0 {
		t.Errorf(
			"expected empty body, got %q",
			string(got.Body),
		)
	}
}

func TestResponseRecorderFirstWriteHeaderWins(t *testing.T) {
	w := httptest.NewRecorder()
	recorder := newResponseRecorder(w, 1024)
	recorder.WriteHeader(http.StatusCreated)
	recorder.WriteHeader(http.StatusInternalServerError)
	got := recorder.GetResponse()
	if got.StatusCode != http.StatusCreated {
		t.Errorf(
			"expected status %d, got %d",
			http.StatusCreated,
			got.StatusCode,
		)
	}
}

func TestResponseRecorderFlush(t *testing.T) {
	w := httptest.NewRecorder()
	recorder := newResponseRecorder(w, 1024)
	recorder.Flush()
	got := recorder.GetResponse()
	if got.StatusCode != http.StatusOK {
		t.Errorf(
			"expected status %d, got %d",
			http.StatusOK,
			got.StatusCode,
		)
	}
	if !w.Flushed {
		t.Fatal("expected underlying writer to be flushed")
	}
}

func TestResponseRecorderBodySizeLimit(t *testing.T) {
	tests := []struct {
		name          string
		writes        []string
		wantBody      string
		wantTruncated bool
	}{
		{
			name:     "below limit",
			writes:   []string{"abcd"},
			wantBody: "abcd",
		},
		{
			name:     "exactly at limit",
			writes:   []string{"abcde"},
			wantBody: "abcde",
		},
		{
			name:          "exceeds in one write",
			writes:        []string{"abcdef"},
			wantBody:      "abcde",
			wantTruncated: true,
		},
		{
			name:          "exceeds across writes",
			writes:        []string{"ab", "cdef"},
			wantBody:      "abcde",
			wantTruncated: true,
		},
		{
			name:     "empty",
			wantBody: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := newResponseRecorder(w, 5)

			for _, part := range tt.writes {
				if _, err := r.Write([]byte(part)); err != nil {
					t.Fatal(err)
				}
			}

			got := r.GetResponse()

			if string(got.Body) != tt.wantBody ||
				got.BodyTruncated != tt.wantTruncated {
				t.Errorf(
					"recorded body = %q truncated = %t; want %q truncated = %t",
					got.Body,
					got.BodyTruncated,
					tt.wantBody,
					tt.wantTruncated,
				)
			}

			// The original response must be forwarded completely.
			if want := strings.Join(tt.writes, ""); w.Body.String() != want {
				t.Errorf(
					"forwarded body = %q, want %q",
					w.Body.String(),
					want,
				)
			}
		})
	}
}
