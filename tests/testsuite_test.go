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

func TestTestSuite_Create(t *testing.T) {
	t.Run("nil params", func(t *testing.T) {
		service := newTestSuiteService(t, nil)

		got, err := service.CreateTestSuite(context.Background(), nil)

		if got != nil {
			t.Fatalf("CreateTestSuite() = %#v, want nil", got)
		}
		if !errors.Is(err, pkgerrors.ErrInvalidRequestParameters) {
			t.Fatalf(
				"CreateTestSuite() error = %v, want %v",
				err,
				pkgerrors.ErrInvalidRequestParameters,
			)
		}
	})

	t.Run("invalid testsuite", func(t *testing.T) {
		service := newTestSuiteService(t, nil)

		got, err := service.CreateTestSuite(
			context.Background(),
			&core.CreateTestSuiteParams{
				Name: "ab",
				Spec: validTestSuiteSpec(),
			},
		)

		if got != nil {
			t.Fatalf("CreateTestSuite() = %#v, want nil", got)
		}
		if !errors.Is(err, pkgerrors.ErrInvalidRequestParameters) {
			t.Fatalf(
				"CreateTestSuite() error = %v, want %v",
				err,
				pkgerrors.ErrInvalidRequestParameters,
			)
		}
	})

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mock.NewMockResourceRepository[repository.TestSuite](ctrl)

		spec := validTestSuiteSpec()

		repo.EXPECT().
			Create(gomock.Any(), gomock.Any()).
			DoAndReturn(func(
				_ context.Context,
				arg *repository.TestSuite,
			) (*repository.TestSuite, error) {
				if arg.Name != "smoke-suite" {
					t.Errorf(
						"Name = %q, want %q",
						arg.Name,
						"smoke-suite",
					)
				}
				if arg.Description != "Smoke test suite" {
					t.Errorf(
						"Description = %q, want %q",
						arg.Description,
						"Smoke test suite",
					)
				}
				if arg.Version != "1.0.0" {
					t.Errorf(
						"Version = %q, want %q",
						arg.Version,
						"1.0.0",
					)
				}

				var gotSpec model.TestSuiteSpec
				if err := json.Unmarshal(arg.Spec, &gotSpec); err != nil {
					t.Fatalf("unmarshal repository spec: %v", err)
				}
				if !reflect.DeepEqual(gotSpec, spec) {
					t.Errorf(
						"Spec = %#v, want %#v",
						gotSpec,
						spec,
					)
				}

				return &repository.TestSuite{
					ID:          "testsuite-001",
					Kind:        string(model.ResourceKindTestSuite),
					Name:        arg.Name,
					Description: arg.Description,
					Version:     arg.Version,
					Spec:        arg.Spec,
				}, nil
			})

		service := newTestSuiteService(t, repo)

		got, err := service.CreateTestSuite(
			context.Background(),
			&core.CreateTestSuiteParams{
				Name:        "smoke-suite",
				Description: "Smoke test suite",
				Spec:        spec,
			},
		)
		if err != nil {
			t.Fatalf("CreateTestSuite() error = %v", err)
		}

		if got.ID != "testsuite-001" {
			t.Errorf(
				"ID = %q, want %q",
				got.ID,
				"testsuite-001",
			)
		}
		if got.Kind != model.ResourceKindTestSuite {
			t.Errorf(
				"Kind = %q, want %q",
				got.Kind,
				model.ResourceKindTestSuite,
			)
		}
		if got.Name != "smoke-suite" {
			t.Errorf(
				"Name = %q, want %q",
				got.Name,
				"smoke-suite",
			)
		}
		if got.Description != "Smoke test suite" {
			t.Errorf(
				"Description = %q, want %q",
				got.Description,
				"Smoke test suite",
			)
		}
		if got.Version != "1.0.0" {
			t.Errorf(
				"Version = %q, want %q",
				got.Version,
				"1.0.0",
			)
		}
		if !reflect.DeepEqual(got.Spec, spec) {
			t.Errorf(
				"Spec = %#v, want %#v",
				got.Spec,
				spec,
			)
		}
	})
}

func TestTestSuite_Validate(t *testing.T) {
	service := newTestSuiteService(t, nil)

	tests := []struct {
		name      string
		testSuite *model.TestSuite
		wantErr   bool
	}{
		{
			name: "valid",
			testSuite: &model.TestSuite{
				Kind:    model.ResourceKindTestSuite,
				Name:    "smoke-suite",
				Version: "1.0.0",
				Spec:    validTestSuiteSpec(),
			},
		},
		{
			name: "invalid",
			testSuite: &model.TestSuite{
				Kind:    model.ResourceKindTestSuite,
				Name:    "ab",
				Version: "1.0.0",
				Spec:    validTestSuiteSpec(),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.ValidateTestSuite(
				context.Background(),
				tt.testSuite,
			)

			if tt.wantErr {
				if err == nil {
					t.Fatal("ValidateTestSuite() error = nil, want error")
				}
				return
			}

			if err != nil {
				t.Fatalf("ValidateTestSuite() error = %v", err)
			}
		})
	}
}

func TestTestSuite_Get(t *testing.T) {
	t.Run("invalid reference", func(t *testing.T) {
		service := newTestSuiteService(t, nil)

		got, err := service.GetTestSuite(
			context.Background(),
			"ab",
		)

		if got != nil {
			t.Fatalf("GetTestSuite() = %#v, want nil", got)
		}
		if !errors.Is(err, pkgerrors.ErrInvalidRequestParameters) {
			t.Fatalf(
				"GetTestSuite() error = %v, want %v",
				err,
				pkgerrors.ErrInvalidRequestParameters,
			)
		}
	})

	t.Run("resolve by id", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mock.NewMockResourceRepository[repository.TestSuite](ctrl)

		repo.EXPECT().
			GetIDsByID(gomock.Any(), "tes").
			Return([]string{"testsuite-001"}, nil)

		repo.EXPECT().
			Get(gomock.Any(), "testsuite-001").
			Return(repositoryTestSuite(), nil)

		service := newTestSuiteService(t, repo)

		got, err := service.GetTestSuite(
			context.Background(),
			"tes",
		)
		if err != nil {
			t.Fatalf("GetTestSuite() error = %v", err)
		}

		if got.ID != "testsuite-001" {
			t.Errorf(
				"ID = %q, want %q",
				got.ID,
				"testsuite-001",
			)
		}
		if got.Name != "smoke-suite" {
			t.Errorf(
				"Name = %q, want %q",
				got.Name,
				"smoke-suite",
			)
		}
	})

	t.Run("resolve by name", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mock.NewMockResourceRepository[repository.TestSuite](ctrl)

		repo.EXPECT().
			GetIDsByID(gomock.Any(), "smo").
			Return(nil, nil)

		repo.EXPECT().
			GetIDsByName(gomock.Any(), "smo").
			Return([]string{"testsuite-001"}, nil)

		repo.EXPECT().
			Get(gomock.Any(), "testsuite-001").
			Return(repositoryTestSuite(), nil)

		service := newTestSuiteService(t, repo)

		got, err := service.GetTestSuite(
			context.Background(),
			"smo",
		)
		if err != nil {
			t.Fatalf("GetTestSuite() error = %v", err)
		}

		if got.ID != "testsuite-001" {
			t.Errorf(
				"ID = %q, want %q",
				got.ID,
				"testsuite-001",
			)
		}
	})
}

func TestTestSuite_Delete(t *testing.T) {
	t.Run("invalid reference", func(t *testing.T) {
		service := newTestSuiteService(t, nil)

		got, err := service.DeleteTestSuite(
			context.Background(),
			"ab",
		)

		if got != nil {
			t.Fatalf("DeleteTestSuite() = %#v, want nil", got)
		}
		if !errors.Is(err, pkgerrors.ErrInvalidRequestParameters) {
			t.Fatalf(
				"DeleteTestSuite() error = %v, want %v",
				err,
				pkgerrors.ErrInvalidRequestParameters,
			)
		}
	})

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mock.NewMockResourceRepository[repository.TestSuite](ctrl)

		repo.EXPECT().
			GetIDsByID(gomock.Any(), "tes").
			Return([]string{"testsuite-001"}, nil)

		repo.EXPECT().
			Delete(gomock.Any(), "testsuite-001").
			Return(repositoryTestSuite(), nil)

		service := newTestSuiteService(t, repo)

		got, err := service.DeleteTestSuite(
			context.Background(),
			"tes",
		)
		if err != nil {
			t.Fatalf("DeleteTestSuite() error = %v", err)
		}

		if got.ID != "testsuite-001" {
			t.Errorf(
				"ID = %q, want %q",
				got.ID,
				"testsuite-001",
			)
		}
	})
}

func TestTestSuite_GetPage(t *testing.T) {
	t.Run("invalid page size", func(t *testing.T) {
		service := newTestSuiteService(t, nil)

		got, err := service.GetTestSuitesPage(
			context.Background(),
			&core.GetTestSuitesPageParams{
				PageSize: 101,
			},
		)

		if got != nil {
			t.Fatalf(
				"GetTestSuitesPage() = %#v, want nil",
				got,
			)
		}
		if !errors.Is(err, pkgerrors.ErrInvalidRequestParameters) {
			t.Fatalf(
				"GetTestSuitesPage() error = %v, want %v",
				err,
				pkgerrors.ErrInvalidRequestParameters,
			)
		}
	})

	t.Run("invalid order by", func(t *testing.T) {
		service := newTestSuiteService(t, nil)

		got, err := service.GetTestSuitesPage(
			context.Background(),
			&core.GetTestSuitesPageParams{
				PageSize: 10,
				OrderBy:  model.OrderByStartedAt,
			},
		)

		if got != nil {
			t.Fatalf(
				"GetTestSuitesPage() = %#v, want nil",
				got,
			)
		}
		if !errors.Is(err, pkgerrors.ErrInvalidRequestParameters) {
			t.Fatalf(
				"GetTestSuitesPage() error = %v, want %v",
				err,
				pkgerrors.ErrInvalidRequestParameters,
			)
		}
	})

	t.Run("invalid order direction", func(t *testing.T) {
		service := newTestSuiteService(t, nil)

		got, err := service.GetTestSuitesPage(
			context.Background(),
			&core.GetTestSuitesPageParams{
				PageSize:       10,
				OrderDirection: model.OrderDirection("sideways"),
			},
		)

		if got != nil {
			t.Fatalf(
				"GetTestSuitesPage() = %#v, want nil",
				got,
			)
		}
		if !errors.Is(err, pkgerrors.ErrInvalidRequestParameters) {
			t.Fatalf(
				"GetTestSuitesPage() error = %v, want %v",
				err,
				pkgerrors.ErrInvalidRequestParameters,
			)
		}
	})

	t.Run("defaults", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mock.NewMockResourceRepository[repository.TestSuite](ctrl)

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
			Return([]repository.TestSuite{
				*repositoryTestSuite(),
			}, nil)

		service := newTestSuiteService(t, repo)

		got, err := service.GetTestSuitesPage(
			context.Background(),
			nil,
		)
		if err != nil {
			t.Fatalf("GetTestSuitesPage() error = %v", err)
		}

		if len(got.Items) != 1 {
			t.Fatalf(
				"len(Items) = %d, want 1",
				len(got.Items),
			)
		}
		if got.Items[0].ID != "testsuite-001" {
			t.Errorf(
				"Items[0].ID = %q, want %q",
				got.Items[0].ID,
				"testsuite-001",
			)
		}
	})

	t.Run("page size and ordering", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mock.NewMockResourceRepository[repository.TestSuite](ctrl)

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
			Return([]repository.TestSuite{
				*repositoryTestSuite(),
			}, nil)

		service := newTestSuiteService(t, repo)

		got, err := service.GetTestSuitesPage(
			context.Background(),
			&core.GetTestSuitesPageParams{
				PageSize:       5,
				OrderBy:        model.OrderByName,
				OrderDirection: model.OrderDirectionDesc,
			},
		)
		if err != nil {
			t.Fatalf("GetTestSuitesPage() error = %v", err)
		}

		if len(got.Items) != 1 {
			t.Fatalf(
				"len(Items) = %d, want 1",
				len(got.Items),
			)
		}
	})
}

func newTestSuiteService(
	t *testing.T,
	testSuiteRepository repository.TestSuiteRepository,
) *core.Service {
	t.Helper()

	service, err := core.NewService(
		core.Repositories{
			TestSuite: testSuiteRepository,
		},
		core.WithRuntimeConfig(&runtimeconfig.Config{}),
	)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	return service
}

func validTestSuiteSpec() model.TestSuiteSpec {
	return model.TestSuiteSpec{
		Selectors: model.TestSelectorsSpec{
			Tags: []string{"smoke"},
		},
	}
}

func repositoryTestSuite() *repository.TestSuite {
	spec, err := json.Marshal(validTestSuiteSpec())
	if err != nil {
		panic(err)
	}

	now := time.Date(
		2026,
		time.September,
		21,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	return &repository.TestSuite{
		ID:          "testsuite-001",
		Kind:        string(model.ResourceKindTestSuite),
		Version:     "1.0.0",
		Name:        "smoke-suite",
		Description: "Smoke test suite",
		Spec:        spec,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}
