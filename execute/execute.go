package execute

import (
	"context"
	"time"

	"github.com/e2engine/core/execute/call"
	"github.com/e2engine/core/model"
)

type TestRunRequest struct {
	Environment *model.Environment `json:"environment" yaml:"environment"`
	Test        *model.Test        `json:"test" yaml:"test"`
}

// TestSuiteRunRequest is the input to a TestSuiteRunner.
type TestSuiteRunRequest struct {
	Environment *model.Environment `json:"environment" yaml:"environment"`
	TestSuite   *model.TestSuite   `json:"testsuite" yaml:"testsuite"`
	Tests       []model.Test       `json:"tests" yaml:"tests"`
}

// TestJob represents a job to execute a test against a specific environment.
// TestJob is the smallest executable unit.
type TestJob struct {
	ExecutionID          string
	TestSuiteExecutionID string
	Environment          model.Environment
	Test                 model.Test
	Runtime              TestExecutionRuntime
}

func (j *TestJob) GetEnvironmentInstanceID() string {
	if j.TestSuiteExecutionID != "" {
		return j.TestSuiteExecutionID
	}

	return j.ExecutionID
}

type TestExecutionRuntime struct {
	Calls *call.Store
}

type Job interface{ TestJob } // will expand in v2.

type JobExecutionResult interface {
	TestExecutionResult | TestSuiteExecutionResult
}

type Worker[J Job, R JobExecutionResult] interface {
	Execute(ctx context.Context, job J) (*R, error)
}

type ProtocolExecutor interface {
	Execute(
		ctx context.Context,
		job *TestJob,
		result *TestExecutionResult,
	) error
}

type TestExecutionResult struct {
	ExecutionID          string
	TestSuiteExecutionID string
	Status               model.ExecutionStatus
	FinishedAt           time.Time
	Summary              *model.TestExecutionSummary
}

func (r *TestExecutionResult) GetEnvironmentInstanceID() string {
	if r.TestSuiteExecutionID != "" {
		return r.TestSuiteExecutionID
	}

	return r.ExecutionID
}

type TestSuiteExecutionResult struct {
	ExecutionID string
	Status      model.ExecutionStatus
	FinishedAt  time.Time
	Summary     *model.TestSuiteExecutionSummary
}
