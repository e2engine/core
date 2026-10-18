package worker

import (
	"context"
	nativeerrors "errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/e2engine/core/execute"
	"github.com/e2engine/core/execute/call"
	httpworker "github.com/e2engine/core/execute/worker/http"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/errors"
)

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

func TestWorkerExecutePassed(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(
				w http.ResponseWriter,
				_ *http.Request,
			) {
				w.WriteHeader(http.StatusOK)
			},
		),
	)
	defer server.Close()

	worker := newHTTPTestWorker(
		t,
		server.Client(),
	)

	job := newHTTPTestJob(
		&call.Store{},
		server.URL,
		http.StatusOK,
	)

	result, err := worker.Execute(
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

	if result.Status != model.ExecutionStatusPassed {
		t.Errorf(
			"expected status %q, got %q",
			model.ExecutionStatusPassed,
			result.Status,
		)
	}

	if len(result.Summary.Deviations) != 0 {
		t.Errorf(
			"expected no deviations, got %+v",
			result.Summary.Deviations,
		)
	}

	if result.Summary.Calls != nil {
		t.Errorf(
			"expected no summarized calls, got %+v",
			result.Summary.Calls,
		)
	}

	if result.Summary.Error != "" {
		t.Errorf(
			"expected no summary error, got %q",
			result.Summary.Error,
		)
	}

	if result.FinishedAt.IsZero() {
		t.Error("expected FinishedAt")
	}
}

func TestWorkerExecuteFailed(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(
				w http.ResponseWriter,
				_ *http.Request,
			) {
				w.WriteHeader(http.StatusTeapot)
			},
		),
	)
	defer server.Close()

	worker := newHTTPTestWorker(
		t,
		server.Client(),
	)

	job := newHTTPTestJob(
		&call.Store{},
		server.URL,
		http.StatusOK,
	)

	result, err := worker.Execute(
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

	deviation := result.Summary.Deviations[0]

	if deviation.Field != "status_code" {
		t.Errorf(
			"expected field %q, got %q",
			"status_code",
			deviation.Field,
		)
	}

	if deviation.Expected != "200" {
		t.Errorf(
			"expected expected value %q, got %q",
			"200",
			deviation.Expected,
		)
	}

	if deviation.Actual != "418" {
		t.Errorf(
			"expected actual value %q, got %q",
			"418",
			deviation.Actual,
		)
	}

	if result.Summary.Error != "" {
		t.Errorf(
			"expected no summary error, got %q",
			result.Summary.Error,
		)
	}

	if result.FinishedAt.IsZero() {
		t.Error("expected FinishedAt")
	}
}

func TestWorkerExecuteExecutorError(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(
				w http.ResponseWriter,
				_ *http.Request,
			) {
				w.WriteHeader(http.StatusOK)
			},
		),
	)

	client := server.Client()
	url := server.URL

	server.Close()

	worker := newHTTPTestWorker(
		t,
		client,
	)

	job := newHTTPTestJob(
		&call.Store{},
		url,
		http.StatusOK,
	)

	result, err := worker.Execute(
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

	if result.Summary.Error == "" {
		t.Error("expected summary error")
	}

	if result.Summary.Request == nil ||
		result.Summary.Request.HTTP == nil {
		t.Fatal(
			"expected attempted HTTP request in summary",
		)
	}

	if result.Summary.Response != nil {
		t.Errorf(
			"expected no response summary, got %+v",
			result.Summary.Response,
		)
	}

	if result.FinishedAt.IsZero() {
		t.Error("expected FinishedAt")
	}
}

func TestWorkerExecuteInvalidExpectedCalls(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(
				w http.ResponseWriter,
				_ *http.Request,
			) {
				w.WriteHeader(http.StatusOK)
			},
		),
	)
	defer server.Close()

	worker := newHTTPTestWorker(
		t,
		server.Client(),
	)

	job := newHTTPTestJob(
		&call.Store{},
		server.URL,
		http.StatusOK,
	)

	job.Environment.Spec.Services =
		[]model.ServiceSpec{
			{
				ID:   "service-1",
				Kind: model.ServiceKindHTTP,
			},
		}

	job.Test.Spec.Expect.Calls =
		[]model.ServiceExpectedCalls{
			{
				ServiceID: "unknown",
				HTTP: &model.HTTPRequestPattern{
					Method: http.MethodGet,
					Path:   "/health",
				},
			},
		}

	result, err := worker.Execute(
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
		t.Error("expected summary error")
	}

	if result.Summary.Request != nil {
		t.Errorf(
			"expected no request summary, got %+v",
			result.Summary.Request,
		)
	}

	if result.Summary.Response != nil {
		t.Errorf(
			"expected no response summary, got %+v",
			result.Summary.Response,
		)
	}

	if result.Summary.Calls != nil {
		t.Errorf(
			"expected no summarized calls, got %+v",
			result.Summary.Calls,
		)
	}

	if result.FinishedAt.IsZero() {
		t.Error("expected FinishedAt")
	}
}

func TestWorkerExecuteInvalidTest(t *testing.T) {
	worker := &Worker{
		cfg: &Config{
			TestTimeout: time.Second,
		},
		client:          http.DefaultClient,
		addressResolver: newTestAddressResolver(t),
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

	result, err := worker.Execute(
		context.Background(),
		job,
	)
	if err != nil {
		t.Fatalf(
			"expected nil error, got %v",
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

	if result.Summary.Error == "" {
		t.Error("expected summary error")
	}

	if result.Summary.Request != nil {
		t.Errorf(
			"expected no request summary, got %+v",
			result.Summary.Request,
		)
	}

	if result.FinishedAt.IsZero() {
		t.Error("expected FinishedAt")
	}
}

func TestWorkerExecuteNilReporterConfig(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(
				w http.ResponseWriter,
				_ *http.Request,
			) {
				w.WriteHeader(http.StatusOK)
			},
		),
	)
	defer server.Close()

	worker := &Worker{
		cfg: &Config{
			TestTimeout: time.Second,
			HTTPEvaluator: &httpworker.EvaluatorConfig{
				MaxResponseBodySize: 1024 * 1024, // 1 MB
			},
			Reporter: nil,
		},
		client:          server.Client(),
		addressResolver: newTestAddressResolver(t),
	}

	job := newHTTPTestJob(
		&call.Store{},
		server.URL,
		http.StatusOK,
	)

	result, err := worker.Execute(
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
}

func newHTTPTestWorker(
	t *testing.T,
	client *http.Client,
) *Worker {
	t.Helper()

	return &Worker{
		cfg: &Config{
			TestTimeout: time.Second,
			HTTPEvaluator: &httpworker.EvaluatorConfig{
				MaxResponseBodySize: 1024 * 1024, // 1 MB
			},
		},
		client:          client,
		addressResolver: newTestAddressResolver(t),
	}
}

func newTestAddressResolver(t *testing.T) *MockaddressResolver {
	t.Helper()

	ctrl := gomock.NewController(t)
	resolver := NewMockaddressResolver(ctrl)

	resolver.EXPECT().
		ResolveHTTPURL(gomock.Any()).
		DoAndReturn(func(value string) (string, error) {
			return value, nil
		}).
		AnyTimes()

	return resolver
}

func newHTTPTestJob(
	callStore *call.Store,
	url string,
	expectedStatus int,
) execute.TestJob {
	return execute.TestJob{
		ExecutionID: "execution-1",
		Runtime: execute.TestExecutionRuntime{
			Calls: callStore,
		},
		Test: model.Test{
			Kind:    model.ResourceKindTest,
			Name:    "test-1",
			Version: "1.0.0",
			Spec: model.TestSpec{
				Request: model.RequestSpec{
					HTTP: &model.HTTPRequest{
						Method: http.MethodGet,
						URL:    url,
					},
				},
				Expect: model.ExpectSpec{
					HTTP: &model.HTTPResponsePattern{
						StatusCode: expectedStatus,
					},
				},
			},
		},
	}
}
