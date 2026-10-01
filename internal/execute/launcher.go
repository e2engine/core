package execute

import (
	"context"
	nativeerrors "errors"
	"time"

	"github.com/e2engine/core/execute"
	"github.com/e2engine/core/internal/util"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/errors"
	idpkg "github.com/e2engine/core/pkg/id"
	"github.com/e2engine/core/pkg/keys"
	"github.com/e2engine/core/pkg/log"
	"github.com/e2engine/core/repository"
)

type scheduler interface {
	PublishTestJob(ctx context.Context, job execute.TestJob) error
}

type stateUpdater interface {
	SetTestCompleted(ctx context.Context, result execute.TestExecutionResult) (bool, error)
	SetTestSuiteCompleted(ctx context.Context, result execute.TestSuiteExecutionResult) (bool, error)
}

type testExecutionRepository interface {
	Create(ctx context.Context, params *repository.TestExecution) (*repository.TestExecution, error)
}

type testSuiteExecutionRepository interface {
	Create(ctx context.Context, params *repository.TestSuiteExecution) (*repository.TestSuiteExecution, error)
}

type LauncherConfig struct {
	JobPublishTimeout time.Duration `json:"job_publish_timeout" yaml:"job_publish_timeout" env:"JOB_PUBLISH_TIMEOUT" default:"30s"` // TODO: add validation.
}

// Launcher starts execution of resolved tests and test suites against resolved environments.
type Launcher struct {
	cfg                *LauncherConfig
	logger             log.Logger
	scheduler          scheduler
	stateUpdater       stateUpdater
	testExecution      testExecutionRepository
	testSuiteExecution testSuiteExecutionRepository
}

func NewLauncher(
	cfg *LauncherConfig,
	logger log.Logger,
	scheduler scheduler,
	stateUpdater stateUpdater,
	testExecution testExecutionRepository,
	testSuiteExecution testSuiteExecutionRepository,
) *Launcher {
	return &Launcher{
		cfg:                cfg,
		logger:             logger.With(log.String(keys.Component, "execution_launcher")),
		scheduler:          scheduler,
		stateUpdater:       stateUpdater,
		testExecution:      testExecution,
		testSuiteExecution: testSuiteExecution,
	}
}

// StartTest schedules execution of a resolved test against a resolved
// environment.
//
// On success, the returned execution ID identifies a TestExecution that has
// already been persisted and can be retrieved through GetTestExecution.
func (l *Launcher) StartTest(
	ctx context.Context,
	req execute.TestRunRequest,
) (string, error) {
	if l.scheduler == nil {
		return "", errors.ErrNilScheduler
	}

	id := idpkg.GenerateRandomID()

	exe := &repository.TestExecution{
		ID:              id,
		Status:          string(model.ExecutionStatusScheduled),
		EnvironmentID:   req.Environment.ID,
		EnvironmentName: req.Environment.Name,
		TestID:          req.Test.ID,
		TestName:        req.Test.Name,
	}

	if _, err := l.testExecution.Create(ctx, exe); err != nil {
		return "", err
	}

	environment, err := util.CloneStruct(*req.Environment)
	if err != nil {
		return id, l.completeTestExecutionAfterError(
			ctx,
			id,
			"",
			err,
		)
	}

	test, err := util.CloneStruct(*req.Test)
	if err != nil {
		return id, l.completeTestExecutionAfterError(
			ctx,
			id,
			"",
			err,
		)
	}

	job := execute.TestJob{
		ExecutionID: id,
		Environment: environment,
		Test:        test,
	}
	publishResult := l.publishTestJob(ctx, job)
	if err := publishResult.Err(); err != nil {
		return id, err
	}

	return id, nil
}

func (l *Launcher) StartTestSuite(
	ctx context.Context,
	req execute.TestSuiteRunRequest,
) (string, error) {
	if l.scheduler == nil {
		return "", errors.ErrNilScheduler
	}

	if len(req.Tests) == 0 {
		return "", errors.ErrNoTestsResolved
	}

	id := idpkg.GenerateRandomID()

	exe := &repository.TestSuiteExecution{
		ID:              id,
		Status:          string(model.ExecutionStatusScheduled),
		EnvironmentID:   req.Environment.ID,
		EnvironmentName: req.Environment.Name,
		TestSuiteID:     req.TestSuite.ID,
		TestSuiteName:   req.TestSuite.Name,
	}

	if _, err := l.testSuiteExecution.Create(ctx, exe); err != nil {
		return "", err
	}

	jobs, err := l.createTestJobs(
		ctx,
		id,
		req.Environment,
		req.Tests,
	)
	if err != nil {
		// we will have a testsuiteexecution record with status error.
		createJobsErr := nativeerrors.Join(
			errors.ErrFailedToCreateTestJobs,
			err,
		)

		return id, nativeerrors.Join(
			createJobsErr,
			l.completeTestSuiteExecutionWithError(
				ctx,
				id,
				createJobsErr,
			),
		)
	}

	var publishErr error

	for i := range jobs {
		result := l.publishTestJob(
			ctx,
			jobs[i],
		)

		if result.scheduleErr == nil {
			continue
		}

		publishErr = nativeerrors.Join(
			publishErr,
			result.Err(),
		)

		if result.stateUpdateErr != nil {
			// Persistent state can no longer be updated reliably.
			// Do not publish any more work.
			abortErr := result.Err()

			if i+1 < len(jobs) {
				publishErr = nativeerrors.Join(
					publishErr,
					l.completeTestExecutionsWithError(
						ctx,
						id,
						getTestJobExecutionIDs(
							jobs[i+1:],
						),
						abortErr,
					),
				)
			}
			publishErr = nativeerrors.Join(
				publishErr,
				l.completeTestSuiteExecutionWithError(
					ctx,
					id,
					publishErr,
				),
			)
			break
		}
	}

	return id, publishErr
}

func (l *Launcher) createTestJobs(
	ctx context.Context,
	executionID string,
	environment *model.Environment,
	tests []model.Test,
) ([]execute.TestJob, error) {
	testJobs := make([]execute.TestJob, 0, len(tests))
	createdExecutions := make([]string, 0, len(tests))

	for i := range tests {
		test := &tests[i]

		testExecutionID := idpkg.GenerateRandomID()

		testExecution := &repository.TestExecution{
			ID:                   testExecutionID,
			TestSuiteExecutionID: executionID,
			Status:               string(model.ExecutionStatusScheduled),
			EnvironmentID:        environment.ID,
			EnvironmentName:      environment.Name,
			TestID:               test.ID,
			TestName:             test.Name,
		}

		if _, err := l.testExecution.Create(ctx, testExecution); err != nil {
			createErr := nativeerrors.Join(
				errors.ErrFailedToCreateTestExecution,
				err,
			)

			return nil, nativeerrors.Join(
				createErr,
				l.completeTestExecutionsWithError(
					ctx,
					executionID,
					createdExecutions,
					createErr,
				),
			)
		}

		createdExecutions = append(createdExecutions, testExecutionID)

		environmentCopy, err := util.CloneStruct(*environment)
		if err != nil {
			return nil, nativeerrors.Join(
				err,
				l.completeTestExecutionsWithError(
					ctx,
					executionID,
					createdExecutions,
					err,
				),
			)
		}

		testCopy, err := util.CloneStruct(*test)
		if err != nil {
			return nil, nativeerrors.Join(
				err,
				l.completeTestExecutionsWithError(
					ctx,
					executionID,
					createdExecutions,
					err,
				),
			)
		}

		testJobs = append(
			testJobs,
			execute.TestJob{
				ExecutionID:          testExecutionID,
				TestSuiteExecutionID: executionID,
				Environment:          environmentCopy,
				Test:                 testCopy,
			},
		)
	}

	return testJobs, nil
}

func (l *Launcher) completeTestExecutionAfterError(
	ctx context.Context,
	executionID string,
	testSuiteExecutionID string,
	err error,
) error {
	return nativeerrors.Join(
		err,
		l.completeTestExecutionWithError(
			ctx,
			executionID,
			testSuiteExecutionID,
			err,
		),
	)
}

func (l *Launcher) completeTestExecutionsWithError(
	ctx context.Context,
	testSuiteExecutionID string,
	executionIDs []string,
	err error,
) error {
	var completeErr error

	for _, executionID := range executionIDs {
		completeErr = nativeerrors.Join(
			completeErr,
			l.completeTestExecutionWithError(
				ctx,
				executionID,
				testSuiteExecutionID,
				err,
			),
		)
	}

	return completeErr
}

type publishTestJobResult struct {
	scheduleErr    error
	stateUpdateErr error
}

func (r publishTestJobResult) Err() error {
	return nativeerrors.Join(
		r.scheduleErr,
		r.stateUpdateErr,
	)
}

func (l *Launcher) publishTestJob(
	ctx context.Context,
	job execute.TestJob,
) publishTestJobResult {
	publishCtx, cancel := context.WithTimeout(
		ctx,
		l.cfg.JobPublishTimeout,
	)
	defer cancel()

	if err := l.scheduler.PublishTestJob(publishCtx, job); err != nil {
		l.logger.Error(
			"Failed to schedule test execution",
			log.String(keys.ExecutionID, job.ExecutionID),
			log.Err(keys.Cause, err),
		)

		scheduleErr := nativeerrors.Join(
			errors.ErrFailedToScheduleJob,
			err,
		)

		return publishTestJobResult{
			scheduleErr: scheduleErr,
			stateUpdateErr: l.completeTestExecutionWithError(
				ctx,
				job.ExecutionID,
				job.TestSuiteExecutionID,
				scheduleErr,
			),
		}
	}

	l.logger.Debug(
		"Scheduled test execution",
		log.String(keys.ExecutionID, job.ExecutionID),
		log.String(keys.EnvironmentID, job.Environment.ID),
		log.String(keys.TestID, job.Test.ID),
	)

	return publishTestJobResult{}
}

func getTestJobExecutionIDs(
	jobs []execute.TestJob,
) []string {
	ids := make([]string, 0, len(jobs))

	for i := range jobs {
		ids = append(
			ids,
			jobs[i].ExecutionID,
		)
	}

	return ids
}

func (l *Launcher) completeTestExecutionWithError(
	ctx context.Context,
	executionID string,
	testSuiteExecutionID string,
	err error,
) error {
	_, setErr := l.stateUpdater.SetTestCompleted(
		ctx,
		execute.TestExecutionResult{
			ExecutionID:          executionID,
			TestSuiteExecutionID: testSuiteExecutionID,
			Status:               model.ExecutionStatusError,
			FinishedAt:           time.Now().UTC(),
			Summary: &model.TestExecutionSummary{
				Error: err.Error(),
			},
		},
	)

	return setErr
}

func (l *Launcher) completeTestSuiteExecutionWithError(
	ctx context.Context,
	executionID string,
	err error,
) error {
	_, setErr := l.stateUpdater.SetTestSuiteCompleted(
		ctx,
		execute.TestSuiteExecutionResult{
			ExecutionID: executionID,
			Status:      model.ExecutionStatusError,
			FinishedAt:  time.Now().UTC(),
			Summary: &model.TestSuiteExecutionSummary{
				Error: err.Error(),
			},
		},
	)

	return setErr
}
