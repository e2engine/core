package http

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/e2engine/core/execute/call"
	"github.com/e2engine/core/execute/worker/evaluator"
	"github.com/e2engine/core/internal/util"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/keys"
)

var _ evaluator.CallEvaluator = (*Evaluator)(nil)

type Evaluator struct {
	reporter executionReporter
}

func NewEvaluator(
	reporter executionReporter,
) *Evaluator {
	return &Evaluator{
		reporter: reporter,
	}
}

func (e *Evaluator) EvaluateResponse(
	resp *http.Response,
	expect *model.HTTPResponse,
) error {
	e.reporter.AddHTTPResponseStatus(resp.StatusCode)
	e.reporter.AddHTTPResponseHeaders(resp.Header)

	if resp.StatusCode != expect.StatusCode {
		e.reporter.AddDeviation(&model.TestExecutionDeviation{
			Field:    string(keys.FieldStatusCode),
			Expected: strconv.Itoa(expect.StatusCode),
			Actual:   strconv.Itoa(resp.StatusCode),
		})
	}

	for key, expectedValues := range expect.Headers {
		actualValues := resp.Header.Values(key)

		if len(actualValues) > 0 &&
			util.IsStringSlicesEqual(actualValues, expectedValues) {
			continue
		}

		e.reporter.AddDeviation(&model.TestExecutionDeviation{
			Field:    "headers." + key,
			Expected: strings.Join(expectedValues, ","),
			Actual:   strings.Join(actualValues, ","),
		})
	}

	if expect.Body == nil && expect.BodyJSON == nil {
		// nil means "do not evaluate body"
		return nil
	}

	actualBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if expect.Body != nil {
		e.reporter.AddHTTPResponseBody(string(actualBody))

		if string(actualBody) != *expect.Body {
			e.reporter.AddPayloadDeviation(&model.TestExecutionDeviation{
				Field:    string(keys.FieldBody),
				Expected: *expect.Body,
				Actual:   string(actualBody),
			})
		}

		return nil
	}

	if !json.Valid(actualBody) {
		e.reporter.AddHTTPResponseBody(string(actualBody))

		e.reporter.AddPayloadDeviation(&model.TestExecutionDeviation{
			Field:    string(keys.FieldBodyJSON),
			Expected: util.CompactJSON([]byte(*expect.BodyJSON)),
			Actual:   string(actualBody),
			Message:  "response body is not valid JSON",
		})

		return nil
	}

	e.reporter.AddHTTPResponseBodyJSON(util.CompactJSON(actualBody))

	equal, err := util.IsJSONEqual(
		[]byte(*expect.BodyJSON),
		actualBody,
	)
	if err != nil {
		// Expected JSON should already have been validated.
		return err
	}

	if !equal {
		e.reporter.AddPayloadDeviation(&model.TestExecutionDeviation{
			Field: string(keys.FieldBodyJSON),
			Expected: util.CompactJSON(
				[]byte(*expect.BodyJSON),
			),
			Actual: util.CompactJSON(actualBody),
		})
	}

	return nil
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
		actual.HTTP.Request,
		expected.HTTP,
	), nil
}

func httpCallMatchesExpectation(
	actual call.HTTPRequest,
	expected *model.HTTPRequestPattern,
) bool {
	if expected.Method != "" && actual.Method != expected.Method {
		return false
	}

	if expected.Path != "" && actual.Path != expected.Path {
		return false
	}

	for key, expectedValues := range expected.Query {
		actualValues, ok := actual.Query[key]
		if !ok || !util.IsStringSlicesEqual(actualValues, expectedValues) {
			return false
		}
	}

	for key, expectedValues := range expected.Headers {
		actualValues := actual.Headers.Values(key)

		if len(actualValues) == 0 ||
			!util.IsStringSlicesEqual(actualValues, expectedValues) {
			return false
		}
	}

	if expected.Body != nil && string(actual.Body) != *expected.Body {
		return false
	}

	if expected.BodyJSON != nil {
		equal, err := util.IsJSONEqual(
			[]byte(*expected.BodyJSON),
			actual.Body,
		)
		if err != nil || !equal {
			return false
		}
	}

	return true
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
