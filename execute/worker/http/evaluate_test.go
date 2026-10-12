package http

import (
	"context"
	"errors"
	"io"
	nethttp "net/http"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/e2engine/core/execute/call"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/keys"
)

func TestEvaluatorEvaluateResponse(t *testing.T) {
	type bodyCall struct {
		json  bool
		value string
	}

	type deviationCall struct {
		payload   bool
		deviation model.TestExecutionDeviation
	}

	tests := []struct {
		name string

		status  int
		body    string
		headers nethttp.Header
		expect  model.HTTPResponse

		bodyCall   *bodyCall
		deviations []deviationCall
	}{
		{
			name:   "matching status without body expectation",
			status: nethttp.StatusOK,
			body:   `{"ignored":true}`,
			expect: model.HTTPResponse{
				StatusCode: nethttp.StatusOK,
			},
		},
		{
			name:   "status mismatch",
			status: nethttp.StatusNotFound,
			expect: model.HTTPResponse{
				StatusCode: nethttp.StatusOK,
			},
			deviations: []deviationCall{
				{
					deviation: model.TestExecutionDeviation{
						Field:    string(keys.FieldStatusCode),
						Expected: "200",
						Actual:   "404",
					},
				},
			},
		},
		{
			name:   "matching JSON body",
			status: nethttp.StatusOK,
			body:   `{"name":"test","count":1}`,
			expect: model.HTTPResponse{
				StatusCode: nethttp.StatusOK,
				BodyJSON:   stringPtr(`{"count":1,"name":"test"}`),
			},
			bodyCall: &bodyCall{
				json:  true,
				value: `{"name":"test","count":1}`,
			},
		},
		{
			name:   "JSON body mismatch",
			status: nethttp.StatusOK,
			body:   `{"name":"actual"}`,
			expect: model.HTTPResponse{
				StatusCode: nethttp.StatusOK,
				BodyJSON:   stringPtr(`{"name":"expected"}`),
			},
			bodyCall: &bodyCall{
				json:  true,
				value: `{"name":"actual"}`,
			},
			deviations: []deviationCall{
				{
					payload: true,
					deviation: model.TestExecutionDeviation{
						Field:    string(keys.FieldBodyJSON),
						Expected: `{"name":"expected"}`,
						Actual:   `{"name":"actual"}`,
					},
				},
			},
		},
		{
			name:   "response body is not JSON",
			status: nethttp.StatusOK,
			body:   "hello",
			expect: model.HTTPResponse{
				StatusCode: nethttp.StatusOK,
				BodyJSON:   stringPtr(`{"message":"hello"}`),
			},
			bodyCall: &bodyCall{
				value: "hello",
			},
			deviations: []deviationCall{
				{
					payload: true,
					deviation: model.TestExecutionDeviation{
						Field:    string(keys.FieldBodyJSON),
						Expected: `{"message":"hello"}`,
						Actual:   "hello",
						Message:  "response body is not valid JSON",
					},
				},
			},
		},
		{
			name:   "status and body mismatch",
			status: nethttp.StatusBadRequest,
			body:   `{"actual":true}`,
			expect: model.HTTPResponse{
				StatusCode: nethttp.StatusOK,
				BodyJSON:   stringPtr(`{"expected":true}`),
			},
			bodyCall: &bodyCall{
				json:  true,
				value: `{"actual":true}`,
			},
			deviations: []deviationCall{
				{
					deviation: model.TestExecutionDeviation{
						Field:    string(keys.FieldStatusCode),
						Expected: "200",
						Actual:   "400",
					},
				},
				{
					payload: true,
					deviation: model.TestExecutionDeviation{
						Field:    string(keys.FieldBodyJSON),
						Expected: `{"expected":true}`,
						Actual:   `{"actual":true}`,
					},
				},
			},
		},
		{
			name:   "matching exact text body",
			status: nethttp.StatusOK,
			body:   "plain text body",
			expect: model.HTTPResponse{
				StatusCode: nethttp.StatusOK,
				Body:       stringPtr("plain text body"),
			},
			bodyCall: &bodyCall{
				value: "plain text body",
			},
		},
		{
			name:   "mismatched exact text body",
			status: nethttp.StatusOK,
			body:   "actual text body",
			expect: model.HTTPResponse{
				StatusCode: nethttp.StatusOK,
				Body:       stringPtr("expected text body"),
			},
			bodyCall: &bodyCall{
				value: "actual text body",
			},
			deviations: []deviationCall{
				{
					payload: true,
					deviation: model.TestExecutionDeviation{
						Field:    string(keys.FieldBody),
						Expected: "expected text body",
						Actual:   "actual text body",
					},
				},
			},
		},
		{
			name:   "expected header is persisted",
			status: nethttp.StatusOK,
			headers: nethttp.Header{
				"X-Test": {"value"},
			},
			expect: model.HTTPResponse{
				StatusCode: nethttp.StatusOK,
				Headers: map[string][]string{
					"X-Test": {"value"},
				},
			},
		},
		{
			name:   "unexpected header is not persisted",
			status: nethttp.StatusOK,
			headers: nethttp.Header{
				"Authorization": {"secret"},
				"X-Test":        {"value"},
			},
			expect: model.HTTPResponse{
				StatusCode: nethttp.StatusOK,
				Headers: map[string][]string{
					"X-Test": {"value"},
				},
			},
		},
		{
			name:   "header name is case insensitive",
			status: nethttp.StatusOK,
			headers: nethttp.Header{
				"X-Test": {"value"},
			},
			expect: model.HTTPResponse{
				StatusCode: nethttp.StatusOK,
				Headers: map[string][]string{
					"x-test": {"value"},
				},
			},
		},
		{
			name:   "header mismatch",
			status: nethttp.StatusOK,
			headers: nethttp.Header{
				"X-Test": {"actual"},
			},
			expect: model.HTTPResponse{
				StatusCode: nethttp.StatusOK,
				Headers: map[string][]string{
					"X-Test": {"expected"},
				},
			},
			deviations: []deviationCall{
				{
					deviation: model.TestExecutionDeviation{
						Field:    "headers.X-Test",
						Expected: "expected",
						Actual:   "actual",
					},
				},
			},
		},
		{
			name:   "expected header is missing",
			status: nethttp.StatusOK,
			headers: nethttp.Header{
				"X-Other": {"value"},
			},
			expect: model.HTTPResponse{
				StatusCode: nethttp.StatusOK,
				Headers: map[string][]string{
					"X-Test": {"expected"},
				},
			},
			deviations: []deviationCall{
				{
					deviation: model.TestExecutionDeviation{
						Field:    "headers.X-Test",
						Expected: "expected",
						Actual:   "",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			reporter := NewMockexecutionReporter(ctrl)

			evaluator := NewEvaluator(reporter)

			response := &nethttp.Response{
				StatusCode: tt.status,
				Header:     tt.headers,
				Body: io.NopCloser(
					strings.NewReader(tt.body),
				),
			}

			reporter.EXPECT().
				AddHTTPResponseStatus(tt.status)

			reporter.EXPECT().
				AddHTTPResponseHeaders(tt.headers)

			for _, deviation := range tt.deviations {
				if deviation.payload {
					reporter.EXPECT().
						AddPayloadDeviation(&deviation.deviation)
				} else {
					reporter.EXPECT().
						AddDeviation(&deviation.deviation)
				}
			}

			if tt.bodyCall != nil {
				if tt.bodyCall.json {
					reporter.EXPECT().
						AddHTTPResponseBodyJSON(
							tt.bodyCall.value,
						)
				} else {
					reporter.EXPECT().
						AddHTTPResponseBody(
							tt.bodyCall.value,
						)
				}
			}

			err := evaluator.EvaluateResponse(
				response,
				&tt.expect,
			)
			if err != nil {
				t.Fatalf(
					"EvaluateResponse() error = %v",
					err,
				)
			}
		})
	}
}

func TestEvaluatorEvaluateResponseReadError(t *testing.T) {
	expectedErr := errors.New("read error")

	ctrl := gomock.NewController(t)
	reporter := NewMockexecutionReporter(ctrl)

	evaluator := NewEvaluator(reporter)

	response := &nethttp.Response{
		StatusCode: nethttp.StatusOK,
		Body: &errorReader{
			err: expectedErr,
		},
	}

	reporter.EXPECT().
		AddHTTPResponseStatus(nethttp.StatusOK)

	reporter.EXPECT().
		AddHTTPResponseHeaders(nil)

	err := evaluator.EvaluateResponse(
		response,
		&model.HTTPResponse{
			StatusCode: nethttp.StatusOK,
			BodyJSON:   stringPtr(`{"result":"ok"}`),
		},
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
		name string

		actual   call.Call
		expected model.ServiceExpectedCalls

		match bool
	}{
		{
			name:   "matches",
			actual: actual,
			expected: model.ServiceExpectedCalls{
				ServiceID: "service-1",
				HTTP: &model.HTTPRequestPattern{
					Method:   "POST",
					Path:     "/users",
					BodyJSON: stringPtr(`{"name":"Yaroslav"}`),
				},
			},
			match: true,
		},
		{
			name:   "different service",
			actual: actual,
			expected: model.ServiceExpectedCalls{
				ServiceID: "service-2",
				HTTP: &model.HTTPRequestPattern{
					Method: "POST",
				},
			},
		},
		{
			name: "actual is not HTTP",
			actual: call.Call{
				ServiceID: "service-1",
			},
			expected: model.ServiceExpectedCalls{
				ServiceID: "service-1",
				HTTP: &model.HTTPRequestPattern{
					Method: "POST",
				},
			},
		},
		{
			name:   "expectation is not HTTP",
			actual: actual,
			expected: model.ServiceExpectedCalls{
				ServiceID: "service-1",
			},
		},
		{
			name:   "HTTP expectation does not match",
			actual: actual,
			expected: model.ServiceExpectedCalls{
				ServiceID: "service-1",
				HTTP: &model.HTTPRequestPattern{
					Method: "GET",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			reporter := NewMockexecutionReporter(ctrl)

			evaluator := NewEvaluator(reporter)

			match, err := evaluator.Matches(
				context.Background(),
				&model.ServiceSpec{},
				tt.actual,
				tt.expected,
			)
			if err != nil {
				t.Fatalf(
					"Matches() error = %v",
					err,
				)
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
		name string

		expected model.HTTPRequestPattern
		body     []byte

		match bool
	}{
		{
			name:  "empty expectation",
			match: true,
		},
		{
			name: "all fields match",
			expected: model.HTTPRequestPattern{
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
				BodyJSON: stringPtr(
					`{"active":true,"name":"Yaroslav"}`,
				),
			},
			match: true,
		},
		{
			name: "method mismatch",
			expected: model.HTTPRequestPattern{
				Method: "GET",
			},
		},
		{
			name: "path mismatch",
			expected: model.HTTPRequestPattern{
				Path: "/orders",
			},
		},
		{
			name: "query missing",
			expected: model.HTTPRequestPattern{
				Query: map[string][]string{
					"missing": {"value"},
				},
			},
		},
		{
			name: "query mismatch",
			expected: model.HTTPRequestPattern{
				Query: map[string][]string{
					"active": {"false"},
				},
			},
		},
		{
			name: "header missing",
			expected: model.HTTPRequestPattern{
				Headers: map[string][]string{
					"X-Missing": {"value"},
				},
			},
		},
		{
			name: "header mismatch",
			expected: model.HTTPRequestPattern{
				Headers: map[string][]string{
					"Content-Type": {"text/plain"},
				},
			},
		},
		{
			name: "header name is case insensitive",
			expected: model.HTTPRequestPattern{
				Headers: map[string][]string{
					"x-test": {"one", "two"},
				},
			},
			match: true,
		},
		{
			name: "JSON body mismatch",
			expected: model.HTTPRequestPattern{
				BodyJSON: stringPtr(
					`{"name":"Other"}`,
				),
			},
		},
		{
			name: "actual body is invalid JSON",
			expected: model.HTTPRequestPattern{
				BodyJSON: stringPtr(
					`{"name":"Yaroslav"}`,
				),
			},
			body: []byte("not-json"),
		},
		{
			name: "exact text body matches",
			expected: model.HTTPRequestPattern{
				Body: stringPtr(
					`{"name":"Yaroslav","active":true}`,
				),
			},
			match: true,
		},
		{
			name: "exact text body mismatch",
			expected: model.HTTPRequestPattern{
				Body: stringPtr(
					`{"active":true,"name":"Yaroslav"}`,
				),
			},
		},
		{
			name: "query subset matches",
			expected: model.HTTPRequestPattern{
				Query: map[string][]string{
					"active": {"true"},
				},
			},
			match: true,
		},
		{
			name: "headers subset matches",
			expected: model.HTTPRequestPattern{
				Headers: map[string][]string{
					"Content-Type": {"application/json"},
				},
			},
			match: true,
		},
		{
			name: "query values match regardless of order",
			expected: model.HTTPRequestPattern{
				Query: map[string][]string{
					"role": {"user", "admin"},
				},
			},
			match: true,
		},
		{
			name: "header values match regardless of order",
			expected: model.HTTPRequestPattern{
				Headers: map[string][]string{
					"X-Test": {"two", "one"},
				},
			},
			match: true,
		},
		{
			name: "explicit empty body matches empty body",
			expected: model.HTTPRequestPattern{
				Body: stringPtr(""),
			},
			body:  []byte{},
			match: true,
		},
		{
			name: "explicit empty body does not match non-empty body",
			expected: model.HTTPRequestPattern{
				Body: stringPtr(""),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := actual

			if tt.body != nil {
				request.Body = tt.body
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
	ctrl := gomock.NewController(t)
	reporter := NewMockexecutionReporter(ctrl)

	evaluator := NewEvaluator(reporter)

	service := &model.ServiceSpec{
		ID: "service-1",
	}

	tests := []struct {
		name string

		actual call.Call

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
							headers.Set(
								string(
									keys.E2EngineMockMissHeader,
								),
								"true",
							)

							return headers
						}(),
					},
				},
			},
			expected: &model.TestExecutionDeviation{
				Field:  "calls.service-1",
				Actual: "POST /users",
				Message: "mocked service received a request " +
					"that did not match any fixture",
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

func stringPtr(value string) *string {
	return &value
}
