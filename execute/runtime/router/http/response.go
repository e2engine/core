package http

import (
	"net/http"
	"slices"

	"github.com/e2engine/core/execute/call"
)

type ResponseRecorder struct {
	w             http.ResponseWriter
	statusCode    int
	header        http.Header
	body          []byte
	maxBodySize   int64
	bodyTruncated bool
}

func newResponseRecorder(
	w http.ResponseWriter,
	maxBodySize int64,
) *ResponseRecorder {
	return &ResponseRecorder{
		w:           w,
		maxBodySize: maxBodySize,
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
	r.header = r.w.Header().Clone()

	r.w.WriteHeader(statusCode)
}

func (r *ResponseRecorder) Write(p []byte) (int, error) {
	if r.statusCode == 0 {
		r.WriteHeader(http.StatusOK)
	}

	n, err := r.w.Write(p)

	if n > 0 {
		remaining := r.maxBodySize - int64(len(r.body))

		if remaining > 0 {
			captureSize := min(n, int(remaining))
			r.body = append(r.body, p[:captureSize]...)

			if captureSize < n {
				r.bodyTruncated = true
			}
		} else {
			r.bodyTruncated = true
		}
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
		StatusCode:    statusCode,
		Headers:       headers.Clone(),
		Body:          slices.Clone(r.body),
		BodyTruncated: r.bodyTruncated,
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

// TODO: support HTTP connection upgrades (http.Hijacker).
// Hijacked connections require explicit lifecycle management because
// http.Server.Shutdown and Close do not terminate them.
// They also bypass the current request/response recording limits.
