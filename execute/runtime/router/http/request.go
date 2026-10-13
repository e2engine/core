package http

import (
	"bytes"
	"io"
	"net/http"

	"github.com/e2engine/core/execute/call"
	"github.com/e2engine/core/internal/util"
)

func captureRequest(req *http.Request) (call.HTTPRequest, error) {
	rawBody, err := io.ReadAll(req.Body)
	if err != nil {
		return call.HTTPRequest{}, err
	}

	if err := req.Body.Close(); err != nil {
		return call.HTTPRequest{}, err
	}

	req.Body = io.NopCloser(bytes.NewReader(rawBody))

	return call.HTTPRequest{
		Method:  req.Method,
		Path:    req.URL.Path,
		Query:   req.URL.Query().Clone(),
		Headers: req.Header.Clone(),
		Body:    util.CloneBytes(rawBody),
	}, nil
}
