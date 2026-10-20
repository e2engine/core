package http

import (
	"io"
	"net/http"
	"slices"
	"sync"

	"github.com/e2engine/core/execute/call"
)

type requestCapturer struct {
	request call.HTTPRequest
	body    *capturingReadCloser
}

func newRequestCapturer(req *http.Request, maxBodySize int64) *requestCapturer {
	capture := &requestCapturer{
		request: call.HTTPRequest{
			Method:  req.Method,
			Path:    req.URL.Path,
			Query:   req.URL.Query().Clone(),
			Headers: req.Header.Clone(),
		},
	}

	if req.Body != nil {
		capture.body = &capturingReadCloser{
			ReadCloser:    req.Body,
			maxBodySize:   maxBodySize,
			contentLength: req.ContentLength,
			complete:      req.ContentLength == 0,
		}
		req.Body = capture.body
	}

	return capture
}

func (c *requestCapturer) GetRequest() call.HTTPRequest {
	request := c.request

	if c.body != nil {
		c.body.mu.Lock()

		request.Body = slices.Clone(c.body.body)
		request.BodyTruncated = c.body.truncated
		request.BodyComplete = c.body.complete

		c.body.mu.Unlock()
	} else {
		request.BodyComplete = true
	}

	return request
}

type capturingReadCloser struct {
	mu sync.Mutex

	io.ReadCloser

	maxBodySize int64
	body        []byte
	truncated   bool
	complete    bool

	contentLength int64
	bytesRead     int64
}

func (r *capturingReadCloser) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)

	r.mu.Lock()
	defer r.mu.Unlock()

	r.bytesRead += int64(n)

	if n > 0 {
		remaining := r.maxBodySize - int64(len(r.body))

		if remaining > 0 {
			captureSize := min(n, int(remaining))
			r.body = append(r.body, p[:captureSize]...)

			if captureSize < n {
				r.truncated = true
			}
		} else {
			r.truncated = true
		}
	}

	if err == io.EOF ||
		(r.contentLength >= 0 && r.bytesRead == r.contentLength) {
		r.complete = true
	}

	return n, err
}
