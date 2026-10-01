package worker

import (
	"context"
	nativeerrors "errors"
	"testing"
	"time"

	"github.com/e2engine/core/execute"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/errors"
)

type testProtocolExecutor struct {
	err error
}

func (e *testProtocolExecutor) Execute(
	_ context.Context,
	_ *execute.TestJob,
	_ *execute.TestExecutionResult,
) error {
	return e.err
}

func TestValidateExpectedCalls(t *testing.T) {
	tests := []struct {
		name     string
		services []model.ServiceSpec
		expected []model.CallExpectation
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
			expected: []model.CallExpectation{
				{
					ServiceID: "http-service",
					HTTP:      &model.HTTPCallExpectation{},
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
			expected: []model.CallExpectation{
				{
					ServiceID: "grpc-service",
					GRPC:      &model.GRPCCallExpectation{},
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
			expected: []model.CallExpectation{
				{
					ServiceID: "unknown",
					HTTP:      &model.HTTPCallExpectation{},
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
			expected: []model.CallExpectation{
				{
					ServiceID: "service",
					HTTP:      &model.HTTPCallExpectation{},
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
			expected: []model.CallExpectation{
				{
					ServiceID: "service",
					GRPC:      &model.GRPCCallExpectation{},
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
	httpExecutor := &testProtocolExecutor{}
	grpcExecutor := &testProtocolExecutor{}

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
					HTTP: &model.HTTPRequestSpec{},
				},
			},
			expected: httpExecutor,
		},
		{
			name: "gRPC",
			spec: &model.TestSpec{
				Request: model.RequestSpec{
					GRPC: &model.GRPCRequestSpec{},
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
				if !nativeerrors.Is(err, tt.wantErr) {
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

	w := &Worker{
		cfg: &Config{
			TestTimeout: time.Second,
		},
		httpExecutor: &testProtocolExecutor{
			err: executionErr,
		},
	}

	job := execute.TestJob{
		ExecutionID: "execution-1",
		Test: model.Test{
			Spec: model.TestSpec{
				Request: model.RequestSpec{
					HTTP: &model.HTTPRequestSpec{},
				},
				Expect: model.ExpectSpec{
					HTTP: &model.HTTPExpectSpec{},
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

	if result.Summary.Calls != nil {
		t.Errorf(
			"expected no calls summary, got %#v",
			result.Summary.Calls,
		)
	}

	if result.FinishedAt.IsZero() {
		t.Error("expected FinishedAt")
	}
}
