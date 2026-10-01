package core

import (
	"context"

	"github.com/ygrebnov/errorc"

	"github.com/e2engine/core/execute"
	"github.com/e2engine/core/execute/persist"
	runtimeconfig "github.com/e2engine/core/execute/runtime/config"
	internalexecute "github.com/e2engine/core/internal/execute"
	"github.com/e2engine/core/internal/resource"
	"github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/keys"
	"github.com/e2engine/core/pkg/log"
	"github.com/e2engine/core/repository"
)

// Service exposes methods to operate E2Engine resources and run tests and testsuites.
type Service struct {
	runtimeCfg *runtimeconfig.Config
	logger     log.Logger
	resources  *resource.Controller
	launcher   launcher
}

type launcher interface {
	StartTest(ctx context.Context, req execute.TestRunRequest) (string, error)
	StartTestSuite(ctx context.Context, req execute.TestSuiteRunRequest) (string, error)
}

var _ launcher = (*internalexecute.Launcher)(nil)

type Repositories struct {
	Environment        repository.EnvironmentRepository
	Test               repository.TestRepository
	TestSuite          repository.TestSuiteRepository
	TestExecution      repository.TestExecutionRepository
	TestSuiteExecution repository.TestSuiteExecutionRepository
}

type settings struct {
	runtimeConfig *runtimeconfig.Config
	logger        log.Logger
	scheduler     Scheduler
}

type Option func(*settings)

func WithLogger(logger log.Logger) Option {
	return func(settings *settings) {
		if logger != nil {
			settings.logger = logger
		}
	}
}

type Scheduler interface {
	PublishTestJob(ctx context.Context, job execute.TestJob) error
}

// WithScheduler sets the scheduler to be used for publishing test jobs.
// Scheduler is not required for resource operations.
func WithScheduler(scheduler Scheduler) Option {
	return func(settings *settings) {
		if scheduler != nil {
			settings.scheduler = scheduler
		}
	}
}

func WithRuntimeConfig(config *runtimeconfig.Config) Option {
	return func(settings *settings) {
		if config != nil {
			settings.runtimeConfig = config
		}
	}
}

func getSettings(opts ...Option) (*settings, error) {
	logger, err := log.NewSilentLogger()
	if err != nil {
		return nil, errorc.With(
			errors.ErrCannotInitializeLogger, errorc.Error(keys.Cause, err),
		)
	}

	s := &settings{
		logger: logger,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}

	if s.runtimeConfig == nil {
		return nil, errors.ErrNilRuntimeConfig
	}

	return s, nil
}

// NewService constructs a new Service providing E2Engine core API.
func NewService(repositories Repositories, opts ...Option) (*Service, error) {
	s, err := getSettings(opts...)
	if err != nil {
		return nil, err
	}

	resources, err := resource.NewController(
		repositories.Environment,
		repositories.Test,
		repositories.TestSuite,
		repositories.TestExecution,
		repositories.TestSuiteExecution,
	)
	if err != nil {
		return nil, err
	}

	stateUpdater := persist.NewUpdater(
		repositories.TestExecution,
		repositories.TestSuiteExecution,
	)

	launcherConfig := &internalexecute.LauncherConfig{JobPublishTimeout: s.runtimeConfig.JobPublishTimeout}

	l := internalexecute.NewLauncher(
		launcherConfig,
		s.logger,
		s.scheduler,
		stateUpdater,
		repositories.TestExecution,
		repositories.TestSuiteExecution,
	)

	return &Service{
		runtimeCfg: s.runtimeConfig,
		logger:     s.logger.With(log.String(keys.Component, "core_service")),
		resources:  resources,
		launcher:   l,
	}, nil
}

func validateReference(ref, requestKind string) error {
	if len(ref) < 3 || len(ref) > 200 {
		return errorc.With(
			errors.ErrInvalidRequestParameters,
			errorc.String(keys.RequestKind, requestKind),
			errorc.String(keys.Validation, "ref length must be between 3 and 200 characters"),
		)
	}

	return nil
}
