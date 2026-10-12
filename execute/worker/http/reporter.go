package http

import (
	"net/http"

	"github.com/e2engine/core/model"
)

type executionReporter interface {
	AddDeviation(deviation *model.TestExecutionDeviation)
	AddPayloadDeviation(deviation *model.TestExecutionDeviation)
	AddHTTPRequest(req *http.Request)
	AddHTTPResponseStatus(status int)
	AddHTTPResponseHeaders(headers http.Header)
	AddHTTPResponseBody(body string)
	AddHTTPResponseBodyJSON(body string)
}

//go:generate go run -mod=mod go.uber.org/mock/mockgen -source=reporter.go -destination=mock_reporter_test.go -package=http
