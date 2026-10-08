package http

import (
	"bytes"
	"io"
	"net/http"

	e2enginehttp "github.com/e2engine/instrumentation-go/http"

	"github.com/e2engine/core/execute/call"
	"github.com/e2engine/core/internal/util"
)

func captureRequest(req *http.Request) (call.HTTPRequest, error) {
	header := e2enginehttp.WithoutTestExecutionID(
		req.Header,
	)

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
