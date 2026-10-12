package model

import (
	"encoding/json"
	"net/url"
	"strings"

	"github.com/ygrebnov/errorc"

	"github.com/e2engine/core/pkg/keys"
)

type HTTPRequest struct {
	Method   string              `json:"method" yaml:"method" validate:"oneof(GET,POST,PUT,DELETE,PATCH,OPTIONS,HEAD)"`
	URL      string              `json:"url" yaml:"url" validate:"min(1)"`
	Query    map[string][]string `json:"query,omitempty" yaml:"query,omitempty"`
	Headers  map[string][]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Body     string              `json:"body,omitempty" yaml:"body,omitempty"`
	BodyJSON string              `json:"body_json,omitempty" yaml:"body_json,omitempty" validate:"omitempty,validjson"`
}

func (h *HTTPRequest) Validate() error {
	u, err := url.Parse(h.URL)
	if err != nil {
		return errorc.With(
			errorc.New("invalid HTTP request URL"),
			errorc.Error(keys.Cause, err),
		)
	}

	if u.Scheme != "http" &&
		u.Scheme != "https" {
		return errorc.New(
			"HTTP request URL scheme must be http or https",
		)
	}

	if strings.TrimSpace(u.Host) == "" {
		return errorc.New(
			"HTTP request URL must contain a host",
		)
	}

	if h.Body != "" && h.BodyJSON != "" {
		return errorc.New(
			"HTTP request cannot have both body and body_json set",
		)
	}

	return nil
}

// HTTPRequestPattern is used to match an HTTP request against a pattern.
// Unlike HTTPRequest, it does not describe an outgoing request.
type HTTPRequestPattern struct {
	Method  string              `json:"method" yaml:"method" validate:"oneof(GET,POST,PUT,DELETE,PATCH,OPTIONS,HEAD)"`
	Path    string              `json:"path" yaml:"path" validate:"min(1)"`
	Query   map[string][]string `json:"query,omitempty" yaml:"query,omitempty"`
	Headers map[string][]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	// Body is pointer to string to distinguish between empty body and no body.
	// body: "" means empty body, while no body field means "will be skipped in matching".
	Body     *string `json:"body,omitempty" yaml:"body,omitempty"`
	BodyJSON *string `json:"body_json,omitempty" yaml:"body_json,omitempty"`
}

func (h *HTTPRequestPattern) Validate() error {
	u, err := url.ParseRequestURI(h.Path)
	if err != nil ||
		u.Path == "" ||
		!strings.HasPrefix(u.Path, "/") ||
		u.RawQuery != "" {
		return errorc.New("invalid HTTP request path")
	}

	if h.Body != nil && h.BodyJSON != nil {
		return errorc.New(
			"HTTP request pattern cannot have both body and body_json set",
		)
	}

	if h.BodyJSON != nil && !json.Valid([]byte(*h.BodyJSON)) {
		return errorc.New(
			"HTTP response body_json is not valid JSON",
		)
	}

	return nil
}

type HTTPResponse struct {
	StatusCode int                 `json:"status_code" yaml:"status_code" validate:"min(100),max(599)"`
	Headers    map[string][]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	// Body is pointer to string to distinguish between empty body and no body.
	// body: "" means empty body, while no body field means "will be skipped in matching".
	Body     *string `json:"body,omitempty" yaml:"body,omitempty"`
	BodyJSON *string `json:"body_json,omitempty" yaml:"body_json,omitempty"`
}

func (h *HTTPResponse) Validate() error {
	if h.Body != nil && h.BodyJSON != nil {
		return errorc.New(
			"HTTP response cannot have both body and body_json set",
		)
	}

	if h.BodyJSON != nil && !json.Valid([]byte(*h.BodyJSON)) {
		return errorc.New(
			"HTTP response body_json is not valid JSON",
		)
	}

	return nil
}
