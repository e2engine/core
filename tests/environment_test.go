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

func TestEnvironment_Create(t *testing.T) {
	t.Run("nil params", func(t *testing.T) {
		service := newEnvironmentService(t, nil)

		got, err := service.CreateEnvironment(context.Background(), nil)

		if got != nil {
			t.Fatalf("CreateEnvironment() = %#v, want nil", got)
		}
		if !errors.Is(err, pkgerrors.ErrInvalidRequestParameters) {
			t.Fatalf(
				"CreateEnvironment() error = %v, want %v",
				err,
				pkgerrors.ErrInvalidRequestParameters,
			)
		}
	})

	t.Run("invalid environment", func(t *testing.T) {
		service := newEnvironmentService(t, nil)

		got, err := service.CreateEnvironment(
			context.Background(),
			&core.CreateEnvironmentParams{
				Name: "ab",
				Spec: validEnvironmentSpec(),
			},
		)

		if got != nil {
			t.Fatalf("CreateEnvironment() = %#v, want nil", got)
		}
		if !errors.Is(err, pkgerrors.ErrInvalidRequestParameters) {
			t.Fatalf(
				"CreateEnvironment() error = %v, want %v",
				err,
				pkgerrors.ErrInvalidRequestParameters,
			)
		}
	})

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mock.NewMockResourceRepository[repository.Environment](ctrl)

		spec := validEnvironmentSpec()

		repo.EXPECT().
			Create(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, arg *repository.Environment) (*repository.Environment, error) {
				if arg.Name != "local" {
					t.Errorf("Name = %q, want %q", arg.Name, "local")
				}
				if arg.Description != "Local environment" {
					t.Errorf(
						"Description = %q, want %q",
						arg.Description,
						"Local environment",
					)
				}
				if arg.Version != "1.0.0" {
					t.Errorf("Version = %q, want %q", arg.Version, "1.0.0")
				}

				var gotSpec model.EnvironmentSpec
				if err := json.Unmarshal(arg.Spec, &gotSpec); err != nil {
					t.Fatalf("unmarshal repository spec: %v", err)
				}
				if !reflect.DeepEqual(gotSpec, spec) {
					t.Errorf("Spec = %#v, want %#v", gotSpec, spec)
				}

				return &repository.Environment{
					ID:          "environment-001",
					Kind:        string(model.ResourceKindEnvironment),
					Name:        arg.Name,
					Description: arg.Description,
					Version:     arg.Version,
					Spec:        arg.Spec,
				}, nil
			})

		service := newEnvironmentService(t, repo)

		got, err := service.CreateEnvironment(
			context.Background(),
			&core.CreateEnvironmentParams{
				Name:        "local",
				Description: "Local environment",
				Spec:        spec,
			},
		)
		if err != nil {
			t.Fatalf("CreateEnvironment() error = %v", err)
		}

		if got.ID != "environment-001" {
			t.Errorf("ID = %q, want %q", got.ID, "environment-001")
		}
		if got.Kind != model.ResourceKindEnvironment {
			t.Errorf("Kind = %q, want %q", got.Kind, model.ResourceKindEnvironment)
		}
		if got.Name != "local" {
			t.Errorf("Name = %q, want %q", got.Name, "local")
		}
		if got.Description != "Local environment" {
			t.Errorf(
				"Description = %q, want %q",
				got.Description,
				"Local environment",
			)
		}
		if got.Version != "1.0.0" {
			t.Errorf("Version = %q, want %q", got.Version, "1.0.0")
		}
		if !reflect.DeepEqual(got.Spec, spec) {
			t.Errorf("Spec = %#v, want %#v", got.Spec, spec)
		}
	})
}

func TestEnvironment_Validate(t *testing.T) {
	service := newEnvironmentService(t, nil)

	tests := []struct {
		name    string
		env     *model.Environment
		wantErr bool
	}{
		{
			name: "valid",
			env: &model.Environment{
				Kind:    model.ResourceKindEnvironment,
				Name:    "local",
				Version: "1.0.0",
				Spec:    validEnvironmentSpec(),
			},
		},
		{
			name: "invalid",
			env: &model.Environment{
				Kind:    model.ResourceKindEnvironment,
				Name:    "ab",
				Version: "1.0.0",
				Spec:    validEnvironmentSpec(),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.ValidateEnvironment(context.Background(), tt.env)

			if tt.wantErr {
				if err == nil {
					t.Fatal("ValidateEnvironment() error = nil, want error")
				}
				return
			}

			if err != nil {
				t.Fatalf("ValidateEnvironment() error = %v", err)
			}
		})
	}
}

func TestEnvironment_Get(t *testing.T) {
	t.Run("invalid reference", func(t *testing.T) {
		service := newEnvironmentService(t, nil)

		got, err := service.GetEnvironment(context.Background(), "ab")

		if got != nil {
			t.Fatalf("GetEnvironment() = %#v, want nil", got)
		}
		if !errors.Is(err, pkgerrors.ErrInvalidRequestParameters) {
			t.Fatalf(
				"GetEnvironment() error = %v, want %v",
				err,
				pkgerrors.ErrInvalidRequestParameters,
			)
		}
	})

	t.Run("resolve by id", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mock.NewMockResourceRepository[repository.Environment](ctrl)

		repo.EXPECT().
			GetIDsByID(gomock.Any(), "env").
			Return([]string{"environment-001"}, nil)

		repo.EXPECT().
			Get(gomock.Any(), "environment-001").
			Return(repositoryEnvironment(), nil)

		service := newEnvironmentService(t, repo)

		got, err := service.GetEnvironment(context.Background(), "env")
		if err != nil {
			t.Fatalf("GetEnvironment() error = %v", err)
		}

		if got.ID != "environment-001" {
			t.Errorf("ID = %q, want %q", got.ID, "environment-001")
		}
		if got.Name != "local" {
			t.Errorf("Name = %q, want %q", got.Name, "local")
		}
	})

	t.Run("resolve by name", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mock.NewMockResourceRepository[repository.Environment](ctrl)

		repo.EXPECT().
			GetIDsByID(gomock.Any(), "loc").
			Return(nil, nil)

		repo.EXPECT().
			GetIDsByName(gomock.Any(), "loc").
			Return([]string{"environment-001"}, nil)

		repo.EXPECT().
			Get(gomock.Any(), "environment-001").
			Return(repositoryEnvironment(), nil)

		service := newEnvironmentService(t, repo)

		got, err := service.GetEnvironment(context.Background(), "loc")
		if err != nil {
			t.Fatalf("GetEnvironment() error = %v", err)
		}

		if got.ID != "environment-001" {
			t.Errorf("ID = %q, want %q", got.ID, "environment-001")
		}
	})
}

func TestEnvironment_Delete(t *testing.T) {
	t.Run("invalid reference", func(t *testing.T) {
		service := newEnvironmentService(t, nil)

		got, err := service.DeleteEnvironment(context.Background(), "ab")

		if got != nil {
			t.Fatalf("DeleteEnvironment() = %#v, want nil", got)
		}
		if !errors.Is(err, pkgerrors.ErrInvalidRequestParameters) {
			t.Fatalf(
				"DeleteEnvironment() error = %v, want %v",
				err,
				pkgerrors.ErrInvalidRequestParameters,
			)
		}
	})

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mock.NewMockResourceRepository[repository.Environment](ctrl)

		repo.EXPECT().
			GetIDsByID(gomock.Any(), "env").
			Return([]string{"environment-001"}, nil)

		repo.EXPECT().
			Delete(gomock.Any(), "environment-001").
			Return(repositoryEnvironment(), nil)

		service := newEnvironmentService(t, repo)

		got, err := service.DeleteEnvironment(context.Background(), "env")
		if err != nil {
			t.Fatalf("DeleteEnvironment() error = %v", err)
		}

		if got.ID != "environment-001" {
			t.Errorf("ID = %q, want %q", got.ID, "environment-001")
		}
	})
}

func TestEnvironment_GetPage(t *testing.T) {
	t.Run("invalid page size", func(t *testing.T) {
		service := newEnvironmentService(t, nil)

		got, err := service.GetEnvironmentsPage(
			context.Background(),
			&core.GetEnvironmentsPageParams{
				PageSize: 101,
			},
		)

		if got != nil {
			t.Fatalf("GetEnvironmentsPage() = %#v, want nil", got)
		}
		if !errors.Is(err, pkgerrors.ErrInvalidRequestParameters) {
			t.Fatalf(
				"GetEnvironmentsPage() error = %v, want %v",
				err,
				pkgerrors.ErrInvalidRequestParameters,
			)
		}
	})

	t.Run("invalid order by", func(t *testing.T) {
		service := newEnvironmentService(t, nil)

		got, err := service.GetEnvironmentsPage(
			context.Background(),
			&core.GetEnvironmentsPageParams{
				PageSize: 10,
				OrderBy:  model.OrderBy("startedAt"),
			},
		)

		if got != nil {
			t.Fatalf("GetEnvironmentsPage() = %#v, want nil", got)
		}
		if !errors.Is(err, pkgerrors.ErrInvalidRequestParameters) {
			t.Fatalf(
				"GetEnvironmentsPage() error = %v, want %v",
				err,
				pkgerrors.ErrInvalidRequestParameters,
			)
		}
	})

	t.Run("invalid order direction", func(t *testing.T) {
		service := newEnvironmentService(t, nil)

		got, err := service.GetEnvironmentsPage(
			context.Background(),
			&core.GetEnvironmentsPageParams{
				PageSize:       10,
				OrderDirection: model.OrderDirection("sideways"),
			},
		)

		if got != nil {
			t.Fatalf("GetEnvironmentsPage() = %#v, want nil", got)
		}
		if !errors.Is(err, pkgerrors.ErrInvalidRequestParameters) {
			t.Fatalf(
				"GetEnvironmentsPage() error = %v, want %v",
				err,
				pkgerrors.ErrInvalidRequestParameters,
			)
		}
	})

	t.Run("defaults", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mock.NewMockResourceRepository[repository.Environment](ctrl)

		repo.EXPECT().
			List(
				gomock.Any(),
				&repository.ListParams{
					Limit:          11,
					Position:       repository.PositionAfter,
					OrderBy:        repository.OrderByUpdatedAt,
					OrderDirection: repository.OrderDirectionAsc,
				},
			).
			Return([]repository.Environment{
				*repositoryEnvironment(),
			}, nil)

		service := newEnvironmentService(t, repo)

		got, err := service.GetEnvironmentsPage(context.Background(), nil)
		if err != nil {
			t.Fatalf("GetEnvironmentsPage() error = %v", err)
		}

		if len(got.Items) != 1 {
			t.Fatalf("len(Items) = %d, want 1", len(got.Items))
		}
		if got.Items[0].ID != "environment-001" {
			t.Errorf(
				"Items[0].ID = %q, want %q",
				got.Items[0].ID,
				"environment-001",
			)
		}
	})

	t.Run("page size and ordering", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mock.NewMockResourceRepository[repository.Environment](ctrl)

		repo.EXPECT().
			List(
				gomock.Any(),
				&repository.ListParams{
					Limit:          6,
					Position:       repository.PositionAfter,
					OrderBy:        repository.OrderByName,
					OrderDirection: repository.OrderDirectionDesc,
				},
			).
			Return([]repository.Environment{
				*repositoryEnvironment(),
			}, nil)

		service := newEnvironmentService(t, repo)

		got, err := service.GetEnvironmentsPage(
			context.Background(),
			&core.GetEnvironmentsPageParams{
				PageSize:       5,
				OrderBy:        model.OrderByName,
				OrderDirection: model.OrderDirectionDesc,
			},
		)
		if err != nil {
			t.Fatalf("GetEnvironmentsPage() error = %v", err)
		}

		if len(got.Items) != 1 {
			t.Fatalf("len(Items) = %d, want 1", len(got.Items))
		}
	})
}

func newEnvironmentService(
	t *testing.T,
	environmentRepository repository.EnvironmentRepository,
) *core.Service {
	t.Helper()

	service, err := core.NewService(
		core.Repositories{
			Environment: environmentRepository,
		},
		core.WithRuntimeConfig(&runtimeconfig.Config{}),
	)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	return service
}

func validEnvironmentSpec() model.EnvironmentSpec {
	return model.EnvironmentSpec{
		Services: []model.ServiceSpec{
			{
				ID:         "api",
				Kind:       model.ServiceKindHTTP,
				Mode:       model.ServiceModeReal,
				Address:    "api:8080",
				HTTPTarget: "http://localhost:8080",
			},
		},
	}
}

func repositoryEnvironment() *repository.Environment {
	spec, err := json.Marshal(validEnvironmentSpec())
	if err != nil {
		panic(err)
	}

	now := time.Date(2026, time.September, 21, 12, 0, 0, 0, time.UTC)

	return &repository.Environment{
		ID:          "environment-001",
		Kind:        string(model.ResourceKindEnvironment),
		Version:     "1.0.0",
		Name:        "local",
		Description: "Local environment",
		Spec:        spec,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}
