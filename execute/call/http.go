package call

import (
	"net/http"
)

type HTTPCall struct {
	Request  HTTPRequest
	Response HTTPResponse
}

type HTTPRequest struct {
	Method        string
	Path          string
	Query         map[string][]string
	Headers       http.Header
	Body          []byte
	BodyTruncated bool
	BodyComplete  bool
}

type HTTPResponse struct {
	StatusCode    int
	Headers       http.Header
	Body          []byte
	BodyTruncated bool
}
