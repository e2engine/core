package http

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

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
	expect *model.HTTPExpectSpec,
	result *execute.TestExecutionResult,
) error {
	result.Summary.Response = &model.TestExecutionResponseSummary{
		HTTP: &model.TestExecutionHTTPResponseSummary{StatusCode: resp.StatusCode},
	}

	if resp.StatusCode != expect.Status {
		result.Summary.Deviations = append(
			result.Summary.Deviations,
			model.TestExecutionDeviation{
				Field:    "status_code",
				Expected: strconv.Itoa(expect.Status),
				Actual:   strconv.Itoa(resp.StatusCode),
			},
		)
	}

	if expect.Body != "" {
		actualBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		if json.Valid(actualBody) {
			result.Summary.Response.HTTP.BodyJSON = util.CompactJSON(actualBody)

			equal, err := util.IsJSONEqual(
				[]byte(expect.Body),
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
						Field:    string(keys.FieldBody),
						Expected: util.CompactJSON([]byte(expect.Body)),
						Actual:   util.CompactJSON(actualBody),
					},
				)
			}

			return nil
		}

		result.Summary.Response.HTTP.BodyText = string(actualBody)

		result.Summary.Deviations = append(
			result.Summary.Deviations,
			model.TestExecutionDeviation{
				Field:    string(keys.FieldBody),
				Expected: util.CompactJSON([]byte(expect.Body)),
				Actual:   string(actualBody),
				Message:  "response body is not valid JSON",
			},
		)

		return nil
	}

	return nil
}

func (e *Evaluator) Matches(
	_ context.Context,
	_ *model.ServiceSpec,
	actual call.Call,
	expected model.CallExpectation,
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
	expected *model.HTTPCallExpectation,
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
		actualValues, ok := actual.Headers[key]
		if !ok || !util.IsStringSlicesEqual(actualValues, expectedValues) {
			return false
		}
	}

	if expected.Body != "" {
		equal, err := util.IsJSONEqual(
			[]byte(expected.Body),
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
