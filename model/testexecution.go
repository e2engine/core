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

	// Calls captures the actual service calls.
	Calls []ServiceObservedCalls `json:"calls,omitempty" yaml:"calls,omitempty"`

	// Deviations contains assertion failures.
	// When empty and Error is empty, Status is typically "passed".
	Deviations []TestExecutionDeviation `json:"deviations,omitempty" yaml:"deviations,omitempty"`
}

type TestExecutionRequestSummary struct {
	HTTP *HTTPRequest `json:"http,omitempty" yaml:"http,omitempty"`
	GRPC *GRPCRequest `json:"grpc,omitempty" yaml:"grpc,omitempty"`
}

type TestExecutionResponseSummary struct {
	HTTP *HTTPResponse `json:"http,omitempty" yaml:"http,omitempty"`
	GRPC *GRPCResponse `json:"grpc,omitempty" yaml:"grpc,omitempty"`
}

// ServiceObservedCalls is an aggregated, diagnostic view of calls observed
// for a service during test execution. Calls are grouped by request identity
// and response status so dependency outcomes are visible without exposing raw
// request or response data. Response status is diagnostic only and does not
// participate in expected-call assertions, which verify the SUT's outgoing
// requests and call counts independently of dependency behavior.
type ServiceObservedCalls struct {
	ServiceID string `json:"service_id" yaml:"service_id"`
	Count     int    `json:"count" yaml:"count"`

	HTTP *HTTPServiceObservedCall `json:"http,omitempty" yaml:"http,omitempty"`
	GRPC *GRPCServiceObservedCall `json:"grpc,omitempty" yaml:"grpc,omitempty"`
}

type HTTPServiceObservedCall struct {
	Method             string `json:"method,omitempty" yaml:"method,omitempty"`
	Path               string `json:"path,omitempty" yaml:"path,omitempty"`
	ResponseStatusCode *int   `json:"response_status_code,omitempty" yaml:"response_status_code,omitempty"`
}

type GRPCServiceObservedCall struct {
	Service        string `json:"service,omitempty" yaml:"service,omitempty"`
	Method         string `json:"method,omitempty" yaml:"method,omitempty"`
	ResponseStatus string `json:"response_status,omitempty" yaml:"response_status,omitempty"`
}

type TestExecutionDeviation struct {
	Field    string `json:"field" yaml:"field"` // e.g. "status_code", "body"
	Expected string `json:"expected,omitempty" yaml:"expected,omitempty"`
	Actual   string `json:"actual,omitempty" yaml:"actual,omitempty"`
	Message  string `json:"message,omitempty" yaml:"message,omitempty"`
}

type Stringer interface {
	String() string
}

type PayloadDeviation struct {
	Field    string
	Expected Stringer
	Actual   []byte
	Message  string
}

type TestExecutionsPage = Page[TestExecution]
