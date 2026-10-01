package http

import (
	"bytes"
	"io"
	"net/http"

	"github.com/e2engine/core/execute/call"
	"github.com/e2engine/core/internal/util"
	"github.com/e2engine/core/pkg/keys"
)

func captureRequest(req *http.Request) (call.HTTPRequest, error) {
	header := util.CloneHTTPHeader(req.Header)
	header.Del(string(keys.E2EngineTestExecutionID))

	query := util.CloneQuery(req.URL.Query())

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
		Query:   query,
		Headers: header,
		Body:    util.CloneBytes(rawBody),
	}, nil
}
