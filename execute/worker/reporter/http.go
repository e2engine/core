package reporter

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/e2engine/core/internal/util"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/model/matcher"
)

func (r *Reporter) ensureHTTPResponseSummary() {
	if r.report.Summary.Response == nil {
		r.report.Summary.Response = &model.TestExecutionResponseSummary{}
	}

	if r.report.Summary.Response.HTTP == nil {
		r.report.Summary.Response.HTTP = &model.HTTPResponse{}
	}
}

func (r *Reporter) AddHTTPRequest(req *http.Request) {
	r.ensureNotFinalized()

	request := &model.HTTPRequest{
		Method:  req.Method,
		URL:     req.URL.String(),
		Headers: r.redactHeaders(req.Header),
	}

	if r.testSpec.Request.HTTP.Body != "" {
		switch {
		case !r.cfg.DumpPayload:
			request.Body = dumpDisabled
		default:
			request.Body = r.testSpec.Request.HTTP.Body
		}
	}

	if r.testSpec.Request.HTTP.BodyJSON != "" {
		switch {
		case !r.cfg.DumpPayload:
			request.BodyJSON = dumpDisabled
		default:
			request.BodyJSON = string(util.CompactJSON(
				json.RawMessage(r.testSpec.Request.HTTP.BodyJSON),
			))
		}
	}

	r.report.Summary.Request = &model.TestExecutionRequestSummary{
		HTTP: request,
	}
}

func (r *Reporter) AddHTTPResponseStatus(status int) {
	r.ensureNotFinalized()

	r.ensureHTTPResponseSummary()
	r.report.Summary.Response.HTTP.StatusCode = status
}

func (r *Reporter) AddHTTPResponseHeaders(headers http.Header) {
	r.ensureNotFinalized()

	r.ensureHTTPResponseSummary()
	r.report.Summary.Response.HTTP.Headers = r.redactHeaders(
		selectHeaders(
			headers,
			r.testSpec.Expect.HTTP.Headers,
		),
	)
}

func (r *Reporter) AddHeadersDeviation(
	header string,
	deviation *model.TestExecutionDeviation,
) {
	if deviation == nil {
		return
	}

	if !r.cfg.SkipRedactingHeadersMetadata {
		if _, ok := r.redactedHeaders[strings.ToLower(header)]; ok {
			redactedDeviation := *deviation
			redactedDeviation.Expected = redacted
			redactedDeviation.Actual = redacted

			r.AddDeviation(&redactedDeviation)
			return
		}
	}

	r.AddDeviation(deviation)
}

func (r *Reporter) AddHTTPResponseBody(body []byte) {
	r.ensureNotFinalized()
	r.ensureHTTPResponseSummary()

	switch {
	case len(body) == 0:
	case !r.cfg.DumpPayload:
		r.report.Summary.Response.HTTP.Body = dumpDisabled
	default:
		r.report.Summary.Response.HTTP.Body = string(body)
	}
}

func (r *Reporter) AddHTTPResponseBodyJSON(body []byte) {
	r.ensureNotFinalized()
	r.ensureHTTPResponseSummary()

	switch {
	case len(body) == 0:
	case !r.cfg.DumpPayload:
		r.report.Summary.Response.HTTP.BodyJSON = dumpDisabled
	default:
		r.report.Summary.Response.HTTP.BodyJSON = string(body)
	}
}

func selectHeaders(
	actual http.Header,
	expected map[string][]matcher.String,
) map[string][]string {
	if len(expected) == 0 {
		return nil
	}

	result := make(
		map[string][]string,
		len(expected),
	)

	for k := range expected {
		values := actual.Values(k)
		if len(values) > 0 {
			result[k] = util.CloneStringSlice(values)
		}
	}

	if len(result) == 0 {
		return nil
	}

	return result
}

func (r *Reporter) redactHeaders(headers http.Header) http.Header {
	if r.cfg.SkipRedactingHeadersMetadata {
		return headers.Clone()
	}

	out := make(http.Header, len(headers))
	for k, v := range headers {
		if _, ok := r.redactedHeaders[strings.ToLower(k)]; ok {
			out[k] = []string{redacted}
		} else {
			out[k] = util.CloneStringSlice(v)
		}
	}

	return out
}
