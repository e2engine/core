package tests

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	core "github.com/e2engine/core/api"
	runtimeconfig "github.com/e2engine/core/execute/runtime/config"
	"github.com/e2engine/core/mock"
	"github.com/e2engine/core/model"
	pkgerrors "github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/repository"
)

func TestTestSuiteExecution_Get(t *testing.T) {
	t.Run("invalid reference", func(t *testing.T) {
		service := newTestSuiteExecutionService(t, nil, nil)

		got, err := service.GetTestSuiteExecution(
			context.Background(),
			"ab",
		)

		if got != nil {
			t.Fatalf(
				"GetTestSuiteExecution() = %#v, want nil",
				got,
			)
		}
		if !errors.Is(err, pkgerrors.ErrInvalidRequestParameters) {
			t.Fatalf(
				"GetTestSuiteExecution() error = %v, want %v",
				err,
				pkgerrors.ErrInvalidRequestParameters,
			)
		}
	})

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		suiteExecutionRepo :=
			mock.NewMockTestSuiteExecutionRepository(ctrl)
		testExecutionRepo :=
			mock.NewMockTestExecutionRepository(ctrl)

		suiteExecutionRepo.EXPECT().
			GetIDsByID(gomock.Any(), "exe").
			Return([]string{"suite-execution-001"}, nil)

		suiteExecutionRepo.EXPECT().
			Get(gomock.Any(), "suite-execution-001").
			Return(repositoryTestSuiteExecution(), nil)

		// GetTestSuiteExecution enriches the suite execution with its
		// child test executions.
		testExecutionRepo.EXPECT().
			GetByTestSuiteExecutionID(
				gomock.Any(),
				"suite-execution-001",
			).
			Return([]repository.TestExecution{
				*repositorySuiteChildTestExecution(),
			}, nil)

		service := newTestSuiteExecutionService(
			t,
			testExecutionRepo,
			suiteExecutionRepo,
		)

		got, err := service.GetTestSuiteExecution(
			context.Background(),
			"exe",
		)
		if err != nil {
			t.Fatalf(
				"GetTestSuiteExecution() error = %v",
				err,
			)
		}

		want := modelTestSuiteExecution(true)

		if !reflect.DeepEqual(got, want) {
			t.Errorf(
				"GetTestSuiteExecution() = %#v, want %#v",
				got,
				want,
			)
		}
	})
}

func TestTestSuiteExecution_GetStatus(t *testing.T) {
	t.Run("invalid reference", func(t *testing.T) {
		service := newTestSuiteExecutionService(t, nil, nil)

		got, err := service.GetTestSuiteExecutionStatus(
			context.Background(),
			"ab",
		)

		if got != "" {
			t.Fatalf(
				"GetTestSuiteExecutionStatus() = %s, want empty",
				got,
			)
		}
		if !errors.Is(err, pkgerrors.ErrInvalidRequestParameters) {
			t.Fatalf(
				"GetTestSuiteExecutionStatus() error = %v, want %v",
				err,
				pkgerrors.ErrInvalidRequestParameters,
			)
		}
	})

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		suiteExecutionRepo :=
			mock.NewMockTestSuiteExecutionRepository(ctrl)

		suiteExecutionRepo.EXPECT().
			GetIDsByID(gomock.Any(), "exe").
			Return([]string{"suite-execution-001"}, nil)

		suiteExecutionRepo.EXPECT().
			GetStatus(gomock.Any(), "suite-execution-001").
			Return(repositoryTestSuiteExecution().Status, nil)

		service := newTestSuiteExecutionService(
			t,
			nil,
			suiteExecutionRepo,
		)

		got, err := service.GetTestSuiteExecutionStatus(
			context.Background(),
			"exe",
		)
		if err != nil {
			t.Fatalf(
				"GetTestSuiteExecutionStatus() error = %v",
				err,
			)
		}

		want := modelTestSuiteExecution(false).Status

		if got != want {
			t.Errorf(
				"GetTestSuiteExecutionStatus() = %s, want %s",
				got,
				want,
			)
		}
	})
}

func TestTestSuiteExecution_Delete(t *testing.T) {
	t.Run("invalid reference", func(t *testing.T) {
		service := newTestSuiteExecutionService(t, nil, nil)

		got, err := service.DeleteTestSuiteExecution(
			context.Background(),
			"ab",
		)

		if got != nil {
			t.Fatalf(
				"DeleteTestSuiteExecution() = %#v, want nil",
				got,
			)
		}
		if !errors.Is(err, pkgerrors.ErrInvalidRequestParameters) {
			t.Fatalf(
				"DeleteTestSuiteExecution() error = %v, want %v",
				err,
				pkgerrors.ErrInvalidRequestParameters,
			)
		}
	})

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		suiteExecutionRepo :=
			mock.NewMockTestSuiteExecutionRepository(ctrl)

		suiteExecutionRepo.EXPECT().
			GetIDsByID(gomock.Any(), "exe").
			Return([]string{"suite-execution-001"}, nil)

		suiteExecutionRepo.EXPECT().
			Delete(gomock.Any(), "suite-execution-001").
			Return(repositoryTestSuiteExecution(), nil)

		service := newTestSuiteExecutionService(
			t,
			nil,
			suiteExecutionRepo,
		)

		got, err := service.DeleteTestSuiteExecution(
			context.Background(),
			"exe",
		)
		if err != nil {
			t.Fatalf(
				"DeleteTestSuiteExecution() error = %v",
				err,
			)
		}

		want := modelTestSuiteExecution(false)

		if !reflect.DeepEqual(got, want) {
			t.Errorf(
				"DeleteTestSuiteExecution() = %#v, want %#v",
				got,
				want,
			)
		}
	})
}

func TestTestSuiteExecution_GetPage(t *testing.T) {
	t.Run("invalid page size", func(t *testing.T) {
		service := newTestSuiteExecutionService(t, nil, nil)

		got, err := service.GetTestSuiteExecutionsPage(
			context.Background(),
			&core.GetTestSuiteExecutionsPageParams{
				PageSize: 101,
			},
		)

		if got != nil {
			t.Fatalf(
				"GetTestSuiteExecutionsPage() = %#v, want nil",
				got,
			)
		}
		if !errors.Is(err, pkgerrors.ErrInvalidRequestParameters) {
			t.Fatalf(
				"GetTestSuiteExecutionsPage() error = %v, want %v",
				err,
				pkgerrors.ErrInvalidRequestParameters,
			)
		}
	})

	t.Run("invalid order by", func(t *testing.T) {
		service := newTestSuiteExecutionService(t, nil, nil)

		got, err := service.GetTestSuiteExecutionsPage(
			context.Background(),
			&core.GetTestSuiteExecutionsPageParams{
				PageSize: 10,
				OrderBy:  model.OrderByName,
			},
		)

		if got != nil {
			t.Fatalf(
				"GetTestSuiteExecutionsPage() = %#v, want nil",
				got,
			)
		}
		if !errors.Is(err, pkgerrors.ErrInvalidRequestParameters) {
			t.Fatalf(
				"GetTestSuiteExecutionsPage() error = %v, want %v",
				err,
				pkgerrors.ErrInvalidRequestParameters,
			)
		}
	})

	t.Run("invalid order direction", func(t *testing.T) {
		service := newTestSuiteExecutionService(t, nil, nil)

		got, err := service.GetTestSuiteExecutionsPage(
			context.Background(),
			&core.GetTestSuiteExecutionsPageParams{
				PageSize:       10,
				OrderDirection: model.OrderDirection("sideways"),
			},
		)

		if got != nil {
			t.Fatalf(
				"GetTestSuiteExecutionsPage() = %#v, want nil",
				got,
			)
		}
		if !errors.Is(err, pkgerrors.ErrInvalidRequestParameters) {
			t.Fatalf(
				"GetTestSuiteExecutionsPage() error = %v, want %v",
				err,
				pkgerrors.ErrInvalidRequestParameters,
			)
		}
	})

	t.Run("defaults", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		suiteExecutionRepo :=
			mock.NewMockTestSuiteExecutionRepository(ctrl)

		suiteExecutionRepo.EXPECT().
			List(
				gomock.Any(),
				&repository.ListParams{
					Limit:          11,
					Position:       repository.PositionAfter,
					OrderBy:        repository.OrderByStartedAt,
					OrderDirection: repository.OrderDirectionDesc,
				},
			).
			Return([]repository.TestSuiteExecution{
				*repositoryTestSuiteExecution(),
			}, nil)

		service := newTestSuiteExecutionService(
			t,
			nil,
			suiteExecutionRepo,
		)

		got, err := service.GetTestSuiteExecutionsPage(
			context.Background(),
			nil,
		)
		if err != nil {
			t.Fatalf(
				"GetTestSuiteExecutionsPage() error = %v",
				err,
			)
		}

		if len(got.Items) != 1 {
			t.Fatalf(
				"len(Items) = %d, want 1",
				len(got.Items),
			)
		}

		// Page results are intentionally not enriched with child
		// TestExecutions.
		want := *modelTestSuiteExecution(false)

		if !reflect.DeepEqual(got.Items[0], want) {
			t.Errorf(
				"Items[0] = %#v, want %#v",
				got.Items[0],
				want,
			)
		}
	})

	t.Run("page size and ordering", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		suiteExecutionRepo :=
			mock.NewMockTestSuiteExecutionRepository(ctrl)

		suiteExecutionRepo.EXPECT().
			List(
				gomock.Any(),
				&repository.ListParams{
					Limit:          6,
					Position:       repository.PositionAfter,
					OrderBy:        repository.OrderByFinishedAt,
					OrderDirection: repository.OrderDirectionAsc,
				},
			).
			Return([]repository.TestSuiteExecution{
				*repositoryTestSuiteExecution(),
			}, nil)

		service := newTestSuiteExecutionService(
			t,
			nil,
			suiteExecutionRepo,
		)

		got, err := service.GetTestSuiteExecutionsPage(
			context.Background(),
			&core.GetTestSuiteExecutionsPageParams{
				PageSize:       5,
				OrderBy:        model.OrderByFinishedAt,
				OrderDirection: model.OrderDirectionAsc,
			},
		)
		if err != nil {
			t.Fatalf(
				"GetTestSuiteExecutionsPage() error = %v",
				err,
			)
		}

		if len(got.Items) != 1 {
			t.Fatalf(
				"len(Items) = %d, want 1",
				len(got.Items),
			)
		}
	})
}

func newTestSuiteExecutionService(
	t *testing.T,
	testExecutionRepository repository.TestExecutionRepository,
	testSuiteExecutionRepository repository.TestSuiteExecutionRepository,
) *core.Service {
	t.Helper()

	service, err := core.NewService(
		core.Repositories{
			TestExecution:      testExecutionRepository,
			TestSuiteExecution: testSuiteExecutionRepository,
		},
		core.WithRuntimeConfig(&runtimeconfig.Config{}),
	)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	return service
}

func repositoryTestSuiteExecution() *repository.TestSuiteExecution {
	startedAt := time.Date(
		2026,
		time.September,
		21,
		12,
		0,
		0,
		0,
		time.UTC,
	)
	finishedAt := startedAt.Add(5 * time.Second)

	summary, err := json.Marshal(modelTestSuiteExecutionSummary())
	if err != nil {
		panic(err)
	}

	return &repository.TestSuiteExecution{
		ID:              "suite-execution-001",
		StartedAt:       startedAt,
		FinishedAt:      finishedAt,
		Status:          string(model.ExecutionStatusPassed),
		EnvironmentID:   "environment-001",
		EnvironmentName: "local",
		TestSuiteID:     "testsuite-001",
		TestSuiteName:   "smoke-suite",
		TestsCount:      1,
		Summary:         summary,
	}
}

func repositorySuiteChildTestExecution() *repository.TestExecution {
	startedAt := time.Date(
		2026,
		time.September,
		21,
		12,
		0,
		0,
		0,
		time.UTC,
	)
	finishedAt := startedAt.Add(2 * time.Second)

	return &repository.TestExecution{
		ID:                   "test-execution-001",
		TestSuiteExecutionID: "suite-execution-001",
		StartedAt:            startedAt,
		FinishedAt:           finishedAt,
		Status:               string(model.ExecutionStatusPassed),
		EnvironmentID:        "environment-001",
		EnvironmentName:      "local",
		TestID:               "test-001",
		TestName:             "health-check",
	}
}

func modelTestSuiteExecution(
	withTests bool,
) *model.TestSuiteExecution {
	startedAt := time.Date(
		2026,
		time.September,
		21,
		12,
		0,
		0,
		0,
		time.UTC,
	)
	finishedAt := startedAt.Add(5 * time.Second)

	result := &model.TestSuiteExecution{
		ID:              "suite-execution-001",
		StartedAt:       startedAt,
		FinishedAt:      finishedAt,
		Status:          model.ExecutionStatusPassed,
		EnvironmentID:   "environment-001",
		EnvironmentName: "local",
		TestSuiteID:     "testsuite-001",
		TestSuiteName:   "smoke-suite",
		TestsCount:      1,
		Summary:         modelTestSuiteExecutionSummary(),
	}

	if withTests {
		result.Tests = []model.TestExecution{
			{
				ID:              "test-execution-001",
				StartedAt:       startedAt,
				FinishedAt:      startedAt.Add(2 * time.Second),
				Status:          model.ExecutionStatusPassed,
				EnvironmentID:   "environment-001",
				EnvironmentName: "local",
				TestID:          "test-001",
				TestName:        "health-check",
			},
		}
	}

	return result
}

func modelTestSuiteExecutionSummary() *model.TestSuiteExecutionSummary {
	return &model.TestSuiteExecutionSummary{
		Total:  1,
		Passed: 1,
		Failed: 0,
		Errors: 0,
	}
}
