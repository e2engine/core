package http

import (
	"context"
	"errors"
	"io"
	nethttp "net/http"
	"reflect"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/e2engine/core/execute/call"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/model/matcher"
	"github.com/e2engine/core/pkg/keys"
)

func TestEvaluatorEvaluateResponse(t *testing.T) {
	type bodyCall struct {
		json  bool
		value string
	}

	type deviationCall struct {
		deviation *model.TestExecutionDeviation
		payload   *model.PayloadDeviation
	}

	tests := []struct {
		name string

		status  int
		body    string
		headers nethttp.Header
		expect  model.HTTPResponsePattern

		bodyCall   bodyCall
		deviations []deviationCall
	}{
		{
			name:   "matching status without body expectation",
			status: nethttp.StatusOK,
			body:   `{"ignored":true}`,
			expect: model.HTTPResponsePattern{
				StatusCode: nethttp.StatusOK,
			},
			bodyCall: bodyCall{
				value: `{"ignored":true}`,
			},
		},
		{
			name:   "status mismatch",
			status: nethttp.StatusNotFound,
			expect: model.HTTPResponsePattern{
				StatusCode: nethttp.StatusOK,
			},
			deviations: []deviationCall{
				{
					deviation: &model.TestExecutionDeviation{
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
			expect: model.HTTPResponsePattern{
				StatusCode: nethttp.StatusOK,
				BodyJSON:   stringMatcher(`{"count":1,"name":"test"}`),
			},
			bodyCall: bodyCall{
				json:  true,
				value: `{"name":"test","count":1}`,
			},
		},
		{
			name:   "matching JSON body with whitespace",
			status: nethttp.StatusOK,
			body:   "{\n  \"name\": \"test\"\n}\n",
			expect: model.HTTPResponsePattern{
				StatusCode: nethttp.StatusOK,
				BodyJSON:   stringMatcher(`{"name":"test"}`),
			},
			bodyCall: bodyCall{
				json:  true,
				value: `{"name":"test"}`,
			},
		},
		{
			name:   "JSON body mismatch",
			status: nethttp.StatusOK,
			body:   `{"name":"actual"}`,
			expect: model.HTTPResponsePattern{
				StatusCode: nethttp.StatusOK,
				BodyJSON:   stringMatcher(`{"name":"expected"}`),
			},
			bodyCall: bodyCall{
				json:  true,
				value: `{"name":"actual"}`,
			},
			deviations: []deviationCall{
				{
					payload: &model.PayloadDeviation{
						Field:    string(keys.FieldBodyJSON),
						Expected: stringMatcher(`{"name":"expected"}`),
						Actual:   []byte(`{"name":"actual"}`),
					},
				},
			},
		},
		{
			name:   "response body is not JSON",
			status: nethttp.StatusOK,
			body:   "hello",
			expect: model.HTTPResponsePattern{
				StatusCode: nethttp.StatusOK,
				BodyJSON:   stringMatcher(`{"message":"hello"}`),
			},
			bodyCall: bodyCall{
				value: "hello",
				json:  true,
			},
			deviations: []deviationCall{
				{
					payload: &model.PayloadDeviation{
						Field:    string(keys.FieldBodyJSON),
						Expected: stringMatcher(`{"message":"hello"}`),
						Actual:   []byte("hello"),
						Message:  "response body is not valid JSON",
					},
				},
			},
		},
		{
			name:   "status and body mismatch",
			status: nethttp.StatusBadRequest,
			body:   `{"actual":true}`,
			expect: model.HTTPResponsePattern{
				StatusCode: nethttp.StatusOK,
				BodyJSON:   stringMatcher(`{"expected":true}`),
			},
			bodyCall: bodyCall{
				json:  true,
				value: `{"actual":true}`,
			},
			deviations: []deviationCall{
				{
					deviation: &model.TestExecutionDeviation{
						Field:    string(keys.FieldStatusCode),
						Expected: "200",
						Actual:   "400",
					},
				},
				{
					payload: &model.PayloadDeviation{
						Field:    string(keys.FieldBodyJSON),
						Expected: stringMatcher(`{"expected":true}`),
						Actual:   []byte(`{"actual":true}`),
					},
				},
			},
		},
		{
			name:   "matching exact text body",
			status: nethttp.StatusOK,
			body:   "plain text body",
			expect: model.HTTPResponsePattern{
				StatusCode: nethttp.StatusOK,
				Body:       stringMatcher("plain text body"),
			},
			bodyCall: bodyCall{
				value: "plain text body",
			},
		},
		{
			name:   "mismatched exact text body",
			status: nethttp.StatusOK,
			body:   "actual text body",
			expect: model.HTTPResponsePattern{
				StatusCode: nethttp.StatusOK,
				Body:       stringMatcher("expected text body"),
			},
			bodyCall: bodyCall{
				value: "actual text body",
			},
			deviations: []deviationCall{
				{
					payload: &model.PayloadDeviation{
						Field:    string(keys.FieldBody),
						Expected: stringMatcher("expected text body"),
						Actual:   []byte("actual text body"),
					},
				},
			},
		},
		{
			name:   "exact body preserves trailing newline",
			status: nethttp.StatusOK,
			body:   "hello\n",
			expect: model.HTTPResponsePattern{
				StatusCode: nethttp.StatusOK,
				Body:       stringMatcher("hello"),
			},
			bodyCall: bodyCall{
				value: "hello\n",
			},
			deviations: []deviationCall{
				{
					payload: &model.PayloadDeviation{
						Field:    string(keys.FieldBody),
						Expected: stringMatcher("hello"),
						Actual:   []byte("hello\n"),
					},
				},
			},
		},
		{
			name:   "matching empty body",
			status: nethttp.StatusOK,
			expect: model.HTTPResponsePattern{
				StatusCode: nethttp.StatusOK,
				Body:       stringMatcher(""),
			},
		},
		{
			name:   "nonempty body matches",
			status: nethttp.StatusOK,
			body:   "hello",
			expect: model.HTTPResponsePattern{
				StatusCode: nethttp.StatusOK,
				Body:       stringMatcherOperator(matcher.StringMatchOperatorNonEmpty),
			},
			bodyCall: bodyCall{
				value: "hello",
			},
		},
		{
			name:   "nonempty body mismatch",
			status: nethttp.StatusOK,
			expect: model.HTTPResponsePattern{
				StatusCode: nethttp.StatusOK,
				Body:       stringMatcherOperator(matcher.StringMatchOperatorNonEmpty),
			},
			deviations: []deviationCall{
				{
					payload: &model.PayloadDeviation{
						Field:    string(keys.FieldBody),
						Expected: stringMatcherOperator(matcher.StringMatchOperatorNonEmpty),
						Actual:   []byte{},
					},
				},
			},
		},
		{
			name:   "exists body matches empty body",
			status: nethttp.StatusOK,
			expect: model.HTTPResponsePattern{
				StatusCode: nethttp.StatusOK,
				Body:       stringMatcherOperator(matcher.StringMatchOperatorExists),
			},
		},
		{
			name:   "nonempty JSON matches empty object",
			status: nethttp.StatusOK,
			body:   `{}`,
			expect: model.HTTPResponsePattern{
				StatusCode: nethttp.StatusOK,
				BodyJSON:   stringMatcherOperator(matcher.StringMatchOperatorNonEmpty),
			},
			bodyCall: bodyCall{
				json:  true,
				value: `{}`,
			},
		},
		{
			name:   "nonempty JSON rejects empty body",
			status: nethttp.StatusOK,
			expect: model.HTTPResponsePattern{
				StatusCode: nethttp.StatusOK,
				BodyJSON:   stringMatcherOperator(matcher.StringMatchOperatorNonEmpty),
			},
			bodyCall: bodyCall{
				json: true,
			},
			deviations: []deviationCall{
				{
					payload: &model.PayloadDeviation{
						Field:    string(keys.FieldBodyJSON),
						Expected: stringMatcherOperator(matcher.StringMatchOperatorNonEmpty),
						Actual:   []byte{},
						Message:  "response body is not valid JSON",
					},
				},
			},
		},
		{
			name:   "expected header matches",
			status: nethttp.StatusOK,
			headers: nethttp.Header{
				"X-Test": {"value"},
			},
			expect: model.HTTPResponsePattern{
				StatusCode: nethttp.StatusOK,
				Headers: map[string][]matcher.String{
					"X-Test": {stringMatcherValue("value")},
				},
			},
		},
		{
			name:   "unexpected header is ignored",
			status: nethttp.StatusOK,
			headers: nethttp.Header{
				"Authorization": {"secret"},
				"X-Test":        {"value"},
			},
			expect: model.HTTPResponsePattern{
				StatusCode: nethttp.StatusOK,
				Headers: map[string][]matcher.String{
					"X-Test": {stringMatcherValue("value")},
				},
			},
		},
		{
			name:   "header name is case insensitive",
			status: nethttp.StatusOK,
			headers: nethttp.Header{
				"X-Test": {"value"},
			},
			expect: model.HTTPResponsePattern{
				StatusCode: nethttp.StatusOK,
				Headers: map[string][]matcher.String{
					"x-test": {stringMatcherValue("value")},
				},
			},
		},
		{
			name:   "header mismatch",
			status: nethttp.StatusOK,
			headers: nethttp.Header{
				"X-Test": {"actual"},
			},
			expect: model.HTTPResponsePattern{
				StatusCode: nethttp.StatusOK,
				Headers: map[string][]matcher.String{
					"X-Test": {stringMatcherValue("expected")},
				},
			},
			deviations: []deviationCall{
				{
					deviation: &model.TestExecutionDeviation{
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
			expect: model.HTTPResponsePattern{
				StatusCode: nethttp.StatusOK,
				Headers: map[string][]matcher.String{
					"X-Test": {stringMatcherValue("expected")},
				},
			},
			deviations: []deviationCall{
				{
					deviation: &model.TestExecutionDeviation{
						Field:    "headers.X-Test",
						Expected: "expected",
					},
				},
			},
		},
		{
			name:   "header exists with empty value",
			status: nethttp.StatusOK,
			headers: nethttp.Header{
				"X-Test": {""},
			},
			expect: model.HTTPResponsePattern{
				StatusCode: nethttp.StatusOK,
				Headers: map[string][]matcher.String{
					"X-Test": {stringMatcherOperatorValue(matcher.StringMatchOperatorExists)},
				},
			},
		},
		{
			name:   "header exists but is missing",
			status: nethttp.StatusOK,
			expect: model.HTTPResponsePattern{
				StatusCode: nethttp.StatusOK,
				Headers: map[string][]matcher.String{
					"X-Test": {stringMatcherOperatorValue(matcher.StringMatchOperatorExists)},
				},
			},
			deviations: []deviationCall{
				{
					deviation: &model.TestExecutionDeviation{
						Field:    "headers.X-Test",
						Expected: "exists",
					},
				},
			},
		},
		{
			name:   "nonempty header matches",
			status: nethttp.StatusOK,
			headers: nethttp.Header{
				"X-Test": {"value"},
			},
			expect: model.HTTPResponsePattern{
				StatusCode: nethttp.StatusOK,
				Headers: map[string][]matcher.String{
					"X-Test": {stringMatcherOperatorValue(matcher.StringMatchOperatorNonEmpty)},
				},
			},
		},
		{
			name:   "nonempty header rejects empty value",
			status: nethttp.StatusOK,
			headers: nethttp.Header{
				"X-Test": {""},
			},
			expect: model.HTTPResponsePattern{
				StatusCode: nethttp.StatusOK,
				Headers: map[string][]matcher.String{
					"X-Test": {stringMatcherOperatorValue(matcher.StringMatchOperatorNonEmpty)},
				},
			},
			deviations: []deviationCall{
				{
					deviation: &model.TestExecutionDeviation{
						Field:    "headers.X-Test",
						Expected: "nonempty",
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
				switch {
				case deviation.payload != nil:
					reporter.EXPECT().
						AddPayloadDeviation(deviation.payload)

				case deviation.deviation != nil &&
					strings.HasPrefix(deviation.deviation.Field, "headers."):
					reporter.EXPECT().
						AddHeadersDeviation(
							strings.TrimPrefix(deviation.deviation.Field, "headers."),
							deviation.deviation,
						)

				default:
					reporter.EXPECT().
						AddDeviation(deviation.deviation)
				}
			}

			if tt.bodyCall.json {
				reporter.EXPECT().
					AddHTTPResponseBodyJSON(
						[]byte(tt.bodyCall.value),
					)
			} else {
				reporter.EXPECT().
					AddHTTPResponseBody(
						[]byte(tt.bodyCall.value),
					)
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
		&model.HTTPResponsePattern{
			StatusCode: nethttp.StatusOK,
			BodyJSON:   stringMatcher(`{"result":"ok"}`),
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
					BodyJSON: stringMatcher(`{"name":"Yaroslav"}`),
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

			matched, err := evaluator.Matches(
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

			if matched != tt.match {
				t.Errorf(
					"expected match %v, got %v",
					tt.match,
					matched,
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
			"X-Empty":      {""},
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
				Headers: map[string][]matcher.String{
					"Content-Type": {stringMatcherValue("application/json")},
					"X-Test": {
						stringMatcherValue("one"),
						stringMatcherValue("two"),
					},
				},
				BodyJSON: stringMatcher(
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
				Headers: map[string][]matcher.String{
					"X-Missing": {stringMatcherValue("value")},
				},
			},
		},
		{
			name: "header mismatch",
			expected: model.HTTPRequestPattern{
				Headers: map[string][]matcher.String{
					"Content-Type": {stringMatcherValue("text/plain")},
				},
			},
		},
		{
			name: "header name is case insensitive",
			expected: model.HTTPRequestPattern{
				Headers: map[string][]matcher.String{
					"x-test": {
						stringMatcherValue("one"),
						stringMatcherValue("two"),
					},
				},
			},
			match: true,
		},
		{
			name: "JSON body mismatch",
			expected: model.HTTPRequestPattern{
				BodyJSON: stringMatcher(
					`{"name":"Other"}`,
				),
			},
		},
		{
			name: "actual body is invalid JSON",
			expected: model.HTTPRequestPattern{
				BodyJSON: stringMatcher(
					`{"name":"Yaroslav"}`,
				),
			},
			body: []byte("not-json"),
		},
		{
			name: "exact text body matches",
			expected: model.HTTPRequestPattern{
				Body: stringMatcher(
					`{"name":"Yaroslav","active":true}`,
				),
			},
			match: true,
		},
		{
			name: "exact text body mismatch",
			expected: model.HTTPRequestPattern{
				Body: stringMatcher(
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
				Headers: map[string][]matcher.String{
					"Content-Type": {stringMatcherValue("application/json")},
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
				Headers: map[string][]matcher.String{
					"X-Test": {
						stringMatcherValue("two"),
						stringMatcherValue("one"),
					},
				},
			},
			match: true,
		},
		{
			name: "header exists",
			expected: model.HTTPRequestPattern{
				Headers: map[string][]matcher.String{
					"X-Test": {
						stringMatcherOperatorValue(matcher.StringMatchOperatorExists),
					},
				},
			},
			match: true,
		},
		{
			name: "header exists with empty value",
			expected: model.HTTPRequestPattern{
				Headers: map[string][]matcher.String{
					"X-Empty": {
						stringMatcherOperatorValue(matcher.StringMatchOperatorExists),
					},
				},
			},
			match: true,
		},
		{
			name: "header exists but is missing",
			expected: model.HTTPRequestPattern{
				Headers: map[string][]matcher.String{
					"X-Missing": {
						stringMatcherOperatorValue(matcher.StringMatchOperatorExists),
					},
				},
			},
		},
		{
			name: "header nonempty",
			expected: model.HTTPRequestPattern{
				Headers: map[string][]matcher.String{
					"X-Test": {
						stringMatcherOperatorValue(matcher.StringMatchOperatorNonEmpty),
					},
				},
			},
			match: true,
		},
		{
			name: "header nonempty rejects empty value",
			expected: model.HTTPRequestPattern{
				Headers: map[string][]matcher.String{
					"X-Empty": {
						stringMatcherOperatorValue(matcher.StringMatchOperatorNonEmpty),
					},
				},
			},
		},
		{
			name: "explicit empty body matches empty body",
			expected: model.HTTPRequestPattern{
				Body: stringMatcher(""),
			},
			body:  []byte{},
			match: true,
		},
		{
			name: "explicit empty body does not match nonempty body",
			expected: model.HTTPRequestPattern{
				Body: stringMatcher(""),
			},
		},
		{
			name: "nonempty body matches",
			expected: model.HTTPRequestPattern{
				Body: stringMatcherOperator(matcher.StringMatchOperatorNonEmpty),
			},
			match: true,
		},
		{
			name: "nonempty body rejects empty body",
			expected: model.HTTPRequestPattern{
				Body: stringMatcherOperator(matcher.StringMatchOperatorNonEmpty),
			},
			body: []byte{},
		},
		{
			name: "JSON nonempty matches",
			expected: model.HTTPRequestPattern{
				BodyJSON: stringMatcherOperator(matcher.StringMatchOperatorNonEmpty),
			},
			match: true,
		},
		{
			name: "JSON exists matches",
			expected: model.HTTPRequestPattern{
				BodyJSON: stringMatcherOperator(matcher.StringMatchOperatorExists),
			},
			match: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := actual

			if tt.body != nil {
				request.Body = tt.body
			}

			matched := httpCallMatchesExpectation(
				request,
				&tt.expected,
			)

			if matched != tt.match {
				t.Errorf(
					"expected match %v, got %v",
					tt.match,
					matched,
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
								string(keys.E2EngineMockMissHeader),
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

			if !reflect.DeepEqual(actual, tt.expected) {
				t.Errorf(
					"expected deviation %+v, got %+v",
					tt.expected,
					actual,
				)
			}
		})
	}
}

type errorReader struct {
	err error
}

func (r *errorReader) Read(_ []byte) (int, error) {
	return 0, r.err
}

func (r *errorReader) Close() error {
	return nil
}

func stringMatcher(value string) *matcher.String {
	return &matcher.String{
		Operator: matcher.StringMatchOperatorEquals,
		Value:    value,
	}
}

func stringMatcherValue(value string) matcher.String {
	return matcher.String{
		Operator: matcher.StringMatchOperatorEquals,
		Value:    value,
	}
}

func stringMatcherOperator(
	operator matcher.StringMatchOperator,
) *matcher.String {
	return &matcher.String{
		Operator: operator,
	}
}

func stringMatcherOperatorValue(
	operator matcher.StringMatchOperator,
) matcher.String {
	return matcher.String{
		Operator: operator,
	}
}
