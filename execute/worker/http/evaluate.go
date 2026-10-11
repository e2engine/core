package http

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/e2engine/core/execute"
	"github.com/e2engine/core/execute/call"
	"github.com/e2engine/core/execute/evaluate"
	"github.com/e2engine/core/internal/util"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/keys"
)

var _ evaluate.CallEvaluator = (*Evaluator)(nil)

type Evaluator struct{}

func NewEvaluator() *Evaluator {
	return &Evaluator{}
}

func (e *Evaluator) EvaluateResponse(
	_ context.Context,
	resp *http.Response,
	expect *model.HTTPResponse,
	result *execute.TestExecutionResult,
) error {
	result.Summary.Response = &model.TestExecutionResponseSummary{
		HTTP: &model.HTTPResponse{
			StatusCode: resp.StatusCode,
			Headers:    selectHeaders(resp.Header, expect.Headers),
		},
	}

	if resp.StatusCode != expect.StatusCode {
		result.Summary.Deviations = append(
			result.Summary.Deviations,
			model.TestExecutionDeviation{
				Field:    "status_code",
				Expected: strconv.Itoa(expect.StatusCode),
				Actual:   strconv.Itoa(resp.StatusCode),
			},
		)
	}

	for key, expectedValues := range expect.Headers {
		actualValues := resp.Header.Values(key)

		if len(actualValues) > 0 &&
			util.IsStringSlicesEqual(actualValues, expectedValues) {
			continue
		}

		result.Summary.Deviations = append(
			result.Summary.Deviations,
			model.TestExecutionDeviation{
				Field:    "headers." + key,
				Expected: strings.Join(expectedValues, ","),
				Actual:   strings.Join(actualValues, ","),
			},
		)
	}

	if expect.Body == "" && expect.BodyJSON == "" {
		return nil
	}

	actualBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if expect.Body != "" {
		result.Summary.Response.HTTP.Body = string(actualBody)

		if string(actualBody) != expect.Body {
			result.Summary.Deviations = append(
				result.Summary.Deviations,
				model.TestExecutionDeviation{
					Field:    string(keys.FieldBody),
					Expected: expect.Body,
					Actual:   string(actualBody),
				},
			)
		}

		return nil
	}

	if !json.Valid(actualBody) {
		result.Summary.Response.HTTP.Body = string(actualBody)

		result.Summary.Deviations = append(
			result.Summary.Deviations,
			model.TestExecutionDeviation{
				Field:    string(keys.FieldBodyJSON),
				Expected: util.CompactJSON([]byte(expect.BodyJSON)),
				Actual:   string(actualBody),
				Message:  "response body is not valid JSON",
			},
		)

		return nil
	}

	result.Summary.Response.HTTP.BodyJSON =
		util.CompactJSON(actualBody)

	equal, err := util.IsJSONEqual(
		[]byte(expect.BodyJSON),
		actualBody,
	)
	if err != nil {
		// Expected JSON should already have been validated.
		return err
	}

	if !equal {
		result.Summary.Deviations = append(
			result.Summary.Deviations,
			model.TestExecutionDeviation{
				Field: string(keys.FieldBodyJSON),
				Expected: util.CompactJSON(
					[]byte(expect.BodyJSON),
				),
				Actual: util.CompactJSON(actualBody),
			},
		)
	}

	return nil
}

func selectHeaders(
	actual http.Header,
	expected map[string][]string,
) map[string][]string {
	if len(expected) == 0 {
		return nil
	}

	result := make(
		map[string][]string,
		len(expected),
	)

	for key := range expected {
		values := actual.Values(key)
		if len(values) > 0 {
			result[key] = util.CloneStringSlice(values)
		}
	}

	if len(result) == 0 {
		return nil
	}

	return result
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

	if expected.Body != "" && string(actual.Body) != expected.Body {
		return false
	}

	if expected.BodyJSON != "" {
		equal, err := util.IsJSONEqual(
			[]byte(expected.BodyJSON),
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
