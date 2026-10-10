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

func TestTestExecution_Get(t *testing.T) {
	t.Run("invalid reference", func(t *testing.T) {
		service := newTestExecutionService(t, nil)

		got, err := service.GetTestExecution(
			context.Background(),
			"ab",
		)

		if got != nil {
			t.Fatalf("GetTestExecution() = %#v, want nil", got)
		}
		if !errors.Is(err, pkgerrors.ErrInvalidRequestParameters) {
			t.Fatalf(
				"GetTestExecution() error = %v, want %v",
				err,
				pkgerrors.ErrInvalidRequestParameters,
			)
		}
	})

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mock.NewMockTestExecutionRepository(ctrl)

		repo.EXPECT().
			GetIDsByID(gomock.Any(), "exe").
			Return([]string{"execution-001"}, nil)

		repo.EXPECT().
			Get(gomock.Any(), "execution-001").
			Return(repositoryTestExecution(), nil)

		service := newTestExecutionService(t, repo)

		got, err := service.GetTestExecution(
			context.Background(),
			"exe",
		)
		if err != nil {
			t.Fatalf("GetTestExecution() error = %v", err)
		}

		want := modelTestExecution()

		if !reflect.DeepEqual(got, want) {
			t.Errorf(
				"GetTestExecution() = %#v, want %#v",
				got,
				want,
			)
		}
	})
}

func TestTestExecution_GetStatus(t *testing.T) {
	t.Run("invalid reference", func(t *testing.T) {
		service := newTestExecutionService(t, nil)

		got, err := service.GetTestExecutionStatus(
			context.Background(),
			"ab",
		)

		if got != "" {
			t.Fatalf("GetTestExecutionStatus() = %q, want empty", got)
		}
		if !errors.Is(err, pkgerrors.ErrInvalidRequestParameters) {
			t.Fatalf(
				"GetTestExecutionStatus() error = %v, want %v",
				err,
				pkgerrors.ErrInvalidRequestParameters,
			)
		}
	})

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mock.NewMockTestExecutionRepository(ctrl)

		repo.EXPECT().
			GetIDsByID(gomock.Any(), "exe").
			Return([]string{"execution-001"}, nil)

		repo.EXPECT().
			GetStatus(gomock.Any(), "execution-001").
			Return(repositoryTestExecution().Status, nil)

		service := newTestExecutionService(t, repo)

		got, err := service.GetTestExecutionStatus(
			context.Background(),
			"exe",
		)
		if err != nil {
			t.Fatalf("GetTestExecutionStatus() error = %v", err)
		}

		want := modelTestExecution().Status

		if got != want {
			t.Errorf(
				"GetTestExecutionStatus() = %s, want %s",
				got,
				want,
			)
		}
	})
}

func TestTestExecution_Delete(t *testing.T) {
	t.Run("invalid reference", func(t *testing.T) {
		service := newTestExecutionService(t, nil)

		got, err := service.DeleteTestExecution(
			context.Background(),
			"ab",
		)

		if got != nil {
			t.Fatalf("DeleteTestExecution() = %#v, want nil", got)
		}
		if !errors.Is(err, pkgerrors.ErrInvalidRequestParameters) {
			t.Fatalf(
				"DeleteTestExecution() error = %v, want %v",
				err,
				pkgerrors.ErrInvalidRequestParameters,
			)
		}
	})

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mock.NewMockTestExecutionRepository(ctrl)

		repo.EXPECT().
			GetIDsByID(gomock.Any(), "exe").
			Return([]string{"execution-001"}, nil)

		repo.EXPECT().
			Delete(gomock.Any(), "execution-001").
			Return(repositoryTestExecution(), nil)

		service := newTestExecutionService(t, repo)

		got, err := service.DeleteTestExecution(
			context.Background(),
			"exe",
		)
		if err != nil {
			t.Fatalf("DeleteTestExecution() error = %v", err)
		}

		want := modelTestExecution()

		if !reflect.DeepEqual(got, want) {
			t.Errorf(
				"DeleteTestExecution() = %#v, want %#v",
				got,
				want,
			)
		}
	})
}

func TestTestExecution_GetPage(t *testing.T) {
	t.Run("invalid page size", func(t *testing.T) {
		service := newTestExecutionService(t, nil)

		got, err := service.GetTestExecutionsPage(
			context.Background(),
			&core.GetTestExecutionsPageParams{
				PageSize: 101,
			},
		)

		if got != nil {
			t.Fatalf(
				"GetTestExecutionsPage() = %#v, want nil",
				got,
			)
		}
		if !errors.Is(err, pkgerrors.ErrInvalidRequestParameters) {
			t.Fatalf(
				"GetTestExecutionsPage() error = %v, want %v",
				err,
				pkgerrors.ErrInvalidRequestParameters,
			)
		}
	})

	t.Run("invalid order by", func(t *testing.T) {
		service := newTestExecutionService(t, nil)

		got, err := service.GetTestExecutionsPage(
			context.Background(),
			&core.GetTestExecutionsPageParams{
				PageSize: 10,
				OrderBy:  model.OrderByName,
			},
		)

		if got != nil {
			t.Fatalf(
				"GetTestExecutionsPage() = %#v, want nil",
				got,
			)
		}
		if !errors.Is(err, pkgerrors.ErrInvalidRequestParameters) {
			t.Fatalf(
				"GetTestExecutionsPage() error = %v, want %v",
				err,
				pkgerrors.ErrInvalidRequestParameters,
			)
		}
	})

	t.Run("invalid order direction", func(t *testing.T) {
		service := newTestExecutionService(t, nil)

		got, err := service.GetTestExecutionsPage(
			context.Background(),
			&core.GetTestExecutionsPageParams{
				PageSize:       10,
				OrderDirection: model.OrderDirection("sideways"),
			},
		)

		if got != nil {
			t.Fatalf(
				"GetTestExecutionsPage() = %#v, want nil",
				got,
			)
		}
		if !errors.Is(err, pkgerrors.ErrInvalidRequestParameters) {
			t.Fatalf(
				"GetTestExecutionsPage() error = %v, want %v",
				err,
				pkgerrors.ErrInvalidRequestParameters,
			)
		}
	})

	t.Run("defaults", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mock.NewMockTestExecutionRepository(ctrl)

		repo.EXPECT().
			List(
				gomock.Any(),
				&repository.ListParams{
					Limit:          11,
					Position:       repository.PositionAfter,
					OrderBy:        repository.OrderByStartedAt,
					OrderDirection: repository.OrderDirectionDesc,
				},
			).
			Return([]repository.TestExecution{
				*repositoryTestExecution(),
			}, nil)

		service := newTestExecutionService(t, repo)

		got, err := service.GetTestExecutionsPage(
			context.Background(),
			nil,
		)
		if err != nil {
			t.Fatalf(
				"GetTestExecutionsPage() error = %v",
				err,
			)
		}

		if len(got.Items) != 1 {
			t.Fatalf(
				"len(Items) = %d, want 1",
				len(got.Items),
			)
		}

		want := *modelTestExecution()

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
		repo := mock.NewMockTestExecutionRepository(ctrl)

		repo.EXPECT().
			List(
				gomock.Any(),
				&repository.ListParams{
					Limit:          6,
					Position:       repository.PositionAfter,
					OrderBy:        repository.OrderByFinishedAt,
					OrderDirection: repository.OrderDirectionAsc,
				},
			).
			Return([]repository.TestExecution{
				*repositoryTestExecution(),
			}, nil)

		service := newTestExecutionService(t, repo)

		got, err := service.GetTestExecutionsPage(
			context.Background(),
			&core.GetTestExecutionsPageParams{
				PageSize:       5,
				OrderBy:        model.OrderByFinishedAt,
				OrderDirection: model.OrderDirectionAsc,
			},
		)
		if err != nil {
			t.Fatalf(
				"GetTestExecutionsPage() error = %v",
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

func newTestExecutionService(
	t *testing.T,
	testExecutionRepository repository.TestExecutionRepository,
) *core.Service {
	t.Helper()

	service, err := core.NewService(
		core.Repositories{
			TestExecution: testExecutionRepository,
		},
		core.WithRuntimeConfig(&runtimeconfig.Config{}),
	)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	return service
}

func repositoryTestExecution() *repository.TestExecution {
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

	summary, err := json.Marshal(modelTestExecutionSummary())
	if err != nil {
		panic(err)
	}

	return &repository.TestExecution{
		ID:              "execution-001",
		StartedAt:       startedAt,
		FinishedAt:      finishedAt,
		Status:          string(model.ExecutionStatusPassed),
		EnvironmentID:   "environment-001",
		EnvironmentName: "local",
		TestID:          "test-001",
		TestName:        "health-check",
		Summary:         summary,
	}
}

func modelTestExecution() *model.TestExecution {
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

	return &model.TestExecution{
		ID:              "execution-001",
		StartedAt:       startedAt,
		FinishedAt:      finishedAt,
		Status:          model.ExecutionStatusPassed,
		EnvironmentID:   "environment-001",
		EnvironmentName: "local",
		TestID:          "test-001",
		TestName:        "health-check",
		Summary:         modelTestExecutionSummary(),
	}
}

func modelTestExecutionSummary() *model.TestExecutionSummary {
	return &model.TestExecutionSummary{
		Request: &model.TestExecutionRequestSummary{
			HTTP: &model.TestExecutionHTTPRequestSummary{
				Method: "GET",
				URL:    "http://localhost:8080/health",
			},
		},
		Response: &model.TestExecutionResponseSummary{
			HTTP: &model.TestExecutionHTTPResponseSummary{
				StatusCode: 200,
				BodyJSON:   `{"status":"ok"}`,
			},
		},
	}
}
