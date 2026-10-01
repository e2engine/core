package persist

import (
	"context"
	"encoding/json"
	"time"

	"github.com/ygrebnov/errorc"

	"github.com/e2engine/core/execute"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/keys"
	corerepository "github.com/e2engine/core/repository"
)

var ErrInvalidExecutionStatus = errorc.New("invalid execution status")

type TestExecutionRepository interface {
	GetByTestSuiteExecutionID(ctx context.Context, id string) ([]corerepository.TestExecution, error)
	SetRunning(ctx context.Context, params *SetRunningParams) error
	SetCompleted(ctx context.Context, params *SetCompletedParams) error
}

type TestSuiteExecutionRepository interface {
	SetRunning(ctx context.Context, params *SetRunningParams) error
	SetCompleted(ctx context.Context, params *SetCompletedParams) error
}

type SetRunningParams = corerepository.SetRunningParams
type SetCompletedParams = corerepository.SetCompletedParams

type Updater struct {
	testExecution      TestExecutionRepository
	testSuiteExecution TestSuiteExecutionRepository
}

func NewUpdater(
	testExecution TestExecutionRepository,
	testSuiteExecution TestSuiteExecutionRepository,
) *Updater {
	return &Updater{
		testExecution:      testExecution,
		testSuiteExecution: testSuiteExecution,
	}
}

func (u *Updater) SetTestRunning(
	ctx context.Context,
	executionID string,
	startedAt time.Time,
) error {
	return u.testExecution.SetRunning(
		ctx,
		&SetRunningParams{
			ID:        executionID,
			StartedAt: startedAt,
		},
	)
}

func (u *Updater) SetTestSuiteRunning(
	ctx context.Context,
	executionID string,
	startedAt time.Time,
) error {
	return u.testSuiteExecution.SetRunning(
		ctx,
		&SetRunningParams{
			ID:        executionID,
			StartedAt: startedAt,
		},
	)
}

func (u *Updater) SetTestCompleted(
	ctx context.Context,
	result execute.TestExecutionResult,
) (bool, error) {
	var summary json.RawMessage

	if result.Summary != nil {
		data, err := json.Marshal(result.Summary)
		if err != nil {
			return false, err
		}

		summary = data
	}

	if err := u.testExecution.SetCompleted(
		ctx,
		&SetCompletedParams{
			ID:         result.ExecutionID,
			Status:     string(result.Status),
			FinishedAt: result.FinishedAt,
			Summary:    summary,
		},
	); err != nil {
		return false, err
	}

	if result.TestSuiteExecutionID == "" {
		return true, nil
	}

	return u.tryCompleteTestSuite(
		ctx,
		result.TestSuiteExecutionID,
	)
}

func (u *Updater) SetTestSuiteCompleted(
	ctx context.Context,
	result execute.TestSuiteExecutionResult,
) (bool, error) {
	var summary json.RawMessage

	if result.Summary != nil {
		data, err := json.Marshal(result.Summary)
		if err != nil {
			return false, err
		}

		summary = data
	}

	err := u.testSuiteExecution.SetCompleted(
		ctx,
		&SetCompletedParams{
			ID:         result.ExecutionID,
			Status:     string(result.Status),
			FinishedAt: result.FinishedAt,
			Summary:    summary,
		},
	)

	return err == nil, err
}

func (u *Updater) tryCompleteTestSuite(
	ctx context.Context,
	testSuiteExecutionID string,
) (bool, error) {
	testExecutions, err := u.testExecution.GetByTestSuiteExecutionID(
		ctx,
		testSuiteExecutionID,
	)
	if err != nil {
		return false, err
	}

	summary := &model.TestSuiteExecutionSummary{
		Total: len(testExecutions),
	}

	status := model.ExecutionStatusPassed
	finishedAt := time.Time{}

	for i := range testExecutions {
		execution := testExecutions[i]

		switch model.ExecutionStatus(execution.Status) {
		case model.ExecutionStatusPassed:
			summary.Passed++

		case model.ExecutionStatusFailed:
			summary.Failed++
			if status != model.ExecutionStatusError {
				status = model.ExecutionStatusFailed
			}

		case model.ExecutionStatusError:
			summary.Errors++
			status = model.ExecutionStatusError

		case model.ExecutionStatusRunning, model.ExecutionStatusScheduled:
			return false, nil

		default:
			return false, errorc.With(
				ErrInvalidExecutionStatus,
				errorc.String(keys.ExecutionStatus, execution.Status),
			)
		}

		if execution.FinishedAt.After(finishedAt) {
			finishedAt = execution.FinishedAt
		}
	}

	return u.SetTestSuiteCompleted(
		ctx,
		execute.TestSuiteExecutionResult{
			ExecutionID: testSuiteExecutionID,
			Status:      status,
			FinishedAt:  finishedAt,
			Summary:     summary,
		},
	)
}
