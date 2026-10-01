package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResponseRecorder(t *testing.T) {
	w := httptest.NewRecorder()
	recorder := newResponseRecorder(w)
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
	recorder := newResponseRecorder(w)
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
	recorder := newResponseRecorder(w)
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
	recorder := newResponseRecorder(w)
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
	recorder := newResponseRecorder(w)
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
