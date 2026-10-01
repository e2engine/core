package persist

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/e2engine/core/execute"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/repository"
)

func TestUpdaterSetTestRunning(t *testing.T) {
	startedAt := time.Date(
		2026, 9, 18, 10, 0, 0, 0, time.UTC,
	)

	testRepository := &testExecutionRepository{}
	updater := NewUpdater(
		testRepository,
		&testSuiteExecutionRepository{},
	)

	err := updater.SetTestRunning(
		context.Background(),
		"test-execution-1",
		startedAt,
	)
	if err != nil {
		t.Fatalf("SetTestRunning() error = %v", err)
	}

	if testRepository.running == nil {
		t.Fatal("expected SetRunning() to be called")
	}

	if testRepository.running.ID != "test-execution-1" {
		t.Errorf(
			"expected ID test-execution-1, got %s",
			testRepository.running.ID,
		)
	}

	if !testRepository.running.StartedAt.Equal(startedAt) {
		t.Errorf(
			"expected StartedAt %v, got %v",
			startedAt,
			testRepository.running.StartedAt,
		)
	}
}

func TestUpdaterSetTestRunningError(t *testing.T) {
	expectedErr := errors.New("set running")

	updater := NewUpdater(
		&testExecutionRepository{
			setRunningErr: expectedErr,
		},
		&testSuiteExecutionRepository{},
	)

	err := updater.SetTestRunning(
		context.Background(),
		"test-execution-1",
		time.Now(),
	)

	if !errors.Is(err, expectedErr) {
		t.Fatalf(
			"expected error %v, got %v",
			expectedErr,
			err,
		)
	}
}

func TestUpdaterSetTestSuiteRunning(t *testing.T) {
	startedAt := time.Date(
		2026, 9, 18, 10, 0, 0, 0, time.UTC,
	)

	suiteRepository := &testSuiteExecutionRepository{}
	updater := NewUpdater(
		&testExecutionRepository{},
		suiteRepository,
	)

	err := updater.SetTestSuiteRunning(
		context.Background(),
		"test-suite-execution-1",
		startedAt,
	)
	if err != nil {
		t.Fatalf("SetTestSuiteRunning() error = %v", err)
	}

	if suiteRepository.running == nil {
		t.Fatal("expected SetRunning() to be called")
	}

	if suiteRepository.running.ID !=
		"test-suite-execution-1" {
		t.Errorf(
			"expected ID test-suite-execution-1, got %s",
			suiteRepository.running.ID,
		)
	}

	if !suiteRepository.running.StartedAt.Equal(startedAt) {
		t.Errorf(
			"expected StartedAt %v, got %v",
			startedAt,
			suiteRepository.running.StartedAt,
		)
	}
}

func TestUpdaterSetTestSuiteRunningError(t *testing.T) {
	expectedErr := errors.New("set suite running")

	updater := NewUpdater(
		&testExecutionRepository{},
		&testSuiteExecutionRepository{
			setRunningErr: expectedErr,
		},
	)

	err := updater.SetTestSuiteRunning(
		context.Background(),
		"test-suite-execution-1",
		time.Now(),
	)

	if !errors.Is(err, expectedErr) {
		t.Fatalf(
			"expected error %v, got %v",
			expectedErr,
			err,
		)
	}
}

func TestUpdaterSetTestCompletedStandalone(t *testing.T) {
	finishedAt := time.Date(
		2026, 9, 18, 10, 1, 0, 0, time.UTC,
	)

	summary := &model.TestExecutionSummary{
		Error: "test error",
	}

	testRepository := &testExecutionRepository{}
	updater := NewUpdater(
		testRepository,
		&testSuiteExecutionRepository{},
	)

	completed, err := updater.SetTestCompleted(
		context.Background(),
		execute.TestExecutionResult{
			ExecutionID: "test-execution-1",
			Status:      model.ExecutionStatusError,
			FinishedAt:  finishedAt,
			Summary:     summary,
		},
	)
	if err != nil {
		t.Fatalf("SetTestCompleted() error = %v", err)
	}

	if !completed {
		t.Fatal("expected standalone test to be completed")
	}

	if testRepository.completed == nil {
		t.Fatal("expected SetCompleted() to be called")
	}

	if testRepository.completed.ID != "test-execution-1" {
		t.Errorf(
			"expected ID test-execution-1, got %s",
			testRepository.completed.ID,
		)
	}

	if testRepository.completed.Status !=
		string(model.ExecutionStatusError) {
		t.Errorf(
			"expected status %s, got %s",
			model.ExecutionStatusError,
			testRepository.completed.Status,
		)
	}

	if !testRepository.completed.FinishedAt.Equal(
		finishedAt,
	) {
		t.Errorf(
			"expected FinishedAt %v, got %v",
			finishedAt,
			testRepository.completed.FinishedAt,
		)
	}

	var actual model.TestExecutionSummary
	if err := json.Unmarshal(
		testRepository.completed.Summary,
		&actual,
	); err != nil {
		t.Fatalf("cannot unmarshal summary: %v", err)
	}

	if actual.Error != summary.Error {
		t.Errorf(
			"expected summary error %q, got %q",
			summary.Error,
			actual.Error,
		)
	}
}

func TestUpdaterSetTestCompletedNilSummary(t *testing.T) {
	testRepository := &testExecutionRepository{}
	updater := NewUpdater(
		testRepository,
		&testSuiteExecutionRepository{},
	)

	completed, err := updater.SetTestCompleted(
		context.Background(),
		execute.TestExecutionResult{
			ExecutionID: "test-execution-1",
			Status:      model.ExecutionStatusPassed,
			FinishedAt:  time.Now(),
		},
	)
	if err != nil {
		t.Fatalf("SetTestCompleted() error = %v", err)
	}

	if !completed {
		t.Fatal("expected standalone test to be completed")
	}

	if testRepository.completed.Summary != nil {
		t.Errorf(
			"expected nil summary, got %s",
			testRepository.completed.Summary,
		)
	}
}

func TestUpdaterSetTestCompletedError(t *testing.T) {
	expectedErr := errors.New("set completed")

	updater := NewUpdater(
		&testExecutionRepository{
			setCompletedErr: expectedErr,
		},
		&testSuiteExecutionRepository{},
	)

	completed, err := updater.SetTestCompleted(
		context.Background(),
		execute.TestExecutionResult{
			ExecutionID: "test-execution-1",
			Status:      model.ExecutionStatusPassed,
		},
	)

	if !errors.Is(err, expectedErr) {
		t.Fatalf(
			"expected error %v, got %v",
			expectedErr,
			err,
		)
	}

	if completed {
		t.Fatal("expected completed to be false")
	}
}

func TestUpdaterSetTestCompletedSuiteRunning(t *testing.T) {
	testRepository := &testExecutionRepository{
		executions: []repository.TestExecution{
			{
				ID:         "test-execution-1",
				Status:     string(model.ExecutionStatusPassed),
				FinishedAt: time.Now(),
			},
			{
				ID:     "test-execution-2",
				Status: string(model.ExecutionStatusRunning),
			},
		},
	}

	suiteRepository := &testSuiteExecutionRepository{}

	updater := NewUpdater(
		testRepository,
		suiteRepository,
	)

	completed, err := updater.SetTestCompleted(
		context.Background(),
		execute.TestExecutionResult{
			ExecutionID:          "test-execution-1",
			TestSuiteExecutionID: "suite-execution-1",
			Status:               model.ExecutionStatusPassed,
			FinishedAt:           time.Now(),
		},
	)
	if err != nil {
		t.Fatalf("SetTestCompleted() error = %v", err)
	}

	if completed {
		t.Fatal(
			"expected suite to remain incomplete",
		)
	}

	if suiteRepository.completed != nil {
		t.Fatal(
			"expected suite SetCompleted() not to be called",
		)
	}
}

func TestUpdaterSetTestCompletedSuiteScheduled(t *testing.T) {
	testRepository := &testExecutionRepository{
		executions: []repository.TestExecution{
			{
				Status: string(
					model.ExecutionStatusPassed,
				),
				FinishedAt: time.Now(),
			},
			{
				Status: string(
					model.ExecutionStatusScheduled,
				),
			},
		},
	}

	updater := NewUpdater(
		testRepository,
		&testSuiteExecutionRepository{},
	)

	completed, err := updater.SetTestCompleted(
		context.Background(),
		execute.TestExecutionResult{
			ExecutionID:          "test-execution-1",
			TestSuiteExecutionID: "suite-execution-1",
			Status:               model.ExecutionStatusPassed,
			FinishedAt:           time.Now(),
		},
	)
	if err != nil {
		t.Fatalf("SetTestCompleted() error = %v", err)
	}

	if completed {
		t.Fatal(
			"expected suite to remain incomplete",
		)
	}
}

func TestUpdaterSetTestCompletedSuitePassed(t *testing.T) {
	firstFinishedAt := time.Date(
		2026, 9, 18, 10, 1, 0, 0, time.UTC,
	)
	secondFinishedAt := firstFinishedAt.Add(time.Minute)

	testRepository := &testExecutionRepository{
		executions: []repository.TestExecution{
			{
				Status: string(
					model.ExecutionStatusPassed,
				),
				FinishedAt: firstFinishedAt,
			},
			{
				Status: string(
					model.ExecutionStatusPassed,
				),
				FinishedAt: secondFinishedAt,
			},
		},
	}

	suiteRepository := &testSuiteExecutionRepository{}

	updater := NewUpdater(
		testRepository,
		suiteRepository,
	)

	completed, err := updater.SetTestCompleted(
		context.Background(),
		execute.TestExecutionResult{
			ExecutionID:          "test-execution-2",
			TestSuiteExecutionID: "suite-execution-1",
			Status:               model.ExecutionStatusPassed,
			FinishedAt:           secondFinishedAt,
		},
	)
	if err != nil {
		t.Fatalf("SetTestCompleted() error = %v", err)
	}

	if !completed {
		t.Fatal("expected suite to be completed")
	}

	assertSuiteCompleted(
		t,
		suiteRepository.completed,
		model.ExecutionStatusPassed,
		secondFinishedAt,
		model.TestSuiteExecutionSummary{
			Total:  2,
			Passed: 2,
		},
	)
}

func TestUpdaterSetTestCompletedSuiteFailed(t *testing.T) {
	finishedAt := time.Date(
		2026, 9, 18, 10, 2, 0, 0, time.UTC,
	)

	testRepository := &testExecutionRepository{
		executions: []repository.TestExecution{
			{
				Status: string(
					model.ExecutionStatusPassed,
				),
				FinishedAt: finishedAt.Add(-time.Minute),
			},
			{
				Status: string(
					model.ExecutionStatusFailed,
				),
				FinishedAt: finishedAt,
			},
		},
	}

	suiteRepository := &testSuiteExecutionRepository{}

	updater := NewUpdater(
		testRepository,
		suiteRepository,
	)

	completed, err := updater.SetTestCompleted(
		context.Background(),
		execute.TestExecutionResult{
			ExecutionID:          "test-execution-2",
			TestSuiteExecutionID: "suite-execution-1",
			Status:               model.ExecutionStatusFailed,
			FinishedAt:           finishedAt,
		},
	)
	if err != nil {
		t.Fatalf("SetTestCompleted() error = %v", err)
	}

	if !completed {
		t.Fatal("expected suite to be completed")
	}

	assertSuiteCompleted(
		t,
		suiteRepository.completed,
		model.ExecutionStatusFailed,
		finishedAt,
		model.TestSuiteExecutionSummary{
			Total:  2,
			Passed: 1,
			Failed: 1,
		},
	)
}

func TestUpdaterSetTestCompletedSuiteErrorPrecedence(
	t *testing.T,
) {
	finishedAt := time.Date(
		2026, 9, 18, 10, 3, 0, 0, time.UTC,
	)

	testRepository := &testExecutionRepository{
		executions: []repository.TestExecution{
			{
				Status: string(
					model.ExecutionStatusPassed,
				),
				FinishedAt: finishedAt.Add(-3 * time.Minute),
			},
			{
				Status: string(
					model.ExecutionStatusError,
				),
				FinishedAt: finishedAt.Add(-2 * time.Minute),
			},
			{
				Status: string(
					model.ExecutionStatusFailed,
				),
				FinishedAt: finishedAt,
			},
		},
	}

	suiteRepository := &testSuiteExecutionRepository{}

	updater := NewUpdater(
		testRepository,
		suiteRepository,
	)

	completed, err := updater.SetTestCompleted(
		context.Background(),
		execute.TestExecutionResult{
			ExecutionID:          "test-execution-3",
			TestSuiteExecutionID: "suite-execution-1",
			Status:               model.ExecutionStatusFailed,
			FinishedAt:           finishedAt,
		},
	)
	if err != nil {
		t.Fatalf("SetTestCompleted() error = %v", err)
	}

	if !completed {
		t.Fatal("expected suite to be completed")
	}

	assertSuiteCompleted(
		t,
		suiteRepository.completed,
		model.ExecutionStatusError,
		finishedAt,
		model.TestSuiteExecutionSummary{
			Total:  3,
			Passed: 1,
			Failed: 1,
			Errors: 1,
		},
	)
}

func TestUpdaterSetTestCompletedInvalidStatus(t *testing.T) {
	updater := NewUpdater(
		&testExecutionRepository{
			executions: []repository.TestExecution{
				{
					Status: "invalid",
				},
			},
		},
		&testSuiteExecutionRepository{},
	)

	completed, err := updater.SetTestCompleted(
		context.Background(),
		execute.TestExecutionResult{
			ExecutionID:          "test-execution-1",
			TestSuiteExecutionID: "suite-execution-1",
			Status:               model.ExecutionStatusPassed,
		},
	)

	if !errors.Is(err, ErrInvalidExecutionStatus) {
		t.Fatalf(
			"expected ErrInvalidExecutionStatus, got %v",
			err,
		)
	}

	if completed {
		t.Fatal("expected completed to be false")
	}
}

func TestUpdaterSetTestCompletedGetSuiteExecutionsError(
	t *testing.T,
) {
	expectedErr := errors.New("get executions")

	updater := NewUpdater(
		&testExecutionRepository{
			getErr: expectedErr,
		},
		&testSuiteExecutionRepository{},
	)

	completed, err := updater.SetTestCompleted(
		context.Background(),
		execute.TestExecutionResult{
			ExecutionID:          "test-execution-1",
			TestSuiteExecutionID: "suite-execution-1",
			Status:               model.ExecutionStatusPassed,
		},
	)

	if !errors.Is(err, expectedErr) {
		t.Fatalf(
			"expected error %v, got %v",
			expectedErr,
			err,
		)
	}

	if completed {
		t.Fatal("expected completed to be false")
	}
}

func TestUpdaterSetTestSuiteCompleted(t *testing.T) {
	finishedAt := time.Date(
		2026, 9, 18, 10, 5, 0, 0, time.UTC,
	)

	summary := &model.TestSuiteExecutionSummary{
		Total:  3,
		Passed: 3,
	}

	suiteRepository := &testSuiteExecutionRepository{}
	updater := NewUpdater(
		&testExecutionRepository{},
		suiteRepository,
	)

	completed, err := updater.SetTestSuiteCompleted(
		context.Background(),
		execute.TestSuiteExecutionResult{
			ExecutionID: "suite-execution-1",
			Status:      model.ExecutionStatusPassed,
			FinishedAt:  finishedAt,
			Summary:     summary,
		},
	)
	if err != nil {
		t.Fatalf(
			"SetTestSuiteCompleted() error = %v",
			err,
		)
	}

	if !completed {
		t.Fatal("expected completed to be true")
	}

	assertSuiteCompleted(
		t,
		suiteRepository.completed,
		model.ExecutionStatusPassed,
		finishedAt,
		*summary,
	)
}

func TestUpdaterSetTestSuiteCompletedNilSummary(
	t *testing.T,
) {
	suiteRepository := &testSuiteExecutionRepository{}
	updater := NewUpdater(
		&testExecutionRepository{},
		suiteRepository,
	)

	completed, err := updater.SetTestSuiteCompleted(
		context.Background(),
		execute.TestSuiteExecutionResult{
			ExecutionID: "suite-execution-1",
			Status:      model.ExecutionStatusError,
			FinishedAt:  time.Now(),
		},
	)
	if err != nil {
		t.Fatalf(
			"SetTestSuiteCompleted() error = %v",
			err,
		)
	}

	if !completed {
		t.Fatal("expected completed to be true")
	}

	if suiteRepository.completed.Summary != nil {
		t.Errorf(
			"expected nil summary, got %s",
			suiteRepository.completed.Summary,
		)
	}
}

func TestUpdaterSetTestSuiteCompletedError(t *testing.T) {
	expectedErr := errors.New("set suite completed")

	updater := NewUpdater(
		&testExecutionRepository{},
		&testSuiteExecutionRepository{
			setCompletedErr: expectedErr,
		},
	)

	completed, err := updater.SetTestSuiteCompleted(
		context.Background(),
		execute.TestSuiteExecutionResult{
			ExecutionID: "suite-execution-1",
			Status:      model.ExecutionStatusError,
		},
	)

	if !errors.Is(err, expectedErr) {
		t.Fatalf(
			"expected error %v, got %v",
			expectedErr,
			err,
		)
	}

	if completed {
		t.Fatal("expected completed to be false")
	}
}

func assertSuiteCompleted(
	t *testing.T,
	params *SetCompletedParams,
	status model.ExecutionStatus,
	finishedAt time.Time,
	expectedSummary model.TestSuiteExecutionSummary,
) {
	t.Helper()

	if params == nil {
		t.Fatal(
			"expected suite SetCompleted() to be called",
		)
	}

	if params.Status != string(status) {
		t.Errorf(
			"expected status %s, got %s",
			status,
			params.Status,
		)
	}

	if !params.FinishedAt.Equal(finishedAt) {
		t.Errorf(
			"expected FinishedAt %v, got %v",
			finishedAt,
			params.FinishedAt,
		)
	}

	var actualSummary model.TestSuiteExecutionSummary
	if err := json.Unmarshal(
		params.Summary,
		&actualSummary,
	); err != nil {
		t.Fatalf(
			"cannot unmarshal suite summary: %v",
			err,
		)
	}

	if actualSummary != expectedSummary {
		t.Errorf(
			"expected summary %+v, got %+v",
			expectedSummary,
			actualSummary,
		)
	}
}

type testExecutionRepository struct {
	executions []repository.TestExecution

	running   *SetRunningParams
	completed *SetCompletedParams

	getErr          error
	setRunningErr   error
	setCompletedErr error
}

func (r *testExecutionRepository) GetByTestSuiteExecutionID(
	_ context.Context,
	_ string,
) ([]repository.TestExecution, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}

	return r.executions, nil
}

func (r *testExecutionRepository) SetRunning(
	_ context.Context,
	params *SetRunningParams,
) error {
	r.running = params

	return r.setRunningErr
}

func (r *testExecutionRepository) SetCompleted(
	_ context.Context,
	params *SetCompletedParams,
) error {
	r.completed = params

	return r.setCompletedErr
}

type testSuiteExecutionRepository struct {
	running   *SetRunningParams
	completed *SetCompletedParams

	setRunningErr   error
	setCompletedErr error
}

func (r *testSuiteExecutionRepository) SetRunning(
	_ context.Context,
	params *SetRunningParams,
) error {
	r.running = params

	return r.setRunningErr
}

func (r *testSuiteExecutionRepository) SetCompleted(
	_ context.Context,
	params *SetCompletedParams,
) error {
	r.completed = params

	return r.setCompletedErr
}
