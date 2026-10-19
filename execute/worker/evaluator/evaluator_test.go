package evaluator

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/e2engine/core/execute/call"
	"github.com/e2engine/core/model"
	errorsPkg "github.com/e2engine/core/pkg/errors"
)

func TestEvaluatorEvaluateCalls(t *testing.T) {
	ctrl := gomock.NewController(t)

	reporter := NewMockdeviationReporter(ctrl)

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
		reporter,
	)

	calls := call.NewStoreFactory(
		&call.Config{
			MaxCalls:         1000,
			MaxRecordedBytes: 1024 * 1024,
		}).New()

	if errRecord := calls.Record(call.Call{
		TestExecutionID: "execution-1",
		ServiceID:       "http-service",
		HTTP:            &call.HTTPCall{},
	}); errRecord != nil {
		t.Fatalf("failed to record call: %v", errRecord)
	}

	if errRecord := calls.Record(call.Call{
		TestExecutionID: "execution-1",
		ServiceID:       "grpc-service",
		GRPC:            &call.GRPCCall{},
	}); errRecord != nil {
		t.Fatalf("failed to record call: %v", errRecord)
	}

	gomock.InOrder(
		reporter.EXPECT().AddDeviation(httpEvaluator.deviation),
		reporter.EXPECT().AddDeviation(grpcEvaluator.deviation),
	)

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
		[]model.ServiceExpectedCalls{
			{
				ServiceID: "http-service",
				HTTP:      &model.HTTPRequestPattern{},
			},
			{
				ServiceID: "grpc-service",
				GRPC:      &model.GRPCRequestPattern{},
			},
		},
		calls,
		"execution-1",
	)
	if err != nil {
		t.Fatalf(
			"EvaluateCalls() error = %v",
			err,
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

	ctrl := gomock.NewController(t)

	reporter := NewMockdeviationReporter(ctrl)

	evaluator := New(
		&testCallEvaluator{},
		&testCallEvaluator{},
		reporter,
	)

	err := evaluator.EvaluateCalls(
		ctx,
		nil,
		nil,
		&call.Store{},
		"execution-1",
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"expected context.Canceled, got %v",
			err,
		)
	}
}

func TestEvaluatorEvaluateActualCalls(t *testing.T) {
	ctrl := gomock.NewController(t)

	reporter := NewMockdeviationReporter(ctrl)

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
		reporter,
	)

	calls := call.NewStoreFactory(
		&call.Config{
			MaxCalls:         1000,
			MaxRecordedBytes: 1024 * 1024,
		}).New()

	if errRecord := calls.Record(call.Call{
		TestExecutionID: "execution-1",
		ServiceID:       "http-service",
		HTTP:            &call.HTTPCall{},
	}); errRecord != nil {
		t.Fatalf("failed to record call: %v", errRecord)
	}

	if errRecord := calls.Record(call.Call{
		TestExecutionID: "execution-1",
		ServiceID:       "grpc-service",
		GRPC:            &call.GRPCCall{},
	}); errRecord != nil {
		t.Fatalf("failed to record call: %v", errRecord)
	}

	// Must not be evaluated because the service is not mocked.
	if errRecord := calls.Record(call.Call{
		TestExecutionID: "execution-1",
		ServiceID:       "real-service",
		HTTP:            &call.HTTPCall{},
	}); errRecord != nil {
		t.Fatalf("failed to record call: %v", errRecord)
	}

	// Must not be evaluated because it belongs to another execution.
	if errRecord := calls.Record(call.Call{
		TestExecutionID: "execution-2",
		ServiceID:       "http-service",
		HTTP:            &call.HTTPCall{},
	}); errRecord != nil {
		t.Fatalf("failed to record call: %v", errRecord)
	}

	gomock.InOrder(
		reporter.EXPECT().AddDeviation(httpEvaluator.deviation),
		reporter.EXPECT().AddDeviation(grpcEvaluator.deviation),
	)

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
		"execution-1",
	)

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
	ctrl := gomock.NewController(t)

	reporter := NewMockdeviationReporter(ctrl)

	httpEvaluator := &testCallEvaluator{}

	evaluator := New(
		httpEvaluator,
		&testCallEvaluator{},
		reporter,
	)

	calls := call.NewStoreFactory(
		&call.Config{
			MaxCalls:         1000,
			MaxRecordedBytes: 1024 * 1024,
		}).New()

	if errRecord := calls.Record(call.Call{
		TestExecutionID: "execution-1",
		ServiceID:       "service-1",
		HTTP:            &call.HTTPCall{},
	}); errRecord != nil {
		t.Fatalf("failed to record call: %v", errRecord)
	}

	evaluator.evaluateActualCalls(
		[]model.ServiceSpec{
			{
				ID:   "service-1",
				Kind: model.ServiceKindHTTP,
				Mode: model.ServiceModeMocked,
			},
		},
		calls,
		"execution-1",
	)
}

func TestEvaluatorEvaluateActualCallsUnsupportedServiceKind(
	t *testing.T,
) {
	ctrl := gomock.NewController(t)

	reporter := NewMockdeviationReporter(ctrl)

	evaluator := New(
		&testCallEvaluator{},
		&testCallEvaluator{},
		reporter,
	)

	calls := call.NewStoreFactory(
		&call.Config{
			MaxCalls:         1000,
			MaxRecordedBytes: 1024 * 1024,
		}).New()

	if errRecord := calls.Record(call.Call{
		TestExecutionID: "execution-1",
		ServiceID:       "unsupported-service",
		HTTP:            &call.HTTPCall{},
	}); errRecord != nil {
		t.Fatalf("failed to record call: %v", errRecord)
	}

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()

	evaluator.evaluateActualCalls(
		[]model.ServiceSpec{
			{
				ID:   "unsupported-service",
				Kind: model.ServiceKind("unsupported"),
				Mode: model.ServiceModeMocked,
			},
		},
		calls,
		"execution-1",
	)
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
			ctrl := gomock.NewController(t)

			reporter := NewMockdeviationReporter(ctrl)

			httpEvaluator := &testCallEvaluator{
				matches: tt.matches,
			}

			evaluator := New(
				httpEvaluator,
				&testCallEvaluator{},
				reporter,
			)

			calls := call.NewStoreFactory(
				&call.Config{
					MaxCalls:         1000,
					MaxRecordedBytes: 1024 * 1024,
				}).New()

			for range tt.calls {
				if errRecord := calls.Record(call.Call{
					TestExecutionID: "execution-1",
					ServiceID:       "service-1",
					HTTP:            &call.HTTPCall{},
				}); errRecord != nil {
					t.Fatalf("failed to record call: %v", errRecord)
				}
			}

			if tt.expectedDeviation != nil {
				reporter.EXPECT().
					AddDeviation(tt.expectedDeviation)
			}

			err := evaluator.evaluateExpectedCalls(
				context.Background(),
				[]model.ServiceSpec{
					{
						ID:   "service-1",
						Kind: model.ServiceKindHTTP,
					},
				},
				[]model.ServiceExpectedCalls{
					{
						ServiceID: "service-1",
						HTTP:      &model.HTTPRequestPattern{},
						Count:     tt.count,
					},
				},
				calls,
				"execution-1",
			)
			if err != nil {
				t.Fatalf(
					"evaluateExpectedCalls() error = %v",
					err,
				)
			}
		})
	}
}

func TestEvaluatorEvaluateExpectedCallsError(
	t *testing.T,
) {
	expectedErr := errors.New("match error")

	ctrl := gomock.NewController(t)

	reporter := NewMockdeviationReporter(ctrl)

	httpEvaluator := &testCallEvaluator{
		matchesErr: expectedErr,
	}

	evaluator := New(
		httpEvaluator,
		&testCallEvaluator{},
		reporter,
	)

	calls := call.NewStoreFactory(
		&call.Config{
			MaxCalls:         1000,
			MaxRecordedBytes: 1024 * 1024,
		}).New()

	if errRecord := calls.Record(call.Call{
		TestExecutionID: "execution-1",
		ServiceID:       "service-1",
		HTTP:            &call.HTTPCall{},
	}); errRecord != nil {
		t.Fatalf("failed to record call: %v", errRecord)
	}

	err := evaluator.evaluateExpectedCalls(
		context.Background(),
		[]model.ServiceSpec{
			{
				ID:   "service-1",
				Kind: model.ServiceKindHTTP,
			},
		},
		[]model.ServiceExpectedCalls{
			{
				ServiceID: "service-1",
				HTTP:      &model.HTTPRequestPattern{},
			},
		},
		calls,
		"execution-1",
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

		expected model.ServiceExpectedCalls

		expectedHTTPCalls int
		expectedGRPCCalls int
		expectedMatch     bool
		expectedPanic     bool
	}{
		{
			name: "HTTP",
			expected: model.ServiceExpectedCalls{
				HTTP: &model.HTTPRequestPattern{},
			},
			expectedHTTPCalls: 1,
			expectedMatch:     true,
		},
		{
			name: "gRPC",
			expected: model.ServiceExpectedCalls{
				GRPC: &model.GRPCRequestPattern{},
			},
			expectedGRPCCalls: 1,
			expectedMatch:     true,
		},
		{
			name:          "no protocol",
			expectedPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			reporter := NewMockdeviationReporter(ctrl)

			httpEvaluator := &testCallEvaluator{
				matches: true,
			}

			grpcEvaluator := &testCallEvaluator{
				matches: true,
			}

			evaluator := New(
				httpEvaluator,
				grpcEvaluator,
				reporter,
			)

			if tt.expectedPanic {
				defer func() {
					if recover() == nil {
						t.Fatal("expected panic")
					}
				}()

				_, _ = evaluator.matches(
					context.Background(),
					&model.ServiceSpec{},
					call.Call{},
					tt.expected,
				)

				return
			}

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

	ctrl := gomock.NewController(t)

	reporter := NewMockdeviationReporter(ctrl)

	httpEvaluator := &testCallEvaluator{
		matchesErr: expectedErr,
	}

	evaluator := New(
		httpEvaluator,
		&testCallEvaluator{},
		reporter,
	)

	_, err := evaluator.matches(
		context.Background(),
		&model.ServiceSpec{},
		call.Call{},
		model.ServiceExpectedCalls{
			HTTP: &model.HTTPRequestPattern{},
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

func TestEvaluatorEvaluateCallsTruncatedHTTPBody(t *testing.T) {
	expectedErr := errorsPkg.ErrHTTPRecordedRequestBodyTruncated

	ctrl := gomock.NewController(t)
	reporter := NewMockdeviationReporter(ctrl)

	httpEvaluator := &testCallEvaluator{
		matchesErr: expectedErr,
	}

	evaluator := New(
		httpEvaluator,
		&testCallEvaluator{},
		reporter,
	)

	calls := call.NewStoreFactory(
		&call.Config{
			MaxCalls:         1000,
			MaxRecordedBytes: 1024 * 1024,
		}).New()

	if errRecord := calls.Record(call.Call{
		TestExecutionID: "execution-1",
		ServiceID:       "http-service",
		HTTP: &call.HTTPCall{
			Request: call.HTTPRequest{
				Method:        "POST",
				Body:          []byte("partial"),
				BodyTruncated: true,
				BodyComplete:  true,
			},
		},
	}); errRecord != nil {
		t.Fatalf("failed to record call: %v", errRecord)
	}

	err := evaluator.EvaluateCalls(
		context.Background(),
		[]model.ServiceSpec{
			{
				ID:   "http-service",
				Kind: model.ServiceKindHTTP,
			},
		},
		[]model.ServiceExpectedCalls{
			{
				ServiceID: "http-service",
				HTTP:      &model.HTTPRequestPattern{},
			},
		},
		calls,
		"execution-1",
	)

	if !errors.Is(err, expectedErr) {
		t.Fatalf(
			"expected ErrHTTPRecordedRequestBodyTruncated, got %v",
			err,
		)
	}

	if httpEvaluator.matchesCalls != 1 {
		t.Errorf(
			"expected Matches() to be called once, got %d",
			httpEvaluator.matchesCalls,
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
	_ model.ServiceExpectedCalls,
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

func intPtr(value int) *int {
	return &value
}
