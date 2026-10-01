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

func TestTest_Create(t *testing.T) {
	t.Run("nil params", func(t *testing.T) {
		service := newTestService(t, nil)

		got, err := service.CreateTest(context.Background(), nil)

		if got != nil {
			t.Fatalf("CreateTest() = %#v, want nil", got)
		}
		if !errors.Is(err, pkgerrors.ErrInvalidRequestParameters) {
			t.Fatalf(
				"CreateTest() error = %v, want %v",
				err,
				pkgerrors.ErrInvalidRequestParameters,
			)
		}
	})

	t.Run("invalid test", func(t *testing.T) {
		service := newTestService(t, nil)

		got, err := service.CreateTest(
			context.Background(),
			&core.CreateTestParams{
				Name: "ab",
				Spec: validTestSpec(),
			},
		)

		if got != nil {
			t.Fatalf("CreateTest() = %#v, want nil", got)
		}
		if !errors.Is(err, pkgerrors.ErrInvalidRequestParameters) {
			t.Fatalf(
				"CreateTest() error = %v, want %v",
				err,
				pkgerrors.ErrInvalidRequestParameters,
			)
		}
	})

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mock.NewMockTestRepository(ctrl)

		spec := validTestSpec()

		repo.EXPECT().
			Create(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, arg *repository.Test) (*repository.Test, error) {
				if arg.Name != "health-check" {
					t.Errorf("Name = %q, want %q", arg.Name, "health-check")
				}
				if arg.Description != "Health check test" {
					t.Errorf(
						"Description = %q, want %q",
						arg.Description,
						"Health check test",
					)
				}
				if arg.Version != "1.0.0" {
					t.Errorf("Version = %q, want %q", arg.Version, "1.0.0")
				}

				var gotSpec model.TestSpec
				if err := json.Unmarshal(arg.Spec, &gotSpec); err != nil {
					t.Fatalf("unmarshal repository spec: %v", err)
				}
				if !reflect.DeepEqual(gotSpec, spec) {
					t.Errorf("Spec = %#v, want %#v", gotSpec, spec)
				}

				return &repository.Test{
					ID:          "test-001",
					Kind:        string(model.ResourceKindTest),
					Name:        arg.Name,
					Description: arg.Description,
					Version:     arg.Version,
					Spec:        arg.Spec,
				}, nil
			})

		service := newTestService(t, repo)

		got, err := service.CreateTest(
			context.Background(),
			&core.CreateTestParams{
				Name:        "health-check",
				Description: "Health check test",
				Spec:        spec,
			},
		)
		if err != nil {
			t.Fatalf("CreateTest() error = %v", err)
		}

		if got.ID != "test-001" {
			t.Errorf("ID = %q, want %q", got.ID, "test-001")
		}
		if got.Kind != model.ResourceKindTest {
			t.Errorf("Kind = %q, want %q", got.Kind, model.ResourceKindTest)
		}
		if got.Name != "health-check" {
			t.Errorf("Name = %q, want %q", got.Name, "health-check")
		}
		if got.Description != "Health check test" {
			t.Errorf(
				"Description = %q, want %q",
				got.Description,
				"Health check test",
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

func TestTest_Validate(t *testing.T) {
	service := newTestService(t, nil)

	tests := []struct {
		name    string
		test    *model.Test
		wantErr bool
	}{
		{
			name: "valid",
			test: &model.Test{
				Kind:    model.ResourceKindTest,
				Name:    "health-check",
				Version: "1.0.0",
				Spec:    validTestSpec(),
			},
		},
		{
			name: "invalid",
			test: &model.Test{
				Kind:    model.ResourceKindTest,
				Name:    "ab",
				Version: "1.0.0",
				Spec:    validTestSpec(),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.ValidateTest(context.Background(), tt.test)

			if tt.wantErr {
				if err == nil {
					t.Fatal("ValidateTest() error = nil, want error")
				}
				return
			}

			if err != nil {
				t.Fatalf("ValidateTest() error = %v", err)
			}
		})
	}
}

func TestTest_Get(t *testing.T) {
	t.Run("invalid reference", func(t *testing.T) {
		service := newTestService(t, nil)

		got, err := service.GetTest(context.Background(), "ab")

		if got != nil {
			t.Fatalf("GetTest() = %#v, want nil", got)
		}
		if !errors.Is(err, pkgerrors.ErrInvalidRequestParameters) {
			t.Fatalf(
				"GetTest() error = %v, want %v",
				err,
				pkgerrors.ErrInvalidRequestParameters,
			)
		}
	})

	t.Run("resolve by id", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mock.NewMockTestRepository(ctrl)

		repo.EXPECT().
			GetIDsByID(gomock.Any(), "tes").
			Return([]string{"test-001"}, nil)

		repo.EXPECT().
			Get(gomock.Any(), "test-001").
			Return(repositoryTest(), nil)

		service := newTestService(t, repo)

		got, err := service.GetTest(context.Background(), "tes")
		if err != nil {
			t.Fatalf("GetTest() error = %v", err)
		}

		if got.ID != "test-001" {
			t.Errorf("ID = %q, want %q", got.ID, "test-001")
		}
		if got.Name != "health-check" {
			t.Errorf("Name = %q, want %q", got.Name, "health-check")
		}
	})

	t.Run("resolve by name", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mock.NewMockTestRepository(ctrl)

		repo.EXPECT().
			GetIDsByID(gomock.Any(), "hea").
			Return(nil, nil)

		repo.EXPECT().
			GetIDsByName(gomock.Any(), "hea").
			Return([]string{"test-001"}, nil)

		repo.EXPECT().
			Get(gomock.Any(), "test-001").
			Return(repositoryTest(), nil)

		service := newTestService(t, repo)

		got, err := service.GetTest(context.Background(), "hea")
		if err != nil {
			t.Fatalf("GetTest() error = %v", err)
		}

		if got.ID != "test-001" {
			t.Errorf("ID = %q, want %q", got.ID, "test-001")
		}
	})
}

func TestTest_Delete(t *testing.T) {
	t.Run("invalid reference", func(t *testing.T) {
		service := newTestService(t, nil)

		got, err := service.DeleteTest(context.Background(), "ab")

		if got != nil {
			t.Fatalf("DeleteTest() = %#v, want nil", got)
		}
		if !errors.Is(err, pkgerrors.ErrInvalidRequestParameters) {
			t.Fatalf(
				"DeleteTest() error = %v, want %v",
				err,
				pkgerrors.ErrInvalidRequestParameters,
			)
		}
	})

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mock.NewMockTestRepository(ctrl)

		repo.EXPECT().
			GetIDsByID(gomock.Any(), "tes").
			Return([]string{"test-001"}, nil)

		repo.EXPECT().
			Delete(gomock.Any(), "test-001").
			Return(repositoryTest(), nil)

		service := newTestService(t, repo)

		got, err := service.DeleteTest(context.Background(), "tes")
		if err != nil {
			t.Fatalf("DeleteTest() error = %v", err)
		}

		if got.ID != "test-001" {
			t.Errorf("ID = %q, want %q", got.ID, "test-001")
		}
	})
}

func TestTest_GetPage(t *testing.T) {
	t.Run("invalid page size", func(t *testing.T) {
		service := newTestService(t, nil)

		got, err := service.GetTestsPage(
			context.Background(),
			&core.GetTestsPageParams{
				PageSize: 101,
			},
		)

		if got != nil {
			t.Fatalf("GetTestsPage() = %#v, want nil", got)
		}
		if !errors.Is(err, pkgerrors.ErrInvalidRequestParameters) {
			t.Fatalf(
				"GetTestsPage() error = %v, want %v",
				err,
				pkgerrors.ErrInvalidRequestParameters,
			)
		}
	})

	t.Run("invalid order by", func(t *testing.T) {
		service := newTestService(t, nil)

		got, err := service.GetTestsPage(
			context.Background(),
			&core.GetTestsPageParams{
				PageSize: 10,
				OrderBy:  model.OrderByStartedAt,
			},
		)

		if got != nil {
			t.Fatalf("GetTestsPage() = %#v, want nil", got)
		}
		if !errors.Is(err, pkgerrors.ErrInvalidRequestParameters) {
			t.Fatalf(
				"GetTestsPage() error = %v, want %v",
				err,
				pkgerrors.ErrInvalidRequestParameters,
			)
		}
	})

	t.Run("invalid order direction", func(t *testing.T) {
		service := newTestService(t, nil)

		got, err := service.GetTestsPage(
			context.Background(),
			&core.GetTestsPageParams{
				PageSize:       10,
				OrderDirection: model.OrderDirection("sideways"),
			},
		)

		if got != nil {
			t.Fatalf("GetTestsPage() = %#v, want nil", got)
		}
		if !errors.Is(err, pkgerrors.ErrInvalidRequestParameters) {
			t.Fatalf(
				"GetTestsPage() error = %v, want %v",
				err,
				pkgerrors.ErrInvalidRequestParameters,
			)
		}
	})

	t.Run("defaults", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mock.NewMockTestRepository(ctrl)

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
			Return([]repository.Test{
				*repositoryTest(),
			}, nil)

		service := newTestService(t, repo)

		got, err := service.GetTestsPage(context.Background(), nil)
		if err != nil {
			t.Fatalf("GetTestsPage() error = %v", err)
		}

		if len(got.Items) != 1 {
			t.Fatalf("len(Items) = %d, want 1", len(got.Items))
		}
		if got.Items[0].ID != "test-001" {
			t.Errorf(
				"Items[0].ID = %q, want %q",
				got.Items[0].ID,
				"test-001",
			)
		}
	})

	t.Run("page size and ordering", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mock.NewMockTestRepository(ctrl)

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
			Return([]repository.Test{
				*repositoryTest(),
			}, nil)

		service := newTestService(t, repo)

		got, err := service.GetTestsPage(
			context.Background(),
			&core.GetTestsPageParams{
				PageSize:       5,
				OrderBy:        model.OrderByName,
				OrderDirection: model.OrderDirectionDesc,
			},
		)
		if err != nil {
			t.Fatalf("GetTestsPage() error = %v", err)
		}

		if len(got.Items) != 1 {
			t.Fatalf("len(Items) = %d, want 1", len(got.Items))
		}
	})
}

func newTestService(
	t *testing.T,
	testRepository repository.TestRepository,
) *core.Service {
	t.Helper()

	service, err := core.NewService(
		core.Repositories{
			Test: testRepository,
		},
		core.WithRuntimeConfig(&runtimeconfig.Config{}),
	)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	return service
}

func validTestSpec() model.TestSpec {
	return model.TestSpec{
		Tags: []string{"smoke"},
		Request: model.RequestSpec{
			HTTP: &model.HTTPRequestSpec{
				Method: "GET",
				URL:    "http://localhost:8080/health",
			},
		},
		Expect: model.ExpectSpec{
			HTTP: &model.HTTPExpectSpec{
				Status: 200,
			},
		},
	}
}

func repositoryTest() *repository.Test {
	spec, err := json.Marshal(validTestSpec())
	if err != nil {
		panic(err)
	}

	now := time.Date(2026, time.September, 21, 12, 0, 0, 0, time.UTC)

	return &repository.Test{
		ID:          "test-001",
		Kind:        string(model.ResourceKindTest),
		Version:     "1.0.0",
		Name:        "health-check",
		Description: "Health check test",
		Spec:        spec,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}
