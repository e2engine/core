package http

import (
	"net/http"

	"github.com/e2engine/core/execute/call"
	"github.com/e2engine/core/internal/util"
)

type ResponseRecorder struct {
	w          http.ResponseWriter
	statusCode int
	header     http.Header
	body       []byte
}

func newResponseRecorder(w http.ResponseWriter) *ResponseRecorder {
	return &ResponseRecorder{
		w: w,
	}
}

func (r *ResponseRecorder) Header() http.Header {
	return r.w.Header()
}

func (r *ResponseRecorder) WriteHeader(statusCode int) {
	if r.statusCode != 0 {
		return
	}

	r.statusCode = statusCode
	r.header = util.CloneHTTPHeader(r.w.Header())

	r.w.WriteHeader(statusCode)
}

func (r *ResponseRecorder) Write(p []byte) (int, error) {
	if r.statusCode == 0 {
		r.WriteHeader(http.StatusOK)
	}

	n, err := r.w.Write(p)

	if n > 0 {
		r.body = append(r.body, p[:n]...)
	}

	return n, err
}

func (r *ResponseRecorder) GetResponse() call.HTTPResponse {
	statusCode := r.statusCode
	headers := r.header

	if statusCode == 0 {
		statusCode = http.StatusOK
		headers = r.w.Header()
	}

	return call.HTTPResponse{
		StatusCode: statusCode,
		Headers:    util.CloneHTTPHeader(headers),
		Body:       util.CloneBytes(r.body),
	}
}

func (r *ResponseRecorder) Flush() {
	if r.statusCode == 0 {
		r.WriteHeader(http.StatusOK)
	}

	if flusher, ok := r.w.(http.Flusher); ok {
		flusher.Flush()
	}
}

// TODO: preserve http.Hijacker in ResponseRecorder.
