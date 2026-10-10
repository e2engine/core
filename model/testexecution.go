package model

import (
	"time"
)

// ExecutionStatus is the canonical status of a test (or testsuite) execution.
type ExecutionStatus string

const (
	ExecutionStatusScheduled ExecutionStatus = "scheduled"
	ExecutionStatusRunning   ExecutionStatus = "running"
	ExecutionStatusPassed    ExecutionStatus = "passed"
	ExecutionStatusFailed    ExecutionStatus = "failed"
	ExecutionStatusError     ExecutionStatus = "error"
)

// TestExecution is a transport-agnostic view of a single test run result.
type TestExecution struct {
	ID         string          `json:"id" yaml:"id"`
	StartedAt  time.Time       `json:"started_at" yaml:"started_at"`
	FinishedAt time.Time       `json:"finished_at" yaml:"finished_at"`
	Status     ExecutionStatus `json:"status" yaml:"status"`

	EnvironmentID   string `json:"environment_id" yaml:"environment_id"`
	EnvironmentName string `json:"environment_name,omitempty" yaml:"environment_name,omitempty"`

	TestID   string `json:"test_id" yaml:"test_id"`
	TestName string `json:"test_name,omitempty" yaml:"test_name,omitempty"`

	Summary *TestExecutionSummary `json:"summary,omitempty" yaml:"summary,omitempty"`
}

// TestExecutionSummary is a stable (for v1) structured representation of the
// summary_json stored on TestExecution.
type TestExecutionSummary struct {
	// Error is set when Status == "error" (transport errors, invalid JSON, etc.).
	Error string `json:"error,omitempty" yaml:"error,omitempty"`

	// Request captures the resolved request that was executed.
	Request *TestExecutionRequestSummary `json:"request,omitempty" yaml:"request,omitempty"`

	// Response captures what we observed.
	Response *TestExecutionResponseSummary `json:"response,omitempty" yaml:"response,omitempty"`

	// ExpectedCalls captures the service call expectations that were evaluated.
	ExpectedCalls []TestExecutionCallExpectationSummary `json:"expected_calls,omitempty" yaml:"expected_calls,omitempty"`

	// Deviations contains assertion failures.
	// When empty and Error is empty, Status is typically "passed".
	Deviations []TestExecutionDeviation `json:"deviations,omitempty" yaml:"deviations,omitempty"`
}

type TestExecutionRequestSummary struct {
	HTTP *TestExecutionHTTPRequestSummary `json:"http,omitempty" yaml:"http,omitempty"`
	GRPC *TestExecutionGRPCRequestSummary `json:"grpc,omitempty" yaml:"grpc,omitempty"`
}

type TestExecutionResponseSummary struct {
	HTTP *TestExecutionHTTPResponseSummary `json:"http,omitempty" yaml:"http,omitempty"`
	GRPC *TestExecutionGRPCResponseSummary `json:"grpc,omitempty" yaml:"grpc,omitempty"`
}

type TestExecutionHTTPRequestSummary struct {
	Method   string              `json:"method,omitempty" yaml:"method,omitempty"`
	URL      string              `json:"url,omitempty" yaml:"url,omitempty"`
	Headers  map[string][]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	BodyJSON string              `json:"body_json,omitempty" yaml:"body_json,omitempty"`
}

type TestExecutionGRPCRequestSummary struct {
	Service  string              `json:"service,omitempty" yaml:"service,omitempty"`
	Method   string              `json:"method,omitempty" yaml:"method,omitempty"`
	Metadata map[string][]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Message  map[string]any      `json:"message,omitempty" yaml:"message,omitempty"`
}

type TestExecutionHTTPResponseSummary struct {
	StatusCode int                 `json:"status_code,omitempty" yaml:"status_code,omitempty"`
	Headers    map[string][]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	BodyJSON   string              `json:"body_json,omitempty" yaml:"body_json,omitempty"`
	BodyText   string              `json:"body_text,omitempty" yaml:"body_text,omitempty"`
}

type TestExecutionGRPCResponseSummary struct {
	Status   string              `json:"status,omitempty" yaml:"status,omitempty"`
	Metadata map[string][]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Message  map[string]any      `json:"message,omitempty" yaml:"message,omitempty"`
}

type TestExecutionGRPCCallExpectationSummary struct {
	Service  string              `json:"service,omitempty" yaml:"service,omitempty"`
	Method   string              `json:"method,omitempty" yaml:"method,omitempty"`
	Metadata map[string][]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Message  map[string]any      `json:"message,omitempty" yaml:"message,omitempty"`
}

type TestExecutionCallExpectationSummary struct {
	ServiceID string `json:"service_id" yaml:"service_id"`
	Count     *int   `json:"count,omitempty" yaml:"count,omitempty"`

	HTTP *TestExecutionHTTPCallExpectationSummary `json:"http,omitempty" yaml:"http,omitempty"`
	GRPC *TestExecutionGRPCCallExpectationSummary `json:"grpc,omitempty" yaml:"grpc,omitempty"`
}

type TestExecutionHTTPCallExpectationSummary struct {
	Method   string              `json:"method,omitempty" yaml:"method,omitempty"`
	Path     string              `json:"path,omitempty" yaml:"path,omitempty"`
	Query    map[string][]string `json:"query,omitempty" yaml:"query,omitempty"`
	Headers  map[string][]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	BodyJSON string              `json:"body_json,omitempty" yaml:"body_json,omitempty"`
}

type TestExecutionDeviation struct {
	Field    string `json:"field" yaml:"field"` // e.g. "status_code", "body_json"
	Expected string `json:"expected,omitempty" yaml:"expected,omitempty"`
	Actual   string `json:"actual,omitempty" yaml:"actual,omitempty"`
	Message  string `json:"message,omitempty" yaml:"message,omitempty"`
}

type TestExecutionsPage = Page[TestExecution]
