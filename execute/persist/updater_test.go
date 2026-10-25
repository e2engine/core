package persist

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/e2engine/core/execute"
	"github.com/e2engine/core/mock"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/repository"
)

const (
	testExecutionID  = "test-execution-1"
	suiteExecutionID = "suite-execution-1"
)

func TestUpdaterSetTestRunning(t *testing.T) {
	startedAt := time.Date(2026, 9, 18, 10, 0, 0, 0, time.UTC)

	ctrl := gomock.NewController(t)
	tests := mock.NewMockTestExecutionRepository(ctrl)
	suites := mock.NewMockTestSuiteExecutionRepository(ctrl)

	tests.EXPECT().
		SetRunning(
			gomock.Any(),
			&repository.SetRunningParams{
				ID:        testExecutionID,
				StartedAt: startedAt,
			},
		).
		Return(nil)

	err := NewUpdater(tests, suites).SetTestRunning(
		t.Context(),
		testExecutionID,
		startedAt,
	)
	if err != nil {
		t.Fatalf("SetTestRunning: %v", err)
	}
}

func TestUpdaterSetTestRunningError(t *testing.T) {
	expected := errors.New("set running")

	ctrl := gomock.NewController(t)
	tests := mock.NewMockTestExecutionRepository(ctrl)

	tests.EXPECT().
		SetRunning(gomock.Any(), gomock.Any()).
		Return(expected)

	err := NewUpdater(
		tests,
		mock.NewMockTestSuiteExecutionRepository(ctrl),
	).SetTestRunning(
		t.Context(),
		testExecutionID,
		time.Now(),
	)

	if !errors.Is(err, expected) {
		t.Fatalf("got %v, want %v", err, expected)
	}
}

func TestUpdaterSetTestSuiteRunning(t *testing.T) {
	startedAt := time.Date(2026, 9, 18, 10, 0, 0, 0, time.UTC)

	ctrl := gomock.NewController(t)
	suites := mock.NewMockTestSuiteExecutionRepository(ctrl)

	suites.EXPECT().
		SetRunning(
			gomock.Any(),
			&repository.SetRunningParams{
				ID:        suiteExecutionID,
				StartedAt: startedAt,
			},
		).
		Return(nil)

	err := NewUpdater(
		mock.NewMockTestExecutionRepository(ctrl),
		suites,
	).SetTestSuiteRunning(
		t.Context(),
		suiteExecutionID,
		startedAt,
	)

	if err != nil {
		t.Fatalf("SetTestSuiteRunning: %v", err)
	}
}

func TestUpdaterSetTestSuiteRunningError(t *testing.T) {
	expected := errors.New("set suite running")

	ctrl := gomock.NewController(t)
	suites := mock.NewMockTestSuiteExecutionRepository(ctrl)

	suites.EXPECT().
		SetRunning(gomock.Any(), gomock.Any()).
		Return(expected)

	err := NewUpdater(
		mock.NewMockTestExecutionRepository(ctrl),
		suites,
	).SetTestSuiteRunning(
		t.Context(),
		suiteExecutionID,
		time.Now(),
	)

	if !errors.Is(err, expected) {
		t.Fatalf("got %v, want %v", err, expected)
	}
}

func TestUpdaterSetTestCompletedStandalone(t *testing.T) {
	finishedAt := time.Date(
		2026, 9, 18, 10, 1, 0, 0, time.UTC,
	)

	ctrl := gomock.NewController(t)
	tests := mock.NewMockTestExecutionRepository(ctrl)

	tests.EXPECT().
		SetCompleted(gomock.Any(), gomock.Any()).
		DoAndReturn(
			func(
				_ any,
				params *repository.SetCompletedParams,
			) error {
				if params.ID != testExecutionID {
					t.Errorf("unexpected execution ID: %s", params.ID)
				}

				if params.Status != string(model.ExecutionStatusError) {
					t.Errorf("unexpected status: %s", params.Status)
				}

				if !params.FinishedAt.Equal(finishedAt) {
					t.Errorf(
						"FinishedAt = %v, want %v",
						params.FinishedAt,
						finishedAt,
					)
				}

				var summary model.TestExecutionSummary
				if err := json.Unmarshal(
					params.Summary,
					&summary,
				); err != nil {
					t.Fatalf("unmarshal summary: %v", err)
				}

				if summary.Error != "test error" {
					t.Errorf(
						"summary error = %q, want %q",
						summary.Error,
						"test error",
					)
				}

				return nil
			},
		)

	completed, err := NewUpdater(
		tests,
		mock.NewMockTestSuiteExecutionRepository(ctrl),
	).SetTestCompleted(
		t.Context(),
		execute.TestExecutionResult{
			ExecutionID: testExecutionID,
			Status:      model.ExecutionStatusError,
			FinishedAt:  finishedAt,
			Summary: &model.TestExecutionSummary{
				Error: "test error",
			},
		},
	)

	if err != nil {
		t.Fatalf("SetTestCompleted: %v", err)
	}
	if !completed {
		t.Fatal("expected standalone test to be completed")
	}
}

func TestUpdaterSetTestCompletedNilSummary(t *testing.T) {
	ctrl := gomock.NewController(t)
	tests := mock.NewMockTestExecutionRepository(ctrl)

	tests.EXPECT().
		SetCompleted(gomock.Any(), gomock.Any()).
		DoAndReturn(
			func(
				_ any,
				params *repository.SetCompletedParams,
			) error {
				if params.Summary != nil {
					t.Errorf(
						"expected nil summary, got %s",
						params.Summary,
					)
				}
				return nil
			},
		)

	completed, err := NewUpdater(
		tests,
		mock.NewMockTestSuiteExecutionRepository(ctrl),
	).SetTestCompleted(
		t.Context(),
		execute.TestExecutionResult{
			ExecutionID: testExecutionID,
			Status:      model.ExecutionStatusPassed,
			FinishedAt:  time.Now(),
		},
	)

	if err != nil || !completed {
		t.Fatalf("completed=%v, err=%v", completed, err)
	}
}

func TestUpdaterSetTestCompletedError(t *testing.T) {
	expected := errors.New("set completed")

	ctrl := gomock.NewController(t)
	tests := mock.NewMockTestExecutionRepository(ctrl)

	tests.EXPECT().
		SetCompleted(gomock.Any(), gomock.Any()).
		Return(expected)

	completed, err := NewUpdater(
		tests,
		mock.NewMockTestSuiteExecutionRepository(ctrl),
	).SetTestCompleted(
		t.Context(),
		execute.TestExecutionResult{
			ExecutionID: testExecutionID,
			Status:      model.ExecutionStatusPassed,
		},
	)

	if completed {
		t.Fatal("expected completed to be false")
	}
	if !errors.Is(err, expected) {
		t.Fatalf("got %v, want %v", err, expected)
	}
}

func TestUpdaterSetTestCompletedSuiteStates(t *testing.T) {
	base := time.Date(
		2026, 9, 18, 10, 0, 0, 0, time.UTC,
	)

	stateErr := errors.New("get states")
	completionErr := errors.New("set suite completed")
	lookupErr := errors.New("get status")

	testCases := []struct {
		name string

		states           []repository.TestSuiteExecutionState
		alreadyCompleted bool
		stateErr         error

		completionErr  error
		currentStatus  string
		statusErr      error
		shouldFinalize bool
		shouldLookup   bool

		wantStatus    model.ExecutionStatus
		wantFinished  time.Time
		wantSummary   model.TestSuiteExecutionSummary
		wantCompleted bool
		wantErr       error
	}{
		{
			name: "running",
			states: []repository.TestSuiteExecutionState{
				{
					Status:             "passed",
					Count:              1,
					LastTestFinishedAt: base,
				},
				{
					Status: "running",
					Count:  1,
				},
			},
		},
		{
			name: "scheduled",
			states: []repository.TestSuiteExecutionState{
				{
					Status: "scheduled",
					Count:  1,
				},
				{
					Status:             "passed",
					Count:              1,
					LastTestFinishedAt: base,
				},
			},
		},
		{
			name: "empty",
		},
		{
			name:             "already completed",
			alreadyCompleted: true,
		},
		{
			name:             "already completed skips states",
			alreadyCompleted: true,
			states: []repository.TestSuiteExecutionState{
				{
					Status:             "passed",
					Count:              1,
					LastTestFinishedAt: base,
				},
			},
		},
		{
			name: "passed",
			states: []repository.TestSuiteExecutionState{
				{
					Status:             "passed",
					Count:              2,
					LastTestFinishedAt: base.Add(2 * time.Minute),
				},
			},
			wantStatus:     model.ExecutionStatusPassed,
			wantFinished:   base.Add(2 * time.Minute),
			wantSummary:    model.TestSuiteExecutionSummary{Total: 2, Passed: 2},
			wantCompleted:  true,
			shouldFinalize: true,
		},
		{
			name: "failed",
			states: []repository.TestSuiteExecutionState{
				{
					Status:             "passed",
					Count:              1,
					LastTestFinishedAt: base,
				},
				{
					Status:             "failed",
					Count:              2,
					LastTestFinishedAt: base.Add(time.Minute),
				},
			},
			wantStatus:     model.ExecutionStatusFailed,
			wantFinished:   base.Add(time.Minute),
			wantSummary:    model.TestSuiteExecutionSummary{Total: 3, Passed: 1, Failed: 2},
			wantCompleted:  true,
			shouldFinalize: true,
		},
		{
			name: "error precedence",
			states: []repository.TestSuiteExecutionState{
				{
					Status:             "error",
					Count:              1,
					LastTestFinishedAt: base,
				},
				{
					Status:             "failed",
					Count:              1,
					LastTestFinishedAt: base.Add(3 * time.Minute),
				},
				{
					Status:             "passed",
					Count:              1,
					LastTestFinishedAt: base.Add(time.Minute),
				},
			},
			wantStatus:     model.ExecutionStatusError,
			wantFinished:   base.Add(3 * time.Minute),
			wantSummary:    model.TestSuiteExecutionSummary{Total: 3, Passed: 1, Failed: 1, Errors: 1},
			wantCompleted:  true,
			shouldFinalize: true,
		},
		{
			name: "invalid status",
			states: []repository.TestSuiteExecutionState{
				{Status: "invalid", Count: 1},
			},
			wantErr: ErrInvalidExecutionStatus,
		},
		{
			name: "missing finish time",
			states: []repository.TestSuiteExecutionState{
				{Status: "passed", Count: 1},
			},
			wantErr: ErrInvalidExecutionStatus,
		},
		{
			name:     "states query error",
			stateErr: stateErr,
			wantErr:  stateErr,
		},
		{
			name: "suite completion error",
			states: []repository.TestSuiteExecutionState{
				{
					Status:             "passed",
					Count:              1,
					LastTestFinishedAt: base,
				},
			},
			completionErr:  completionErr,
			wantErr:        completionErr,
			shouldFinalize: true,
			wantStatus:     model.ExecutionStatusPassed,
			wantFinished:   base,
			wantSummary:    model.TestSuiteExecutionSummary{Total: 1, Passed: 1},
		},
		{
			name: "concurrent suite completion",
			states: []repository.TestSuiteExecutionState{
				{
					Status:             "passed",
					Count:              1,
					LastTestFinishedAt: base,
				},
			},
			completionErr:  repository.ErrInvalidExecutionStateTransition,
			currentStatus:  "passed",
			shouldFinalize: true,
			shouldLookup:   true,
			wantStatus:     model.ExecutionStatusPassed,
			wantFinished:   base,
			wantSummary:    model.TestSuiteExecutionSummary{Total: 1, Passed: 1},
		},
		{
			name: "concurrent suite completion failed",
			states: []repository.TestSuiteExecutionState{
				{
					Status:             "passed",
					Count:              1,
					LastTestFinishedAt: base,
				},
			},
			completionErr:  repository.ErrInvalidExecutionStateTransition,
			currentStatus:  "failed",
			shouldFinalize: true,
			shouldLookup:   true,
			wantStatus:     model.ExecutionStatusPassed,
			wantFinished:   base,
			wantSummary:    model.TestSuiteExecutionSummary{Total: 1, Passed: 1},
		},
		{
			name: "rejected while running",
			states: []repository.TestSuiteExecutionState{
				{
					Status:             "passed",
					Count:              1,
					LastTestFinishedAt: base,
				},
			},
			completionErr:  repository.ErrInvalidExecutionStateTransition,
			currentStatus:  "running",
			wantErr:        repository.ErrInvalidExecutionStateTransition,
			shouldFinalize: true,
			shouldLookup:   true,
			wantStatus:     model.ExecutionStatusPassed,
			wantFinished:   base,
			wantSummary:    model.TestSuiteExecutionSummary{Total: 1, Passed: 1},
		},
		{
			name: "status lookup error",
			states: []repository.TestSuiteExecutionState{
				{
					Status:             "passed",
					Count:              1,
					LastTestFinishedAt: base,
				},
			},
			completionErr:  repository.ErrInvalidExecutionStateTransition,
			statusErr:      lookupErr,
			wantErr:        lookupErr,
			shouldFinalize: true,
			shouldLookup:   true,
			wantStatus:     model.ExecutionStatusPassed,
			wantFinished:   base,
			wantSummary:    model.TestSuiteExecutionSummary{Total: 1, Passed: 1},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			tests := mock.NewMockTestExecutionRepository(ctrl)
			suites := mock.NewMockTestSuiteExecutionRepository(ctrl)

			tests.EXPECT().
				SetCompleted(gomock.Any(), gomock.Any()).
				Return(nil)

			tests.EXPECT().
				GetTestSuiteExecutionStates(
					gomock.Any(),
					suiteExecutionID,
				).
				Return(tc.states, tc.alreadyCompleted, tc.stateErr)

			if tc.shouldFinalize {
				suites.EXPECT().
					SetCompleted(gomock.Any(), gomock.Any()).
					DoAndReturn(
						func(
							_ any,
							params *repository.SetCompletedParams,
						) error {
							assertSuiteCompleted(
								t,
								params,
								tc.wantStatus,
								tc.wantFinished,
								tc.wantSummary,
							)
							return tc.completionErr
						},
					)
			}

			if tc.shouldLookup {
				suites.EXPECT().
					GetStatus(gomock.Any(), suiteExecutionID).
					Return(tc.currentStatus, tc.statusErr)
			}

			completed, err := NewUpdater(tests, suites).
				SetTestCompleted(
					t.Context(),
					execute.TestExecutionResult{
						ExecutionID:          testExecutionID,
						TestSuiteExecutionID: suiteExecutionID,
						Status:               model.ExecutionStatusPassed,
						FinishedAt:           base,
					},
				)

			if completed != tc.wantCompleted {
				t.Errorf(
					"completed=%v, want %v",
					completed,
					tc.wantCompleted,
				)
			}

			if tc.wantErr == nil && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf(
					"error=%v, want %v",
					err,
					tc.wantErr,
				)
			}
		})
	}
}

func TestUpdaterTryCompleteTestSuite(t *testing.T) {
	ctrl := gomock.NewController(t)
	tests := mock.NewMockTestExecutionRepository(ctrl)
	suites := mock.NewMockTestSuiteExecutionRepository(ctrl)

	tests.EXPECT().
		GetTestSuiteExecutionStates(
			gomock.Any(),
			suiteExecutionID,
		).
		Return(nil, true, nil)

	completed, err := NewUpdater(tests, suites).
		TryCompleteTestSuite(t.Context(), suiteExecutionID)

	if err != nil || completed {
		t.Fatalf("completed=%v, err=%v", completed, err)
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

	ctrl := gomock.NewController(t)
	suites := mock.NewMockTestSuiteExecutionRepository(ctrl)

	suites.EXPECT().
		SetCompleted(gomock.Any(), gomock.Any()).
		DoAndReturn(
			func(
				_ any,
				params *repository.SetCompletedParams,
			) error {
				assertSuiteCompleted(
					t,
					params,
					model.ExecutionStatusPassed,
					finishedAt,
					*summary,
				)
				return nil
			},
		)

	completed, err := NewUpdater(
		mock.NewMockTestExecutionRepository(ctrl),
		suites,
	).SetTestSuiteCompleted(
		t.Context(),
		execute.TestSuiteExecutionResult{
			ExecutionID: suiteExecutionID,
			Status:      model.ExecutionStatusPassed,
			FinishedAt:  finishedAt,
			Summary:     summary,
		},
	)

	if err != nil || !completed {
		t.Fatalf("completed=%v, err=%v", completed, err)
	}
}

func TestUpdaterSetTestSuiteCompletedNilSummary(t *testing.T) {
	ctrl := gomock.NewController(t)
	suites := mock.NewMockTestSuiteExecutionRepository(ctrl)

	suites.EXPECT().
		SetCompleted(gomock.Any(), gomock.Any()).
		DoAndReturn(
			func(
				_ any,
				params *repository.SetCompletedParams,
			) error {
				if params.Summary != nil {
					t.Errorf(
						"expected nil summary, got %s",
						params.Summary,
					)
				}
				return nil
			},
		)

	completed, err := NewUpdater(
		mock.NewMockTestExecutionRepository(ctrl),
		suites,
	).SetTestSuiteCompleted(
		t.Context(),
		execute.TestSuiteExecutionResult{
			ExecutionID: suiteExecutionID,
			Status:      model.ExecutionStatusError,
			FinishedAt:  time.Now(),
		},
	)

	if err != nil || !completed {
		t.Fatalf("completed=%v, err=%v", completed, err)
	}
}

func TestUpdaterSetTestSuiteCompletedError(t *testing.T) {
	expected := errors.New("set suite completed")

	ctrl := gomock.NewController(t)
	suites := mock.NewMockTestSuiteExecutionRepository(ctrl)

	suites.EXPECT().
		SetCompleted(gomock.Any(), gomock.Any()).
		Return(expected)

	completed, err := NewUpdater(
		mock.NewMockTestExecutionRepository(ctrl),
		suites,
	).SetTestSuiteCompleted(
		t.Context(),
		execute.TestSuiteExecutionResult{
			ExecutionID: suiteExecutionID,
			Status:      model.ExecutionStatusError,
		},
	)

	if completed || !errors.Is(err, expected) {
		t.Fatalf("completed=%v, err=%v", completed, err)
	}
}

func assertSuiteCompleted(
	t *testing.T,
	params *repository.SetCompletedParams,
	status model.ExecutionStatus,
	finishedAt time.Time,
	expectedSummary model.TestSuiteExecutionSummary,
) {
	t.Helper()

	if params == nil {
		t.Fatal("expected suite SetCompleted parameters")
	}

	if params.ID != suiteExecutionID {
		t.Errorf(
			"ID=%q, want %q",
			params.ID,
			suiteExecutionID,
		)
	}

	if params.Status != string(status) {
		t.Errorf(
			"status=%q, want %q",
			params.Status,
			status,
		)
	}

	if !params.FinishedAt.Equal(finishedAt) {
		t.Errorf(
			"FinishedAt=%v, want %v",
			params.FinishedAt,
			finishedAt,
		)
	}

	var actual model.TestSuiteExecutionSummary
	if err := json.Unmarshal(
		params.Summary,
		&actual,
	); err != nil {
		t.Fatalf("unmarshal summary: %v", err)
	}

	if actual != expectedSummary {
		t.Errorf(
			"summary=%+v, want %+v",
			actual,
			expectedSummary,
		)
	}
}
