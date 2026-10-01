package http

import (
	"context"
	"errors"
	"io"
	nethttp "net/http"
	"strings"
	"testing"

	"github.com/e2engine/core/execute"
	"github.com/e2engine/core/execute/call"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/keys"
)

func TestEvaluatorEvaluateResponse(t *testing.T) {
	tests := []struct {
		name string

		status int
		body   string
		expect model.HTTPExpectSpec

		expectedBodyJSON string
		expectedBodyText string
		expected         []model.TestExecutionDeviation
	}{
		{
			name:   "matching status without body expectation",
			status: nethttp.StatusOK,
			body:   `{"ignored":true}`,
			expect: model.HTTPExpectSpec{
				Status: nethttp.StatusOK,
			},
		},
		{
			name:   "status mismatch",
			status: nethttp.StatusNotFound,
			expect: model.HTTPExpectSpec{
				Status: nethttp.StatusOK,
			},
			expected: []model.TestExecutionDeviation{
				{
					Field:    "status_code",
					Expected: "200",
					Actual:   "404",
				},
			},
		},
		{
			name:   "matching JSON body",
			status: nethttp.StatusOK,
			body:   `{"name":"test","count":1}`,
			expect: model.HTTPExpectSpec{
				Status: nethttp.StatusOK,
				Body:   `{"count":1,"name":"test"}`,
			},
			expectedBodyJSON: `{"name":"test","count":1}`,
		},
		{
			name:   "JSON body mismatch",
			status: nethttp.StatusOK,
			body:   `{"name":"actual"}`,
			expect: model.HTTPExpectSpec{
				Status: nethttp.StatusOK,
				Body:   `{"name":"expected"}`,
			},
			expectedBodyJSON: `{"name":"actual"}`,
			expected: []model.TestExecutionDeviation{
				{
					Field:    "body",
					Expected: `{"name":"expected"}`,
					Actual:   `{"name":"actual"}`,
				},
			},
		},
		{
			name:   "response body is not JSON",
			status: nethttp.StatusOK,
			body:   "hello",
			expect: model.HTTPExpectSpec{
				Status: nethttp.StatusOK,
				Body:   `{"message":"hello"}`,
			},
			expectedBodyText: "hello",
			expected: []model.TestExecutionDeviation{
				{
					Field:    "body",
					Expected: `{"message":"hello"}`,
					Actual:   "hello",
					Message:  "response body is not valid JSON",
				},
			},
		},
		{
			name:   "status and body mismatch",
			status: nethttp.StatusBadRequest,
			body:   `{"actual":true}`,
			expect: model.HTTPExpectSpec{
				Status: nethttp.StatusOK,
				Body:   `{"expected":true}`,
			},
			expectedBodyJSON: `{"actual":true}`,
			expected: []model.TestExecutionDeviation{
				{
					Field:    "status_code",
					Expected: "200",
					Actual:   "400",
				},
				{
					Field:    "body",
					Expected: `{"expected":true}`,
					Actual:   `{"actual":true}`,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluator := NewEvaluator()

			response := &nethttp.Response{
				StatusCode: tt.status,
				Body: io.NopCloser(
					strings.NewReader(tt.body),
				),
			}

			result := &execute.TestExecutionResult{
				Summary: &model.TestExecutionSummary{},
			}

			err := evaluator.EvaluateResponse(
				context.Background(),
				response,
				&tt.expect,
				result,
			)
			if err != nil {
				t.Fatalf("EvaluateResponse() error = %v", err)
			}

			if result.Summary.Response.HTTP == nil {
				t.Fatal("expected HTTP response summary")
			}

			actual := result.Summary.Response.HTTP

			if actual.StatusCode != tt.status {
				t.Errorf(
					"expected status code %d, got %d",
					tt.status,
					actual.StatusCode,
				)
			}

			if actual.BodyJSON != tt.expectedBodyJSON {
				t.Errorf(
					"expected JSON body %q, got %q",
					tt.expectedBodyJSON,
					actual.BodyJSON,
				)
			}

			if actual.BodyText != tt.expectedBodyText {
				t.Errorf(
					"expected text body %q, got %q",
					tt.expectedBodyText,
					actual.BodyText,
				)
			}

			assertDeviations(
				t,
				result.Summary.Deviations,
				tt.expected,
			)
		})
	}
}

func TestEvaluatorEvaluateResponseReadError(t *testing.T) {
	expectedErr := errors.New("read error")

	evaluator := NewEvaluator()

	response := &nethttp.Response{
		StatusCode: nethttp.StatusOK,
		Body: &errorReader{
			err: expectedErr,
		},
	}

	result := &execute.TestExecutionResult{
		Summary: &model.TestExecutionSummary{},
	}

	err := evaluator.EvaluateResponse(
		context.Background(),
		response,
		&model.HTTPExpectSpec{
			Status: nethttp.StatusOK,
			Body:   `{"result":"ok"}`,
		},
		result,
	)

	if !errors.Is(err, expectedErr) {
		t.Fatalf(
			"expected error %v, got %v",
			expectedErr,
			err,
		)
	}
}

func TestEvaluatorMatches(t *testing.T) {
	evaluator := NewEvaluator()

	actual := call.Call{
		ServiceID: "service-1",
		HTTP: &call.HTTPCall{
			Request: call.HTTPRequest{
				Method: "POST",
				Path:   "/users",
				Body:   []byte(`{"name":"Yaroslav"}`),
			},
		},
	}

	tests := []struct {
		name     string
		actual   call.Call
		expected model.CallExpectation
		match    bool
	}{
		{
			name:   "matches",
			actual: actual,
			expected: model.CallExpectation{
				ServiceID: "service-1",
				HTTP: &model.HTTPCallExpectation{
					Method: "POST",
					Path:   "/users",
					Body:   `{"name":"Yaroslav"}`,
				},
			},
			match: true,
		},
		{
			name:   "different service",
			actual: actual,
			expected: model.CallExpectation{
				ServiceID: "service-2",
				HTTP: &model.HTTPCallExpectation{
					Method: "POST",
				},
			},
		},
		{
			name: "actual is not HTTP",
			actual: call.Call{
				ServiceID: "service-1",
			},
			expected: model.CallExpectation{
				ServiceID: "service-1",
				HTTP: &model.HTTPCallExpectation{
					Method: "POST",
				},
			},
		},
		{
			name:   "expectation is not HTTP",
			actual: actual,
			expected: model.CallExpectation{
				ServiceID: "service-1",
			},
		},
		{
			name:   "HTTP expectation does not match",
			actual: actual,
			expected: model.CallExpectation{
				ServiceID: "service-1",
				HTTP: &model.HTTPCallExpectation{
					Method: "GET",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match, err := evaluator.Matches(
				context.Background(),
				&model.ServiceSpec{},
				tt.actual,
				tt.expected,
			)
			if err != nil {
				t.Fatalf("Matches() error = %v", err)
			}

			if match != tt.match {
				t.Errorf(
					"expected match %v, got %v",
					tt.match,
					match,
				)
			}
		})
	}
}

func TestHTTPCallMatchesExpectation(t *testing.T) {
	actual := call.HTTPRequest{
		Method: "POST",
		Path:   "/users",
		Query: map[string][]string{
			"active": {"true"},
			"role":   {"admin", "user"},
		},
		Headers: nethttp.Header{
			"Content-Type": {"application/json"},
			"X-Test":       {"one", "two"},
		},
		Body: []byte(`{"name":"Yaroslav","active":true}`),
	}

	tests := []struct {
		name     string
		expected model.HTTPCallExpectation
		match    bool
	}{
		{
			name:  "empty expectation",
			match: true,
		},
		{
			name: "all fields match",
			expected: model.HTTPCallExpectation{
				Method: "POST",
				Path:   "/users",
				Query: map[string][]string{
					"active": {"true"},
					"role":   {"admin", "user"},
				},
				Headers: map[string][]string{
					"Content-Type": {"application/json"},
					"X-Test":       {"one", "two"},
				},
				Body: `{"active":true,"name":"Yaroslav"}`,
			},
			match: true,
		},
		{
			name: "method mismatch",
			expected: model.HTTPCallExpectation{
				Method: "GET",
			},
		},
		{
			name: "path mismatch",
			expected: model.HTTPCallExpectation{
				Path: "/orders",
			},
		},
		{
			name: "query missing",
			expected: model.HTTPCallExpectation{
				Query: map[string][]string{
					"missing": {"value"},
				},
			},
		},
		{
			name: "query mismatch",
			expected: model.HTTPCallExpectation{
				Query: map[string][]string{
					"active": {"false"},
				},
			},
		},
		{
			name: "header missing",
			expected: model.HTTPCallExpectation{
				Headers: map[string][]string{
					"X-Missing": {"value"},
				},
			},
		},
		{
			name: "header mismatch",
			expected: model.HTTPCallExpectation{
				Headers: map[string][]string{
					"Content-Type": {"text/plain"},
				},
			},
		},
		{
			name: "JSON body mismatch",
			expected: model.HTTPCallExpectation{
				Body: `{"name":"Other"}`,
			},
		},
		{
			name: "actual body is invalid JSON",
			expected: model.HTTPCallExpectation{
				Body: `{"name":"Yaroslav"}`,
			},
			match: false,
		},
		{
			name: "query subset matches",
			expected: model.HTTPCallExpectation{
				Query: map[string][]string{
					"active": {"true"},
				},
			},
			match: true,
		},
		{
			name: "headers subset matches",
			expected: model.HTTPCallExpectation{
				Headers: map[string][]string{
					"Content-Type": {"application/json"},
				},
			},
			match: true,
		},
		{
			name: "query values match regardless of order",
			expected: model.HTTPCallExpectation{
				Query: map[string][]string{
					"role": {"user", "admin"},
				},
			},
			match: true,
		},
		{
			name: "header values match regardless of order",
			expected: model.HTTPCallExpectation{
				Headers: map[string][]string{
					"X-Test": {"two", "one"},
				},
			},
			match: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := actual

			if tt.name == "actual body is invalid JSON" {
				request.Body = []byte("not-json")
			}

			match := httpCallMatchesExpectation(
				request,
				&tt.expected,
			)

			if match != tt.match {
				t.Errorf(
					"expected match %v, got %v",
					tt.match,
					match,
				)
			}
		})
	}
}

func TestEvaluatorDeviation(t *testing.T) {
	evaluator := NewEvaluator()

	service := &model.ServiceSpec{
		ID: "service-1",
	}

	tests := []struct {
		name     string
		actual   call.Call
		expected *model.TestExecutionDeviation
	}{
		{
			name: "not HTTP",
			actual: call.Call{
				ServiceID: "service-1",
			},
		},
		{
			name: "no mock miss",
			actual: call.Call{
				ServiceID: "service-1",
				HTTP: &call.HTTPCall{
					Request: call.HTTPRequest{
						Method: "GET",
						Path:   "/users",
					},
					Response: call.HTTPResponse{
						Headers: nethttp.Header{},
					},
				},
			},
		},
		{
			name: "mock miss",
			actual: call.Call{
				ServiceID: "service-1",
				HTTP: &call.HTTPCall{
					Request: call.HTTPRequest{
						Method: "POST",
						Path:   "/users",
					},
					Response: call.HTTPResponse{
						Headers: func() nethttp.Header {
							headers := make(nethttp.Header)
							headers.Set(string(keys.E2EngineMockMissHeader), "true")
							return headers
						}(),
					},
				},
			},
			expected: &model.TestExecutionDeviation{
				Field:   "calls.service-1",
				Actual:  "POST /users",
				Message: "mocked service received a request that did not match any fixture",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := evaluator.Deviation(
				service,
				tt.actual,
			)

			if tt.expected == nil {
				if actual != nil {
					t.Fatalf(
						"expected no deviation, got %+v",
						actual,
					)
				}

				return
			}

			if actual == nil {
				t.Fatal("expected deviation")
			}

			if *actual != *tt.expected {
				t.Errorf(
					"expected deviation %+v, got %+v",
					*tt.expected,
					*actual,
				)
			}
		})
	}
}

type errorReader struct {
	err error
}

func (r *errorReader) Read(
	_ []byte,
) (int, error) {
	return 0, r.err
}

func (r *errorReader) Close() error {
	return nil
}

func assertDeviations(
	t *testing.T,
	actual []model.TestExecutionDeviation,
	expected []model.TestExecutionDeviation,
) {
	t.Helper()

	if len(actual) != len(expected) {
		t.Fatalf(
			"expected %d deviations, got %d: %+v",
			len(expected),
			len(actual),
			actual,
		)
	}

	for i := range expected {
		if actual[i] != expected[i] {
			t.Errorf(
				"deviation %d: expected %+v, got %+v",
				i,
				expected[i],
				actual[i],
			)
		}
	}
}
