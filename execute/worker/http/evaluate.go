package http

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/e2engine/core/execute/call"
	"github.com/e2engine/core/execute/match"
	"github.com/e2engine/core/execute/worker/evaluator"
	"github.com/e2engine/core/internal/util"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/model/matcher"
	"github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/keys"
)

var _ evaluator.CallEvaluator = (*Evaluator)(nil)

type Evaluator struct {
	cfg      *EvaluatorConfig
	reporter executionReporter
}

type EvaluatorConfig struct {
	MaxResponseBodySize int64 `json:"max_response_body_size" yaml:"max_response_body_size" default:"1048576" validate:"min(1),max(67108864)"`
}

func NewEvaluator(
	cfg *EvaluatorConfig,
	reporter executionReporter,
) *Evaluator {
	return &Evaluator{
		cfg:      cfg,
		reporter: reporter,
	}
}

func (e *Evaluator) EvaluateResponse(
	resp *http.Response,
	expect *model.HTTPResponsePattern,
) error {
	e.evaluateStatusCode(expect.StatusCode, resp.StatusCode)

	e.evaluateHeaders(expect.Headers, resp.Header)

	actualBody, err := io.ReadAll(
		io.LimitReader(resp.Body, e.cfg.MaxResponseBodySize+1),
	)
	if err != nil {
		return err
	}

	if int64(len(actualBody)) > e.cfg.MaxResponseBodySize {
		return errors.ErrHTTPResponseBodyTooLarge
	}

	if expect.Body == nil && expect.BodyJSON == nil {
		// nil means "do not evaluate body"
		// we still need to report the body we received
		e.reporter.AddHTTPResponseBody(actualBody)
		return nil
	}

	if expect.Body != nil {
		e.reporter.AddHTTPResponseBody(actualBody)

		if !match.IsStringMatched(actualBody, expect.Body) {
			e.reporter.AddPayloadDeviation(&model.PayloadDeviation{
				Field:    string(keys.FieldBody),
				Expected: expect.Body,
				Actual:   actualBody,
			})
		}

		return nil
	}

	if !json.Valid(actualBody) {
		e.reporter.AddHTTPResponseBodyJSON(actualBody)

		e.reporter.AddPayloadDeviation(&model.PayloadDeviation{
			Field:    string(keys.FieldBodyJSON),
			Expected: expect.BodyJSON,
			Actual:   actualBody,
			Message:  "response body is not valid JSON",
		})

		return nil
	}

	actualJSON := util.CompactJSON(actualBody)

	e.reporter.AddHTTPResponseBodyJSON(actualJSON)

	matched, err := match.IsJSONMatched(
		actualBody,
		expect.BodyJSON,
	)
	if err != nil {
		// Expected JSON has already been validated.
		return err
	}

	if !matched {
		e.reporter.AddPayloadDeviation(&model.PayloadDeviation{
			Field:    string(keys.FieldBodyJSON),
			Expected: expect.BodyJSON,
			Actual:   actualJSON,
		})
	}

	return nil
}

func (e *Evaluator) evaluateStatusCode(expected, actual int) {
	e.reporter.AddHTTPResponseStatus(actual)

	if actual != expected {
		e.reporter.AddDeviation(&model.TestExecutionDeviation{
			Field:    string(keys.FieldStatusCode),
			Expected: strconv.Itoa(expected),
			Actual:   strconv.Itoa(actual),
		})
	}
}

func (e *Evaluator) evaluateHeaders(
	expected map[string][]matcher.String,
	actual http.Header,
) {
	e.reporter.AddHTTPResponseHeaders(actual)

	for key, expectedValues := range expected {
		canonicalKey := http.CanonicalHeaderKey(key)
		actualValues, exists := actual[canonicalKey]

		if match.IsStringSliceMatched(
			actualValues,
			exists,
			expectedValues,
		) {
			continue
		}

		sortedActual := slices.Clone(actualValues)
		slices.Sort(sortedActual)

		expectedStrings := make([]string, len(expectedValues))
		for i, v := range expectedValues {
			expectedStrings[i] = v.String()
		}
		slices.Sort(expectedStrings)

		e.reporter.AddHeadersDeviation(
			key,
			&model.TestExecutionDeviation{
				Field:    "headers." + key,
				Expected: strings.Join(expectedStrings, ","),
				Actual:   strings.Join(sortedActual, ","),
			},
		)
	}
}

func (e *Evaluator) Matches(
	_ context.Context,
	_ *model.ServiceSpec,
	actual call.Call,
	expected model.ServiceExpectedCalls,
) (bool, error) {
	if actual.ServiceID != expected.ServiceID {
		return false, nil
	}

	if actual.HTTP == nil || expected.HTTP == nil {
		return false, nil
	}

	return httpCallMatchesExpectation(
		&actual.HTTP.Request,
		expected.HTTP,
	)
}

func httpCallMatchesExpectation(
	actual *call.HTTPRequest,
	expected *model.HTTPRequestPattern,
) (bool, error) {
	if expected.Method != "" && actual.Method != expected.Method {
		return false, nil
	}

	if expected.Path != "" && actual.Path != expected.Path {
		return false, nil
	}

	for key, expectedValues := range expected.Query {
		actualValues, ok := actual.Query[key]
		if !ok || !util.IsStringSlicesEqual(actualValues, expectedValues) {
			return false, nil
		}
	}

	for key, expectedValues := range expected.Headers {
		canonicalKey := http.CanonicalHeaderKey(key)
		actualValues, exists := actual.Headers[canonicalKey]

		if !match.IsStringSliceMatched(
			actualValues,
			exists,
			expectedValues,
		) {
			return false, nil
		}
	}

	if expected.Body != nil || expected.BodyJSON != nil {
		if actual.BodyTruncated {
			return false, errors.ErrHTTPRecordedRequestBodyTruncated
		}

		if !actual.BodyComplete {
			return false, errors.ErrHTTPRecordedRequestBodyIncomplete
		}
	}

	if expected.Body != nil &&
		!match.IsStringMatched(actual.Body, expected.Body) {
		return false, nil
	}

	if expected.BodyJSON != nil {
		matched, err := match.IsJSONMatched(
			actual.Body,
			expected.BodyJSON,
		)
		if err != nil || !matched {
			return false, nil
		}
	}

	return true, nil
}

func (e *Evaluator) Deviation(
	service *model.ServiceSpec,
	actual call.Call,
) *model.TestExecutionDeviation {
	if actual.HTTP == nil {
		return nil
	}

	if actual.HTTP.Response.Headers.Get(
		string(keys.E2EngineMockMissHeader),
	) != "true" {
		return nil
	}

	return &model.TestExecutionDeviation{
		Field: "calls." + service.ID,
		Actual: actual.HTTP.Request.Method +
			" " +
			actual.HTTP.Request.Path,
		Message: "mocked service received a request that did not match any fixture",
	}
}
