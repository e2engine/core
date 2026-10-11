package worker

import (
	"context"
	nativeerrors "errors"
	"net/http"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/e2engine/core/execute"
	"github.com/e2engine/core/execute/call"
	"github.com/e2engine/core/execute/evaluate"
	"github.com/e2engine/core/mock"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/errors"
)

type testCallEvaluator struct {
	matches    bool
	matchesErr error
	deviation  *model.TestExecutionDeviation
}

func (e *testCallEvaluator) Matches(
	_ context.Context,
	_ *model.ServiceSpec,
	_ call.Call,
	_ model.ServiceExpectedCalls,
) (bool, error) {
	return e.matches, e.matchesErr
}

func (e *testCallEvaluator) Deviation(
	_ *model.ServiceSpec,
	_ call.Call,
) *model.TestExecutionDeviation {
	return e.deviation
}

func newHTTPTestJob(
	callStore *call.Store,
) execute.TestJob {
	return execute.TestJob{
		ExecutionID: "execution-1",
		Runtime: execute.TestExecutionRuntime{
			Calls: callStore,
		},
		Test: model.Test{
			Spec: model.TestSpec{
				Request: model.RequestSpec{
					HTTP: &model.HTTPRequest{},
				},
				Expect: model.ExpectSpec{
					HTTP: &model.HTTPResponse{},
				},
			},
		},
	}
}

func TestValidateExpectedCalls(t *testing.T) {
	tests := []struct {
		name     string
		services []model.ServiceSpec
		expected []model.ServiceExpectedCalls
		wantErr  bool
	}{
		{
			name: "HTTP",
			services: []model.ServiceSpec{
				{
					ID:   "http-service",
					Kind: model.ServiceKindHTTP,
				},
			},
			expected: []model.ServiceExpectedCalls{
				{
					ServiceID: "http-service",
					HTTP:      &model.HTTPRequestPattern{},
				},
			},
		},
		{
			name: "gRPC",
			services: []model.ServiceSpec{
				{
					ID:   "grpc-service",
					Kind: model.ServiceKindGRPC,
				},
			},
			expected: []model.ServiceExpectedCalls{
				{
					ServiceID: "grpc-service",
					GRPC:      &model.GRPCRequestPattern{},
				},
			},
		},
		{
			name: "unknown service",
			services: []model.ServiceSpec{
				{
					ID:   "http-service",
					Kind: model.ServiceKindHTTP,
				},
			},
			expected: []model.ServiceExpectedCalls{
				{
					ServiceID: "unknown",
					HTTP:      &model.HTTPRequestPattern{},
				},
			},
			wantErr: true,
		},
		{
			name: "HTTP expectation references gRPC service",
			services: []model.ServiceSpec{
				{
					ID:   "service",
					Kind: model.ServiceKindGRPC,
				},
			},
			expected: []model.ServiceExpectedCalls{
				{
					ServiceID: "service",
					HTTP:      &model.HTTPRequestPattern{},
				},
			},
			wantErr: true,
		},
		{
			name: "gRPC expectation references HTTP service",
			services: []model.ServiceSpec{
				{
					ID:   "service",
					Kind: model.ServiceKindHTTP,
				},
			},
			expected: []model.ServiceExpectedCalls{
				{
					ServiceID: "service",
					GRPC:      &model.GRPCRequestPattern{},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateExpectedCalls(
				tt.services,
				tt.expected,
			)

			if !tt.wantErr {
				if err != nil {
					t.Fatalf(
						"validateExpectedCalls() error = %v",
						err,
					)
				}

				return
			}

			if err == nil {
				t.Fatal("expected error")
			}

			if !nativeerrors.Is(
				err,
				errors.ErrInvalidTestExecutionInput,
			) {
				t.Errorf(
					"expected %v, got %v",
					errors.ErrInvalidTestExecutionInput,
					err,
				)
			}
		})
	}
}

func TestWorkerGetExecutor(t *testing.T) {
	ctrl := gomock.NewController(t)

	httpExecutor := mock.NewMockProtocolExecutor(ctrl)
	grpcExecutor := mock.NewMockProtocolExecutor(ctrl)

	w := &Worker{
		httpExecutor: httpExecutor,
		grpcExecutor: grpcExecutor,
	}

	tests := []struct {
		name     string
		spec     *model.TestSpec
		expected execute.ProtocolExecutor
		wantErr  error
	}{
		{
			name: "HTTP",
			spec: &model.TestSpec{
				Request: model.RequestSpec{
					HTTP: &model.HTTPRequest{},
				},
			},
			expected: httpExecutor,
		},
		{
			name: "gRPC",
			spec: &model.TestSpec{
				Request: model.RequestSpec{
					GRPC: &model.GRPCRequest{},
				},
			},
			expected: grpcExecutor,
		},
		{
			name:    "unsupported",
			spec:    &model.TestSpec{},
			wantErr: errors.ErrUnsupportedTestProtocol,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := w.getExecutor(tt.spec)

			if tt.wantErr != nil {
				if !nativeerrors.Is(
					err,
					tt.wantErr,
				) {
					t.Errorf(
						"expected error %v, got %v",
						tt.wantErr,
						err,
					)
				}

				return
			}

			if err != nil {
				t.Fatalf(
					"getExecutor() error = %v",
					err,
				)
			}

			if actual != tt.expected {
				t.Errorf(
					"expected executor %T, got %T",
					tt.expected,
					actual,
				)
			}
		})
	}
}

func TestSetExecutionError(t *testing.T) {
	result := &execute.TestExecutionResult{
		Summary: &model.TestExecutionSummary{},
	}

	executionErr := nativeerrors.New(
		"execution failed",
	)

	before := time.Now().UTC()

	setExecutionError(
		result,
		executionErr,
	)

	after := time.Now().UTC()

	if result.Status != model.ExecutionStatusError {
		t.Errorf(
			"expected status %q, got %q",
			model.ExecutionStatusError,
			result.Status,
		)
	}

	if result.Summary.Error != "execution failed" {
		t.Errorf(
			"expected error %q, got %q",
			"execution failed",
			result.Summary.Error,
		)
	}

	if result.FinishedAt.Before(before) ||
		result.FinishedAt.After(after) {
		t.Errorf(
			"unexpected FinishedAt %v",
			result.FinishedAt,
		)
	}
}

func TestWorkerExecuteExecutorError(t *testing.T) {
	executionErr := nativeerrors.New(

		"execution failed",
	)
	ctrl := gomock.NewController(t)
	executor := mock.NewMockProtocolExecutor(ctrl)
	executor.EXPECT().
		Execute(
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
		).
		Return(executionErr)
	w := &Worker{
		cfg: &Config{
			TestTimeout: time.Second,
		},
		httpExecutor: executor,
	}
	callStore := &call.Store{}
	job := newHTTPTestJob(callStore)

	result, err := w.Execute(
		context.Background(),
		job,
	)
	if err != nil {
		t.Fatalf(
			"Execute() error = %v",
			err,
		)
	}

	if result == nil {
		t.Fatal("expected result")
	}

	if result.Status != model.ExecutionStatusError {
		t.Errorf(
			"expected status %q, got %q",
			model.ExecutionStatusError,
			result.Status,
		)
	}

	if result.Summary.Error != executionErr.Error() {
		t.Errorf(
			"expected summary error %q, got %q",
			executionErr.Error(),
			result.Summary.Error,
		)
	}

	if result.FinishedAt.IsZero() {
		t.Error("expected FinishedAt")
	}
}

func TestWorkerExecutePassed(t *testing.T) {
	ctrl := gomock.NewController(t)
	executor := mock.NewMockProtocolExecutor(ctrl)
	callStore := &call.Store{}

	executor.EXPECT().
		Execute(
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
		).
		DoAndReturn(func(
			_ context.Context,
			job *execute.TestJob,
			_ *execute.TestExecutionResult,
		) error {
			job.Runtime.Calls.Record(call.Call{
				TestExecutionID: job.ExecutionID,
				ServiceID:       "service-1",
				HTTP: &call.HTTPCall{
					Request: call.HTTPRequest{
						Method: http.MethodGet,
						Path:   "/health",
					},
					Response: call.HTTPResponse{
						StatusCode: http.StatusOK,
					},
				},
			})

			return nil
		})

	w := &Worker{
		cfg: &Config{
			TestTimeout: time.Second,
		},
		httpExecutor: executor,
		evaluator: evaluate.New(
			&testCallEvaluator{},
			&testCallEvaluator{},
		),
	}

	job := newHTTPTestJob(callStore)

	result, err := w.Execute(
		context.Background(),
		job,
	)
	if err != nil {
		t.Fatalf(
			"Execute() error = %v",
			err,
		)
	}

	if result.Status != model.ExecutionStatusPassed {
		t.Errorf(
			"expected status %q, got %q",
			model.ExecutionStatusPassed,
			result.Status,
		)
	}

	if len(result.Summary.Calls) != 1 {
		t.Fatalf(
			"expected 1 summarized call, got %d",
			len(result.Summary.Calls),
		)
	}

	actual := result.Summary.Calls[0]

	if actual.ServiceID != "service-1" {
		t.Errorf(
			"expected service ID %q, got %q",
			"service-1",
			actual.ServiceID,
		)
	}

	if actual.Count != 1 {
		t.Errorf(
			"expected count 1, got %d",
			actual.Count,
		)
	}

	if actual.HTTP == nil {
		t.Fatal(
			"expected HTTP call summary",
		)
	}

	if actual.HTTP.Method != http.MethodGet {
		t.Errorf(
			"expected method %q, got %q",
			http.MethodGet,
			actual.HTTP.Method,
		)
	}

	if actual.HTTP.Path != "/health" {
		t.Errorf(
			"expected path %q, got %q",
			"/health",
			actual.HTTP.Path,
		)
	}

	if actual.HTTP.ResponseStatusCode == nil ||
		*actual.HTTP.ResponseStatusCode !=
			http.StatusOK {
		t.Errorf(
			"expected status code %d, got %#v",
			http.StatusOK,
			actual.HTTP.ResponseStatusCode,
		)
	}

	if result.FinishedAt.IsZero() {
		t.Error(
			"expected FinishedAt",
		)
	}
}

func TestWorkerExecuteFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	executor := mock.NewMockProtocolExecutor(ctrl)

	executor.EXPECT().
		Execute(
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
		).
		DoAndReturn(func(
			_ context.Context,
			job *execute.TestJob,
			result *execute.TestExecutionResult,
		) error {
			job.Runtime.Calls.Record(call.Call{
				TestExecutionID: job.ExecutionID,
				ServiceID:       "service-1",
				HTTP: &call.HTTPCall{
					Request: call.HTTPRequest{
						Method: http.MethodGet,
						Path:   "/health",
					},
					Response: call.HTTPResponse{
						StatusCode: http.StatusOK,
					},
				},
			})

			result.Summary.Deviations = append(
				result.Summary.Deviations,
				model.TestExecutionDeviation{
					Field:    "status_code",
					Expected: "204",
					Actual:   "200",
				},
			)

			return nil
		})

	w := &Worker{
		cfg: &Config{
			TestTimeout: time.Second,
		},
		httpExecutor: executor,
		evaluator: evaluate.New(
			&testCallEvaluator{},
			&testCallEvaluator{},
		),
	}

	callStore := &call.Store{}
	job := newHTTPTestJob(callStore)

	result, err := w.Execute(
		context.Background(),
		job,
	)
	if err != nil {
		t.Fatalf(
			"Execute() error = %v",
			err,
		)
	}

	if result.Status != model.ExecutionStatusFailed {
		t.Errorf(
			"expected status %q, got %q",
			model.ExecutionStatusFailed,
			result.Status,
		)
	}

	if len(result.Summary.Deviations) != 1 {
		t.Fatalf(
			"expected 1 deviation, got %d",
			len(result.Summary.Deviations),
		)
	}

	if len(result.Summary.Calls) != 1 {
		t.Fatalf(
			"expected 1 summarized call, got %d",
			len(result.Summary.Calls),
		)
	}

	if result.FinishedAt.IsZero() {
		t.Error(
			"expected FinishedAt",
		)
	}
}

func TestWorkerExecuteInvalidExpectedCalls(t *testing.T) {
	ctrl := gomock.NewController(t)
	executor := mock.NewMockProtocolExecutor(ctrl)

	w := &Worker{
		cfg: &Config{
			TestTimeout: time.Second,
		},
		httpExecutor: executor,
	}

	callStore := &call.Store{}

	job := execute.TestJob{
		ExecutionID: "execution-1",
		Runtime: execute.TestExecutionRuntime{
			Calls: callStore,
		},
		Environment: model.Environment{
			Spec: model.EnvironmentSpec{
				Services: []model.ServiceSpec{
					{
						ID:   "service-1",
						Kind: model.ServiceKindHTTP,
					},
				},
			},
		},
		Test: model.Test{
			Spec: model.TestSpec{
				Request: model.RequestSpec{
					HTTP: &model.HTTPRequest{},
				},
				Expect: model.ExpectSpec{
					HTTP: &model.HTTPResponse{},
					Calls: []model.ServiceExpectedCalls{
						{
							ServiceID: "unknown",
							HTTP: &model.HTTPRequestPattern{
								Method: http.MethodGet,
								Path:   "/health",
							},
						},
					},
				},
			},
		},
	}

	result, err := w.Execute(
		context.Background(),
		job,
	)
	if err != nil {
		t.Fatalf(
			"Execute() error = %v",
			err,
		)
	}

	if result.Status != model.ExecutionStatusError {
		t.Errorf(
			"expected status %q, got %q",
			model.ExecutionStatusError,
			result.Status,
		)
	}

	if result.Summary.Error == "" {
		t.Error(
			"expected summary error",
		)
	}

	if result.Summary.Calls != nil {
		t.Errorf(
			"expected no summarized calls, got %+v",
			result.Summary.Calls,
		)
	}

	if result.FinishedAt.IsZero() {
		t.Error(
			"expected FinishedAt",
		)
	}
}

func TestWorkerExecuteEvaluateCallsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	executor := mock.NewMockProtocolExecutor(ctrl)

	executor.EXPECT().
		Execute(
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
		).
		DoAndReturn(func(
			_ context.Context,
			job *execute.TestJob,
			_ *execute.TestExecutionResult,
		) error {
			job.Runtime.Calls.Record(call.Call{
				TestExecutionID: job.ExecutionID,
				ServiceID:       "service-1",
				HTTP: &call.HTTPCall{
					Request: call.HTTPRequest{
						Method: http.MethodGet,
						Path:   "/health",
					},
					Response: call.HTTPResponse{
						StatusCode: http.StatusOK,
					},
				},
			})

			return nil
		})

	evaluationErr := nativeerrors.New(
		"evaluation failed",
	)

	w := &Worker{
		cfg: &Config{
			TestTimeout: time.Second,
		},
		httpExecutor: executor,
		evaluator: evaluate.New(
			&testCallEvaluator{
				matchesErr: evaluationErr,
			},
			&testCallEvaluator{},
		),
	}

	count := 1

	job := execute.TestJob{
		ExecutionID: "execution-1",
		Runtime: execute.TestExecutionRuntime{
			Calls: &call.Store{},
		},
		Environment: model.Environment{
			Spec: model.EnvironmentSpec{
				Services: []model.ServiceSpec{
					{
						ID:   "service-1",
						Kind: model.ServiceKindHTTP,
						Mode: model.ServiceModeReal,
					},
				},
			},
		},
		Test: model.Test{
			Spec: model.TestSpec{
				Request: model.RequestSpec{
					HTTP: &model.HTTPRequest{},
				},
				Expect: model.ExpectSpec{
					HTTP: &model.HTTPResponse{},
					Calls: []model.ServiceExpectedCalls{
						{
							ServiceID: "service-1",
							Count:     &count,
							HTTP: &model.HTTPRequestPattern{
								Method: http.MethodGet,
								Path:   "/health",
							},
						},
					},
				},
			},
		},
	}

	result, err := w.Execute(
		context.Background(),
		job,
	)
	if err != nil {
		t.Fatalf(
			"Execute() error = %v",
			err,
		)
	}

	if result.Status != model.ExecutionStatusError {
		t.Errorf(
			"expected status %q, got %q",
			model.ExecutionStatusError,
			result.Status,
		)
	}

	if result.Summary.Error !=
		evaluationErr.Error() {
		t.Errorf(
			"expected summary error %q, got %q",
			evaluationErr.Error(),
			result.Summary.Error,
		)
	}

	if len(result.Summary.Calls) != 1 {
		t.Fatalf(
			"expected 1 summarized call, got %d",
			len(result.Summary.Calls),
		)
	}

	if result.FinishedAt.IsZero() {
		t.Error(
			"expected FinishedAt",
		)
	}
}

func TestWorkerExecuteUnsupportedProtocol(t *testing.T) {
	w := &Worker{
		cfg: &Config{
			TestTimeout: time.Second,
		},
	}

	job := execute.TestJob{
		ExecutionID: "execution-1",
		Runtime: execute.TestExecutionRuntime{
			Calls: &call.Store{},
		},
		Test: model.Test{
			Spec: model.TestSpec{},
		},
	}

	result, err := w.Execute(
		context.Background(),
		job,
	)

	if !nativeerrors.Is(
		err,
		errors.ErrUnsupportedTestProtocol,
	) {
		t.Fatalf(
			"expected error %v, got %v",
			errors.ErrUnsupportedTestProtocol,
			err,
		)
	}

	if result != nil {
		t.Errorf(
			"expected nil result, got %+v",
			result,
		)
	}
}
