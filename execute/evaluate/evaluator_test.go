package evaluate

import (
	"context"
	"errors"
	"testing"

	"github.com/e2engine/core/execute"
	"github.com/e2engine/core/execute/call"
	"github.com/e2engine/core/model"
)

func TestEvaluatorEvaluateCalls(t *testing.T) {
	httpEvaluator := &testCallEvaluator{
		deviation: &model.TestExecutionDeviation{
			Field:   "calls.http-service",
			Message: "HTTP deviation",
		},
		matches: true,
	}

	grpcEvaluator := &testCallEvaluator{
		deviation: &model.TestExecutionDeviation{
			Field:   "calls.grpc-service",
			Message: "gRPC deviation",
		},
		matches: true,
	}

	evaluator := New(
		httpEvaluator,
		grpcEvaluator,
	)

	calls := &call.Store{}

	calls.Record(call.Call{
		TestExecutionID: "execution-1",
		ServiceID:       "http-service",
		HTTP:            &call.HTTPCall{},
	})

	calls.Record(call.Call{
		TestExecutionID: "execution-1",
		ServiceID:       "grpc-service",
		GRPC:            &call.GRPCCall{},
	})

	result := newTestExecutionResult()

	err := evaluator.EvaluateCalls(
		context.Background(),
		[]model.ServiceSpec{
			{
				ID:   "http-service",
				Kind: model.ServiceKindHTTP,
				Mode: model.ServiceModeMocked,
			},
			{
				ID:   "grpc-service",
				Kind: model.ServiceKindGRPC,
				Mode: model.ServiceModeMocked,
			},
		},
		[]model.CallExpectation{
			{
				ServiceID: "http-service",
				HTTP:      &model.HTTPCallExpectation{},
			},
			{
				ServiceID: "grpc-service",
				GRPC:      &model.GRPCCallExpectation{},
			},
		},
		calls,
		result,
	)
	if err != nil {
		t.Fatalf(
			"EvaluateCalls() error = %v",
			err,
		)
	}

	if len(result.Summary.Deviations) != 2 {
		t.Fatalf(
			"expected 2 deviations, got %d",
			len(result.Summary.Deviations),
		)
	}

	if result.Summary.Deviations[0].Field !=
		"calls.http-service" {
		t.Errorf(
			"expected HTTP deviation, got %+v",
			result.Summary.Deviations[0],
		)
	}

	if result.Summary.Deviations[1].Field !=
		"calls.grpc-service" {
		t.Errorf(
			"expected gRPC deviation, got %+v",
			result.Summary.Deviations[1],
		)
	}
}

func TestEvaluatorEvaluateCallsCancelledContext(
	t *testing.T,
) {
	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	evaluator := New(
		&testCallEvaluator{},
		&testCallEvaluator{},
	)

	result := newTestExecutionResult()

	err := evaluator.EvaluateCalls(
		ctx,
		nil,
		nil,
		&call.Store{},
		result,
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"expected context.Canceled, got %v",
			err,
		)
	}
}

func TestEvaluatorEvaluateActualCalls(t *testing.T) {
	httpEvaluator := &testCallEvaluator{
		deviation: &model.TestExecutionDeviation{
			Field:   "calls.http-service",
			Message: "HTTP deviation",
		},
	}

	grpcEvaluator := &testCallEvaluator{
		deviation: &model.TestExecutionDeviation{
			Field:   "calls.grpc-service",
			Message: "gRPC deviation",
		},
	}

	evaluator := New(
		httpEvaluator,
		grpcEvaluator,
	)

	calls := &call.Store{}

	calls.Record(call.Call{
		TestExecutionID: "execution-1",
		ServiceID:       "http-service",
		HTTP:            &call.HTTPCall{},
	})

	calls.Record(call.Call{
		TestExecutionID: "execution-1",
		ServiceID:       "grpc-service",
		GRPC:            &call.GRPCCall{},
	})

	// Must not be evaluated because the service is not mocked.
	calls.Record(call.Call{
		TestExecutionID: "execution-1",
		ServiceID:       "real-service",
		HTTP:            &call.HTTPCall{},
	})

	// Must not be evaluated because it belongs to another execution.
	calls.Record(call.Call{
		TestExecutionID: "execution-2",
		ServiceID:       "http-service",
		HTTP:            &call.HTTPCall{},
	})

	result := newTestExecutionResult()

	evaluator.evaluateActualCalls(
		[]model.ServiceSpec{
			{
				ID:   "http-service",
				Kind: model.ServiceKindHTTP,
				Mode: model.ServiceModeMocked,
			},
			{
				ID:   "grpc-service",
				Kind: model.ServiceKindGRPC,
				Mode: model.ServiceModeMocked,
			},
			{
				ID:   "real-service",
				Kind: model.ServiceKindHTTP,
			},
		},
		calls,
		result,
	)

	if len(result.Summary.Deviations) != 2 {
		t.Fatalf(
			"expected 2 deviations, got %d",
			len(result.Summary.Deviations),
		)
	}

	if httpEvaluator.deviationCalls != 1 {
		t.Errorf(
			"expected HTTP Deviation() to be called once, got %d",
			httpEvaluator.deviationCalls,
		)
	}

	if grpcEvaluator.deviationCalls != 1 {
		t.Errorf(
			"expected gRPC Deviation() to be called once, got %d",
			grpcEvaluator.deviationCalls,
		)
	}
}

func TestEvaluatorEvaluateActualCallsNoDeviation(
	t *testing.T,
) {
	httpEvaluator := &testCallEvaluator{}

	evaluator := New(
		httpEvaluator,
		&testCallEvaluator{},
	)

	calls := &call.Store{}

	calls.Record(call.Call{
		TestExecutionID: "execution-1",
		ServiceID:       "service-1",
		HTTP:            &call.HTTPCall{},
	})

	result := newTestExecutionResult()

	evaluator.evaluateActualCalls(
		[]model.ServiceSpec{
			{
				ID:   "service-1",
				Kind: model.ServiceKindHTTP,
				Mode: model.ServiceModeMocked,
			},
		},
		calls,
		result,
	)

	if len(result.Summary.Deviations) != 0 {
		t.Fatalf(
			"expected no deviations, got %+v",
			result.Summary.Deviations,
		)
	}
}

func TestEvaluatorEvaluateExpectedCalls(t *testing.T) {
	tests := []struct {
		name string

		count   *int
		matches bool
		calls   int

		expectedDeviation *model.TestExecutionDeviation
	}{
		{
			name:    "at least one match",
			matches: true,
			calls:   1,
		},
		{
			name:    "multiple matches without count",
			matches: true,
			calls:   2,
		},
		{
			name:    "no match without count",
			matches: false,
			calls:   1,
			expectedDeviation: &model.TestExecutionDeviation{
				Field:    "calls.service-1.count",
				Expected: "at least 1",
				Actual:   "0",
			},
		},
		{
			name:    "no calls without count",
			matches: true,
			calls:   0,
			expectedDeviation: &model.TestExecutionDeviation{
				Field:    "calls.service-1.count",
				Expected: "at least 1",
				Actual:   "0",
			},
		},
		{
			name:    "exact count matches",
			count:   intPtr(2),
			matches: true,
			calls:   2,
		},
		{
			name:    "exact count too low",
			count:   intPtr(2),
			matches: true,
			calls:   1,
			expectedDeviation: &model.TestExecutionDeviation{
				Field:    "calls.service-1.count",
				Expected: "2",
				Actual:   "1",
			},
		},
		{
			name:    "exact count too high",
			count:   intPtr(1),
			matches: true,
			calls:   2,
			expectedDeviation: &model.TestExecutionDeviation{
				Field:    "calls.service-1.count",
				Expected: "1",
				Actual:   "2",
			},
		},
		{
			name:    "zero expected and no matches",
			count:   intPtr(0),
			matches: false,
			calls:   1,
		},
		{
			name:    "zero expected but call matches",
			count:   intPtr(0),
			matches: true,
			calls:   1,
			expectedDeviation: &model.TestExecutionDeviation{
				Field:    "calls.service-1.count",
				Expected: "0",
				Actual:   "1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpEvaluator := &testCallEvaluator{
				matches: tt.matches,
			}

			evaluator := New(
				httpEvaluator,
				&testCallEvaluator{},
			)

			calls := &call.Store{}

			for range tt.calls {
				calls.Record(call.Call{
					TestExecutionID: "execution-1",
					ServiceID:       "service-1",
					HTTP:            &call.HTTPCall{},
				})
			}

			result := newTestExecutionResult()

			err := evaluator.evaluateExpectedCalls(
				context.Background(),
				[]model.ServiceSpec{
					{
						ID:   "service-1",
						Kind: model.ServiceKindHTTP,
					},
				},
				[]model.CallExpectation{
					{
						ServiceID: "service-1",
						HTTP:      &model.HTTPCallExpectation{},
						Count:     tt.count,
					},
				},
				calls,
				result,
			)
			if err != nil {
				t.Fatalf(
					"evaluateExpectedCalls() error = %v",
					err,
				)
			}

			if tt.expectedDeviation == nil {
				if len(result.Summary.Deviations) != 0 {
					t.Fatalf(
						"expected no deviations, got %+v",
						result.Summary.Deviations,
					)
				}

				return
			}

			if len(result.Summary.Deviations) != 1 {
				t.Fatalf(
					"expected 1 deviation, got %d",
					len(result.Summary.Deviations),
				)
			}

			actual := result.Summary.Deviations[0]

			if actual != *tt.expectedDeviation {
				t.Errorf(
					"expected deviation %+v, got %+v",
					*tt.expectedDeviation,
					actual,
				)
			}
		})
	}
}

func TestEvaluatorEvaluateExpectedCallsError(
	t *testing.T,
) {
	expectedErr := errors.New("match error")

	httpEvaluator := &testCallEvaluator{
		matchesErr: expectedErr,
	}

	evaluator := New(
		httpEvaluator,
		&testCallEvaluator{},
	)

	calls := &call.Store{}

	calls.Record(call.Call{
		TestExecutionID: "execution-1",
		ServiceID:       "service-1",
		HTTP:            &call.HTTPCall{},
	})

	result := newTestExecutionResult()

	err := evaluator.evaluateExpectedCalls(
		context.Background(),
		[]model.ServiceSpec{
			{
				ID:   "service-1",
				Kind: model.ServiceKindHTTP,
			},
		},
		[]model.CallExpectation{
			{
				ServiceID: "service-1",
				HTTP:      &model.HTTPCallExpectation{},
			},
		},
		calls,
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
	tests := []struct {
		name string

		expected model.CallExpectation

		expectedHTTPCalls int
		expectedGRPCCalls int
		expectedMatch     bool
	}{
		{
			name: "HTTP",
			expected: model.CallExpectation{
				HTTP: &model.HTTPCallExpectation{},
			},
			expectedHTTPCalls: 1,
			expectedMatch:     true,
		},
		{
			name: "gRPC",
			expected: model.CallExpectation{
				GRPC: &model.GRPCCallExpectation{},
			},
			expectedGRPCCalls: 1,
			expectedMatch:     true,
		},
		{
			name:          "no protocol",
			expectedMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpEvaluator := &testCallEvaluator{
				matches: true,
			}

			grpcEvaluator := &testCallEvaluator{
				matches: true,
			}

			evaluator := New(
				httpEvaluator,
				grpcEvaluator,
			)

			match, err := evaluator.matches(
				context.Background(),
				&model.ServiceSpec{},
				call.Call{},
				tt.expected,
			)
			if err != nil {
				t.Fatalf(
					"matches() error = %v",
					err,
				)
			}

			if match != tt.expectedMatch {
				t.Errorf(
					"expected match %v, got %v",
					tt.expectedMatch,
					match,
				)
			}

			if httpEvaluator.matchesCalls !=
				tt.expectedHTTPCalls {
				t.Errorf(
					"expected HTTP Matches() calls %d, got %d",
					tt.expectedHTTPCalls,
					httpEvaluator.matchesCalls,
				)
			}

			if grpcEvaluator.matchesCalls !=
				tt.expectedGRPCCalls {
				t.Errorf(
					"expected gRPC Matches() calls %d, got %d",
					tt.expectedGRPCCalls,
					grpcEvaluator.matchesCalls,
				)
			}
		})
	}
}

func TestEvaluatorMatchesError(t *testing.T) {
	expectedErr := errors.New("match error")

	httpEvaluator := &testCallEvaluator{
		matchesErr: expectedErr,
	}

	evaluator := New(
		httpEvaluator,
		&testCallEvaluator{},
	)

	_, err := evaluator.matches(
		context.Background(),
		&model.ServiceSpec{},
		call.Call{},
		model.CallExpectation{
			HTTP: &model.HTTPCallExpectation{},
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

type testCallEvaluator struct {
	matches    bool
	matchesErr error
	deviation  *model.TestExecutionDeviation

	matchesCalls   int
	deviationCalls int
}

func (e *testCallEvaluator) Matches(
	_ context.Context,
	_ *model.ServiceSpec,
	_ call.Call,
	_ model.CallExpectation,
) (bool, error) {
	e.matchesCalls++

	return e.matches, e.matchesErr
}

func (e *testCallEvaluator) Deviation(
	_ *model.ServiceSpec,
	_ call.Call,
) *model.TestExecutionDeviation {
	e.deviationCalls++

	return e.deviation
}

func newTestExecutionResult() *execute.TestExecutionResult {
	return &execute.TestExecutionResult{
		ExecutionID: "execution-1",
		Summary:     &model.TestExecutionSummary{},
	}
}

func intPtr(value int) *int {
	return &value
}
