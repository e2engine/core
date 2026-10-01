package resource

import (
	"github.com/ygrebnov/errorc"
	modellib "github.com/ygrebnov/model"

	"github.com/e2engine/core/internal/cursor"
	"github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/keys"
	"github.com/e2engine/core/repository"
)

const (
	InitialVersion = "1.0.0"
)

// Controller exposes CRUD methods to operate E2Engine resources.
type Controller struct {
	environment        repository.EnvironmentRepository
	test               repository.TestRepository
	testSuite          repository.TestSuiteRepository
	testExecution      repository.TestExecutionRepository
	testSuiteExecution repository.TestSuiteExecutionRepository

	cursorBinding *modellib.Binding[cursor.Payload]
}

func NewController(
	environmentRepository repository.EnvironmentRepository,
	testRepository repository.TestRepository,
	testSuiteRepository repository.TestSuiteRepository,
	testExecutionRepository repository.TestExecutionRepository,
	testSuiteExecutionRepository repository.TestSuiteExecutionRepository,
) (*Controller, error) {
	cursorBinding, err := cursor.GetBinding()
	if err != nil {
		return nil, errorc.With(
			errors.ErrCannotConstructCursorPayloadBinding,
			errorc.Error(keys.Cause, err),
		)
	}

	return &Controller{
		environment:        environmentRepository,
		test:               testRepository,
		testSuite:          testSuiteRepository,
		testExecution:      testExecutionRepository,
		testSuiteExecution: testSuiteExecutionRepository,
		cursorBinding:      cursorBinding,
	}, nil
}
