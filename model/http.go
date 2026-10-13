package model

import (
	"encoding/json"
	"net/url"
	"strings"

	"github.com/ygrebnov/errorc"
	modellib "github.com/ygrebnov/model"

	"github.com/e2engine/core/model/matcher"
	"github.com/e2engine/core/pkg/errors"
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
	u, err := url.ParseRequestURI(h.URL)
	if err != nil {
		return errorc.With(
			errorc.New("invalid HTTP request URL"),
			errorc.Error(keys.Cause, err),
		)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
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
	Method  string                      `json:"method" yaml:"method" validate:"oneof(GET,POST,PUT,DELETE,PATCH,OPTIONS,HEAD)"`
	Path    string                      `json:"path" yaml:"path" validate:"min(1)"`
	Query   map[string][]string         `json:"query,omitempty" yaml:"query,omitempty"`
	Headers map[string][]matcher.String `json:"headers,omitempty" yaml:"headers,omitempty" validateElem:"stringmatchers"`
	// Body is nil when body matching should be skipped.
	// A non-nil matcher defines how the body is matched, including explicit
	// matching of an empty body.
	Body *matcher.String `json:"body,omitempty" yaml:"body,omitempty" validate:"stringmatcher"`
	// BodyJSON matches the response body as JSON. For the equals operator,
	// Value must contain valid JSON and comparison is JSON-semantic.
	BodyJSON *matcher.String `json:"body_json,omitempty" yaml:"body_json,omitempty" validate:"stringmatcher,validjson"`
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

	return nil
}

// HTTPResponse describes HTTP response observed during execution or a Fixture Then response.
// Validation is needed for Fixture.Then validation.
type HTTPResponse struct {
	StatusCode int                 `json:"status_code" yaml:"status_code" validate:"min(100),max(599)"`
	Headers    map[string][]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Body       string              `json:"body,omitempty" yaml:"body,omitempty"`
	BodyJSON   string              `json:"body_json,omitempty" yaml:"body_json,omitempty" validate:"omitempty,validjson"`
}

func (h *HTTPResponse) Validate() error {
	if h.Body != "" && h.BodyJSON != "" {
		return errorc.New(
			"HTTP response cannot have both body and body_json set",
		)
	}

	return nil
}

// HTTPResponsePattern is used to match an HTTP response against a pattern.
type HTTPResponsePattern struct {
	StatusCode int                         `json:"status_code" yaml:"status_code" validate:"min(100),max(599)"`
	Headers    map[string][]matcher.String `json:"headers,omitempty" yaml:"headers,omitempty" validateElem:"stringmatchers"`
	// Body is nil when body matching should be skipped.
	// A non-nil matcher defines how the body is matched, including explicit
	// matching of an empty body.
	Body *matcher.String `json:"body,omitempty" yaml:"body,omitempty" validate:"stringmatcher"`
	// BodyJSON matches the response body as JSON. For the equals operator,
	// Value must contain valid JSON and comparison is JSON-semantic.
	BodyJSON *matcher.String `json:"body_json,omitempty" yaml:"body_json,omitempty" validate:"stringmatcher,validjson"`
}

func (h *HTTPResponsePattern) Validate() error {
	if h.Body != nil && h.BodyJSON != nil {
		return errorc.New(
			"HTTP response pattern cannot have both body and body_json set",
		)
	}

	return nil
}

func getStringMatcherValidJSONRule() (modellib.Rule, error) {
	return modellib.NewRule[*matcher.String]("validjson", func(value *matcher.String, _ ...string) error {
		if value == nil {
			return nil
		}
		if value.Operator == matcher.StringMatchOperatorEquals {
			if !json.Valid([]byte(value.Value)) {
				return errorc.With(
					errors.ErrInvalidSpec,
					errorc.String(
						keys.Validation,
						"value must contain valid JSON",
					),
				)
			}
		}

		return nil
	})
}
