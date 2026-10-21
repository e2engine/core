package execute

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	coreexecute "github.com/e2engine/core/execute"
	"github.com/e2engine/core/mock"
	"github.com/e2engine/core/model"
	coreerrors "github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/log"
	"github.com/e2engine/core/repository"
)

type schedulerFunc func(
	ctx context.Context,
	job coreexecute.TestJob,
) error

func (f schedulerFunc) PublishTestJob(
	ctx context.Context,
	job coreexecute.TestJob,
) error {
	return f(ctx, job)
}

type stateUpdaterFunc struct {
	setTestCompleted      func(context.Context, coreexecute.TestExecutionResult) (bool, error)
	setTestSuiteCompleted func(context.Context, coreexecute.TestSuiteExecutionResult) (bool, error)
}

func (u *stateUpdaterFunc) SetTestCompleted(
	ctx context.Context,
	result coreexecute.TestExecutionResult,
) (bool, error) {
	if u.setTestCompleted == nil {
		return false, nil
	}

	return u.setTestCompleted(ctx, result)
}

func (u *stateUpdaterFunc) SetTestSuiteCompleted(
	ctx context.Context,
	result coreexecute.TestSuiteExecutionResult,
) (bool, error) {
	if u.setTestSuiteCompleted == nil {
		return false, nil
	}

	return u.setTestSuiteCompleted(ctx, result)
}

func newTestLauncher(
	t *testing.T,
	scheduler scheduler,
	stateUpdater stateUpdater,
	testExecution testExecutionRepository,
	testSuiteExecution testSuiteExecutionRepository,
) *Launcher {
	t.Helper()

	logger, err := log.NewSilentLogger()
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	return NewLauncher(
		&LauncherConfig{
			JobPublishTimeout: 100 * time.Millisecond,
		},
		logger,
		scheduler,
		stateUpdater,
		testExecution,
		testSuiteExecution,
	)
}

func testRunRequest() coreexecute.TestRunRequest {
	return coreexecute.TestRunRequest{
		Environment: &model.Environment{
			ID:   "environment-1",
			Name: "environment",
		},
		Test: &model.Test{
			ID:   "test-1",
			Name: "test",
		},
	}
}

func testSuiteRunRequest(testCount int) coreexecute.TestSuiteRunRequest {
	tests := make([]model.Test, testCount)

	for i := range tests {
		tests[i] = model.Test{
			ID:   "test-" + string(rune('1'+i)),
			Name: "test",
		}
	}

	return coreexecute.TestSuiteRunRequest{
		Environment: &model.Environment{
			ID:   "environment-1",
			Name: "environment",
		},
		TestSuite: &model.TestSuite{
			ID:   "suite-1",
			Name: "suite",
		},
		Tests: tests,
	}
}

func TestLauncherStartTest(t *testing.T) {
	ctrl := gomock.NewController(t)

	testExecution := mock.NewMockTestExecutionRepository(ctrl)

	var published coreexecute.TestJob

	scheduler := schedulerFunc(func(
		_ context.Context,
		job coreexecute.TestJob,
	) error {
		published = job
		return nil
	})

	req := testRunRequest()

	testExecution.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			execution *repository.TestExecution,
		) (*repository.TestExecution, error) {
			if execution.Status != string(model.ExecutionStatusScheduled) {
				t.Errorf(
					"status = %q, want %q",
					execution.Status,
					model.ExecutionStatusScheduled,
				)
			}

			if execution.EnvironmentID != req.Environment.ID {
				t.Errorf(
					"environment ID = %q, want %q",
					execution.EnvironmentID,
					req.Environment.ID,
				)
			}

			if execution.TestID != req.Test.ID {
				t.Errorf(
					"test ID = %q, want %q",
					execution.TestID,
					req.Test.ID,
				)
			}

			return execution, nil
		})

	launcher := newTestLauncher(
		t,
		scheduler,
		&stateUpdaterFunc{},
		testExecution,
		nil,
	)

	executionID, err := launcher.StartTest(
		context.Background(),
		req,
	)
	if err != nil {
		t.Fatalf("StartTest() error = %v", err)
	}

	if executionID == "" {
		t.Fatal("StartTest() returned empty execution ID")
	}

	if published.ExecutionID != executionID {
		t.Errorf(
			"published execution ID = %q, want %q",
			published.ExecutionID,
			executionID,
		)
	}

	if published.Environment.ID != req.Environment.ID {
		t.Errorf(
			"published environment ID = %q, want %q",
			published.Environment.ID,
			req.Environment.ID,
		)
	}

	if published.Test.ID != req.Test.ID {
		t.Errorf(
			"published test ID = %q, want %q",
			published.Test.ID,
			req.Test.ID,
		)
	}
}

func TestLauncherStartTestRepositoryError(t *testing.T) {
	ctrl := gomock.NewController(t)

	testExecution := mock.NewMockTestExecutionRepository(ctrl)

	repositoryErr := errors.New("repository unavailable")

	testExecution.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil, repositoryErr)

	published := false

	launcher := newTestLauncher(
		t,
		schedulerFunc(func(
			_ context.Context,
			_ coreexecute.TestJob,
		) error {
			published = true
			return nil
		}),
		&stateUpdaterFunc{},
		testExecution,
		nil,
	)

	executionID, err := launcher.StartTest(
		context.Background(),
		testRunRequest(),
	)

	if !errors.Is(err, repositoryErr) {
		t.Fatalf(
			"StartTest() error = %v, want %v",
			err,
			repositoryErr,
		)
	}

	if executionID != "" {
		t.Errorf(
			"execution ID = %q, want empty",
			executionID,
		)
	}

	if published {
		t.Error("job was published after repository failure")
	}
}

func TestLauncherStartTestPublishErrorCompletesExecution(t *testing.T) {
	ctrl := gomock.NewController(t)

	testExecution := mock.NewMockTestExecutionRepository(ctrl)

	testExecution.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			execution *repository.TestExecution,
		) (*repository.TestExecution, error) {
			return execution, nil
		})

	publishErr := errors.New("publish failed")

	var completed coreexecute.TestExecutionResult

	updater := &stateUpdaterFunc{
		setTestCompleted: func(
			_ context.Context,
			result coreexecute.TestExecutionResult,
		) (bool, error) {
			completed = result
			return true, nil
		},
	}

	launcher := newTestLauncher(
		t,
		schedulerFunc(func(
			_ context.Context,
			_ coreexecute.TestJob,
		) error {
			return publishErr
		}),
		updater,
		testExecution,
		nil,
	)

	executionID, err := launcher.StartTest(
		context.Background(),
		testRunRequest(),
	)

	if executionID == "" {
		t.Fatal("StartTest() returned empty execution ID")
	}

	if !errors.Is(err, coreerrors.ErrFailedToScheduleJob) {
		t.Errorf(
			"error = %v, want ErrFailedToScheduleJob",
			err,
		)
	}

	if !errors.Is(err, publishErr) {
		t.Errorf(
			"error = %v, want publish error",
			err,
		)
	}

	if completed.ExecutionID != executionID {
		t.Errorf(
			"completed execution ID = %q, want %q",
			completed.ExecutionID,
			executionID,
		)
	}

	if completed.Status != model.ExecutionStatusError {
		t.Errorf(
			"completed status = %q, want %q",
			completed.Status,
			model.ExecutionStatusError,
		)
	}

	if completed.Summary == nil {
		t.Fatal("completed summary is nil")
	}
}

func TestLauncherStartTestPublishTimeout(t *testing.T) {
	ctrl := gomock.NewController(t)

	testExecution := mock.NewMockTestExecutionRepository(ctrl)

	testExecution.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			execution *repository.TestExecution,
		) (*repository.TestExecution, error) {
			return execution, nil
		})

	var cleanupContextErr error

	updater := &stateUpdaterFunc{
		setTestCompleted: func(
			ctx context.Context,
			_ coreexecute.TestExecutionResult,
		) (bool, error) {
			cleanupContextErr = ctx.Err()
			return true, nil
		},
	}

	logger, err := log.NewSilentLogger()
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	launcher := NewLauncher(
		&LauncherConfig{
			JobPublishTimeout: 10 * time.Millisecond,
		},
		logger,
		schedulerFunc(func(
			ctx context.Context,
			_ coreexecute.TestJob,
		) error {
			<-ctx.Done()
			return ctx.Err()
		}),
		updater,
		testExecution,
		nil,
	)

	_, err = launcher.StartTest(
		context.Background(),
		testRunRequest(),
	)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf(
			"StartTest() error = %v, want DeadlineExceeded",
			err,
		)
	}

	if cleanupContextErr != nil {
		t.Errorf(
			"cleanup context error = %v, want nil",
			cleanupContextErr,
		)
	}
}

func TestLauncherStartTestSuiteNoTests(t *testing.T) {
	launcher := newTestLauncher(
		t,
		schedulerFunc(func(
			_ context.Context,
			_ coreexecute.TestJob,
		) error {
			t.Fatal("scheduler must not be called")
			return nil
		}),
		&stateUpdaterFunc{},
		nil,
		nil,
	)

	executionID, err := launcher.StartTestSuite(
		context.Background(),
		testSuiteRunRequest(0),
	)

	if !errors.Is(err, coreerrors.ErrNoTestsResolved) {
		t.Fatalf(
			"StartTestSuite() error = %v, want ErrNoTestsResolved",
			err,
		)
	}

	if executionID != "" {
		t.Errorf(
			"execution ID = %q, want empty",
			executionID,
		)
	}
}

func TestLauncherStartTestSuite(t *testing.T) {
	ctrl := gomock.NewController(t)

	testExecution := mock.NewMockTestExecutionRepository(ctrl)
	testSuiteExecution := mock.NewMockTestSuiteExecutionRepository(ctrl)

	req := testSuiteRunRequest(3)

	testSuiteExecution.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			execution *repository.TestSuiteExecution,
		) (*repository.TestSuiteExecution, error) {
			return execution, nil
		})

	testExecution.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			execution *repository.TestExecution,
		) (*repository.TestExecution, error) {
			return execution, nil
		}).
		Times(len(req.Tests))

	var published []coreexecute.TestJob

	launcher := newTestLauncher(
		t,
		schedulerFunc(func(
			_ context.Context,
			job coreexecute.TestJob,
		) error {
			published = append(published, job)
			return nil
		}),
		&stateUpdaterFunc{},
		testExecution,
		testSuiteExecution,
	)

	suiteExecutionID, err := launcher.StartTestSuite(
		context.Background(),
		req,
	)
	if err != nil {
		t.Fatalf("StartTestSuite() error = %v", err)
	}

	if suiteExecutionID == "" {
		t.Fatal("StartTestSuite() returned empty execution ID")
	}

	if len(published) != len(req.Tests) {
		t.Fatalf(
			"published jobs = %d, want %d",
			len(published),
			len(req.Tests),
		)
	}

	for i := range published {
		if published[i].TestSuiteExecutionID != suiteExecutionID {
			t.Errorf(
				"job %d suite execution ID = %q, want %q",
				i,
				published[i].TestSuiteExecutionID,
				suiteExecutionID,
			)
		}

		if published[i].Test.ID != req.Tests[i].ID {
			t.Errorf(
				"job %d test ID = %q, want %q",
				i,
				published[i].Test.ID,
				req.Tests[i].ID,
			)
		}
	}
}

func TestLauncherStartTestCanceledDuringPublishFinalizesExecution(t *testing.T) {
	ctrl := gomock.NewController(t)

	testExecution := mock.NewMockTestExecutionRepository(ctrl)

	testExecution.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			execution *repository.TestExecution,
		) (*repository.TestExecution, error) {
			return execution, nil
		})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var completed coreexecute.TestExecutionResult
	cleanupCalled := false

	updater := &stateUpdaterFunc{
		setTestCompleted: func(
			updateCtx context.Context,
			result coreexecute.TestExecutionResult,
		) (bool, error) {
			cleanupCalled = true
			completed = result

			if err := updateCtx.Err(); err != nil {
				t.Errorf(
					"cleanup context canceled: %v",
					err,
				)
			}

			if _, ok := updateCtx.Deadline(); !ok {
				t.Error("cleanup context has no deadline")
			}

			return true, nil
		},
	}

	launcher := newTestLauncher(
		t,
		schedulerFunc(func(
			publishCtx context.Context,
			_ coreexecute.TestJob,
		) error {
			// Simulate caller cancellation during publishing.
			cancel()

			<-publishCtx.Done()
			return publishCtx.Err()
		}),
		updater,
		testExecution,
		nil,
	)

	executionID, err := launcher.StartTest(
		ctx,
		testRunRequest(),
	)

	if executionID == "" {
		t.Fatal("StartTest() returned empty execution ID")
	}

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"StartTest() error = %v, want context.Canceled",
			err,
		)
	}

	if !cleanupCalled {
		t.Fatal("SetTestCompleted() was not called")
	}

	if completed.ExecutionID != executionID {
		t.Errorf(
			"completed execution ID = %q, want %q",
			completed.ExecutionID,
			executionID,
		)
	}

	if completed.Status != model.ExecutionStatusError {
		t.Errorf(
			"completed status = %q, want %q",
			completed.Status,
			model.ExecutionStatusError,
		)
	}
}

func TestLauncherStartTestSuiteCanceledDuringJobCreationFinalizesRecords(t *testing.T) {
	ctrl := gomock.NewController(t)

	testExecution := mock.NewMockTestExecutionRepository(ctrl)
	testSuiteExecution := mock.NewMockTestSuiteExecutionRepository(ctrl)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	testSuiteExecution.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			execution *repository.TestSuiteExecution,
		) (*repository.TestSuiteExecution, error) {
			return execution, nil
		})

	// First test execution is successfully persisted.
	testExecution.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			execution *repository.TestExecution,
		) (*repository.TestExecution, error) {
			return execution, nil
		})

	// Second test execution fails because the caller is canceled.
	testExecution.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			_ *repository.TestExecution,
		) (*repository.TestExecution, error) {
			cancel()
			return nil, context.Canceled
		})

	completedTests := 0
	completedSuites := 0

	updater := &stateUpdaterFunc{
		setTestCompleted: func(
			updateCtx context.Context,
			result coreexecute.TestExecutionResult,
		) (bool, error) {
			completedTests++

			if err := updateCtx.Err(); err != nil {
				t.Errorf(
					"test cleanup context canceled: %v",
					err,
				)
			}

			if _, ok := updateCtx.Deadline(); !ok {
				t.Error("test cleanup context has no deadline")
			}

			if result.Status != model.ExecutionStatusError {
				t.Errorf(
					"test status = %q, want %q",
					result.Status,
					model.ExecutionStatusError,
				)
			}

			return true, nil
		},

		setTestSuiteCompleted: func(
			updateCtx context.Context,
			result coreexecute.TestSuiteExecutionResult,
		) (bool, error) {
			completedSuites++

			if err := updateCtx.Err(); err != nil {
				t.Errorf(
					"suite cleanup context canceled: %v",
					err,
				)
			}

			if _, ok := updateCtx.Deadline(); !ok {
				t.Error("suite cleanup context has no deadline")
			}

			if result.Status != model.ExecutionStatusError {
				t.Errorf(
					"suite status = %q, want %q",
					result.Status,
					model.ExecutionStatusError,
				)
			}

			return true, nil
		},
	}

	launcher := newTestLauncher(
		t,
		schedulerFunc(func(
			context.Context,
			coreexecute.TestJob,
		) error {
			t.Error("scheduler must not be called")
			return nil
		}),
		updater,
		testExecution,
		testSuiteExecution,
	)

	executionID, err := launcher.StartTestSuite(
		ctx,
		testSuiteRunRequest(3),
	)

	if executionID == "" {
		t.Fatal("StartTestSuite() returned empty execution ID")
	}

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"StartTestSuite() error = %v, want context.Canceled",
			err,
		)
	}

	if completedTests != 1 {
		t.Errorf(
			"completed tests = %d, want 1",
			completedTests,
		)
	}

	if completedSuites != 1 {
		t.Errorf(
			"completed suites = %d, want 1",
			completedSuites,
		)
	}
}

func TestLauncherStartTestSuitePublishAndStateUpdateFailureFinalizesRemaining(t *testing.T) {
	ctrl := gomock.NewController(t)

	testExecution := mock.NewMockTestExecutionRepository(ctrl)
	testSuiteExecution := mock.NewMockTestSuiteExecutionRepository(ctrl)

	testSuiteExecution.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			execution *repository.TestSuiteExecution,
		) (*repository.TestSuiteExecution, error) {
			return execution, nil
		})

	testExecution.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			execution *repository.TestExecution,
		) (*repository.TestExecution, error) {
			return execution, nil
		}).
		Times(3)

	publishErr := errors.New("publishing failed")
	updateErr := errors.New("state update failed")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var published []string
	var completed []coreexecute.TestExecutionResult
	var completedSuite coreexecute.TestSuiteExecutionResult

	updater := &stateUpdaterFunc{
		setTestCompleted: func(
			updateCtx context.Context,
			result coreexecute.TestExecutionResult,
		) (bool, error) {
			if err := updateCtx.Err(); err != nil {
				t.Errorf(
					"test completion context canceled: %v",
					err,
				)
			}

			if _, ok := updateCtx.Deadline(); !ok {
				t.Error("test completion context has no deadline")
			}

			completed = append(completed, result)

			// Simulate failure to persist the first
			// test execution's final status.
			if len(completed) == 1 {
				return false, updateErr
			}

			return true, nil
		},

		setTestSuiteCompleted: func(
			updateCtx context.Context,
			result coreexecute.TestSuiteExecutionResult,
		) (bool, error) {
			if err := updateCtx.Err(); err != nil {
				t.Errorf(
					"suite completion context canceled: %v",
					err,
				)
			}

			if _, ok := updateCtx.Deadline(); !ok {
				t.Error("suite completion context has no deadline")
			}

			completedSuite = result

			return true, nil
		},
	}

	launcher := newTestLauncher(
		t,
		schedulerFunc(func(
			_ context.Context,
			job coreexecute.TestJob,
		) error {
			published = append(
				published,
				job.ExecutionID,
			)

			// Cancel the caller while publishing.
			cancel()

			return publishErr
		}),
		updater,
		testExecution,
		testSuiteExecution,
	)

	suiteID, err := launcher.StartTestSuite(
		ctx,
		testSuiteRunRequest(3),
	)

	if suiteID == "" {
		t.Fatal("StartTestSuite() returned empty suite execution ID")
	}

	// Both the publishing failure and the state-update
	// failure must be preserved in the returned error.
	if !errors.Is(err, coreerrors.ErrFailedToScheduleJob) {
		t.Errorf(
			"error = %v, want ErrFailedToScheduleJob",
			err,
		)
	}

	if !errors.Is(err, publishErr) {
		t.Errorf(
			"error = %v, want publishing error",
			err,
		)
	}

	if !errors.Is(err, updateErr) {
		t.Errorf(
			"error = %v, want state update error",
			err,
		)
	}

	// Publishing must stop after the first job because
	// its execution state could not be updated.
	if len(published) != 1 {
		t.Fatalf(
			"published jobs = %d, want 1",
			len(published),
		)
	}

	// The launcher must attempt finalization of all
	// three executions, including the failed job.
	if len(completed) != 3 {
		t.Fatalf(
			"test completion attempts = %d, want 3",
			len(completed),
		)
	}

	if completed[0].ExecutionID != published[0] {
		t.Errorf(
			"first completed execution ID = %q, want %q",
			completed[0].ExecutionID,
			published[0],
		)
	}

	seen := make(map[string]bool)

	for _, result := range completed {
		if seen[result.ExecutionID] {
			t.Errorf(
				"execution %q finalized twice",
				result.ExecutionID,
			)
		}

		seen[result.ExecutionID] = true

		if result.TestSuiteExecutionID != suiteID {
			t.Errorf(
				"test suite execution ID = %q, want %q",
				result.TestSuiteExecutionID,
				suiteID,
			)
		}

		if result.Status != model.ExecutionStatusError {
			t.Errorf(
				"execution %q status = %q, want %q",
				result.ExecutionID,
				result.Status,
				model.ExecutionStatusError,
			)
		}

		if result.Summary == nil {
			t.Errorf(
				"execution %q summary is nil",
				result.ExecutionID,
			)
		}
	}

	// Verify that the parent test suite is finalized.
	if completedSuite.ExecutionID != suiteID {
		t.Errorf(
			"completed suite ID = %q, want %q",
			completedSuite.ExecutionID,
			suiteID,
		)
	}

	if completedSuite.Status != model.ExecutionStatusError {
		t.Errorf(
			"suite status = %q, want %q",
			completedSuite.Status,
			model.ExecutionStatusError,
		)
	}

	if completedSuite.Summary == nil {
		t.Error("completed suite summary is nil")
	}
}
