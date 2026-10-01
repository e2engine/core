package resource

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/e2engine/core/mock"
	"github.com/e2engine/core/model"
	coreerrors "github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/repository"
)

func TestGetEnvironmentCursorValue(t *testing.T) {
	createdAt := time.Date(
		2026, time.September, 20,
		10, 11, 12, 123456789,
		time.UTC,
	)
	updatedAt := createdAt.Add(time.Hour)

	environment := &model.Environment{
		ID:        "env-001",
		Name:      "development",
		Version:   "1.0.0",
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}

	tests := []struct {
		name     string
		orderBy  model.OrderBy
		expected string
	}{
		{
			name:     "id",
			orderBy:  model.OrderByID,
			expected: environment.ID,
		},
		{
			name:     "name",
			orderBy:  model.OrderByName,
			expected: environment.Name,
		},
		{
			name:     "version",
			orderBy:  model.OrderByVersion,
			expected: environment.Version,
		},
		{
			name:     "created at",
			orderBy:  model.OrderByCreatedAt,
			expected: createdAt.Format(time.RFC3339Nano),
		},
		{
			name:     "updated at",
			orderBy:  model.OrderByUpdatedAt,
			expected: updatedAt.Format(time.RFC3339Nano),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := getEnvironmentCursorValue(
				environment,
				tt.orderBy,
			)

			if actual != tt.expected {
				t.Fatalf(
					"expected %q, got %q",
					tt.expected,
					actual,
				)
			}
		})
	}
}

func TestGetEnvironmentCursorValueUnsupported(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()

	getEnvironmentCursorValue(
		&model.Environment{},
		model.OrderBy("unsupported"),
	)
}

func TestNewEnvironmentFromRepository(t *testing.T) {
	createdAt := time.Date(
		2026, time.September, 20,
		10, 0, 0, 0,
		time.UTC,
	)
	updatedAt := createdAt.Add(time.Hour)

	spec := model.EnvironmentSpec{}

	specJSON, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("marshal spec: %v", err)
	}

	tests := []struct {
		name string

		input *repository.Environment

		expected      *model.Environment
		expectedError error
	}{
		{
			name: "nil",
		},
		{
			name: "success",
			input: &repository.Environment{
				ID:          "env-001",
				Version:     "1.0.0",
				Name:        "development",
				Description: "Development environment",
				Spec:        specJSON,
				CreatedAt:   createdAt,
				UpdatedAt:   updatedAt,
			},
			expected: &model.Environment{
				Kind:        model.ResourceKindEnvironment,
				ID:          "env-001",
				Version:     "1.0.0",
				Name:        "development",
				Description: "Development environment",
				Spec:        spec,
				CreatedAt:   createdAt,
				UpdatedAt:   updatedAt,
			},
		},
		{
			name: "invalid spec",
			input: &repository.Environment{
				ID:   "env-001",
				Spec: []byte("{"),
			},
			expectedError: coreerrors.ErrCannotParseSpec,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := newEnvironmentFromRepository(
				tt.input,
			)

			if tt.expectedError != nil {
				if !errors.Is(err, tt.expectedError) {
					t.Fatalf(
						"expected error %v, got %v",
						tt.expectedError,
						err,
					)
				}

				if actual != nil {
					t.Fatalf(
						"expected nil environment, got %+v",
						actual,
					)
				}

				return
			}

			if err != nil {
				t.Fatalf(
					"expected no error, got %v",
					err,
				)
			}

			if !reflect.DeepEqual(actual, tt.expected) {
				t.Fatalf(
					"expected\n%+v\ngot\n%+v",
					tt.expected,
					actual,
				)
			}
		})
	}
}

func TestNewEnvironmentSliceFromRepository(t *testing.T) {
	specJSON, err := json.Marshal(model.EnvironmentSpec{})
	if err != nil {
		t.Fatalf("marshal spec: %v", err)
	}

	items := []repository.Environment{
		{
			ID:   "env-001",
			Spec: specJSON,
		},
		{
			ID:   "env-002",
			Spec: specJSON,
		},
		{
			ID:   "env-003",
			Spec: specJSON,
		},
	}

	invalid := repository.Environment{
		ID:   "env-invalid",
		Spec: []byte("{"),
	}

	tests := []struct {
		name string

		input                    []repository.Environment
		restorePresentationOrder bool

		expectedIDs   []string
		expectedError error
	}{
		{
			name:        "nil",
			expectedIDs: []string{},
		},
		{
			name:        "empty",
			input:       []repository.Environment{},
			expectedIDs: []string{},
		},
		{
			name:                     "forward",
			input:                    items,
			restorePresentationOrder: false,
			expectedIDs: []string{
				"env-001",
				"env-002",
				"env-003",
			},
		},
		{
			name:                     "reverse",
			input:                    items,
			restorePresentationOrder: true,
			expectedIDs: []string{
				"env-003",
				"env-002",
				"env-001",
			},
		},
		{
			name: "invalid first",
			input: []repository.Environment{
				invalid,
				items[0],
				items[1],
			},
			expectedError: coreerrors.ErrCannotParseSpec,
		},
		{
			name: "invalid middle",
			input: []repository.Environment{
				items[0],
				invalid,
				items[2],
			},
			expectedError: coreerrors.ErrCannotParseSpec,
		},
		{
			name: "invalid last while reversing",
			input: []repository.Environment{
				items[0],
				items[1],
				invalid,
			},
			restorePresentationOrder: true,
			expectedError:            coreerrors.ErrCannotParseSpec,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := newEnvironmentSliceFromRepository(
				tt.input,
				tt.restorePresentationOrder,
			)

			if tt.expectedError != nil {
				if !errors.Is(err, tt.expectedError) {
					t.Fatalf(
						"expected error %v, got %v",
						tt.expectedError,
						err,
					)
				}

				return
			}

			if err != nil {
				t.Fatalf(
					"expected no error, got %v",
					err,
				)
			}

			if len(actual) != len(tt.expectedIDs) {
				t.Fatalf(
					"expected %d items, got %d",
					len(tt.expectedIDs),
					len(actual),
				)
			}

			for i := range actual {
				if actual[i].ID != tt.expectedIDs[i] {
					t.Fatalf(
						"item %d: expected ID %q, got %q",
						i,
						tt.expectedIDs[i],
						actual[i].ID,
					)
				}
			}
		})
	}
}

func TestControllerGetEnvironment(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string

		ref string

		prepare func(
			*mock.MockResourceRepository[repository.Environment],
		)

		expectedID    string
		expectedError error
	}{
		{
			name: "success by unique id prefix",
			ref:  "env",
			prepare: func(
				repo *mock.MockResourceRepository[repository.Environment],
			) {
				repo.EXPECT().
					GetIDsByID(ctx, "env").
					Return([]string{"env-001"}, nil)

				repo.EXPECT().
					Get(ctx, "env-001").
					Return(
						&repository.Environment{
							ID:   "env-001",
							Spec: []byte("{}"),
						},
						nil,
					)
			},
			expectedID: "env-001",
		},
		{
			name: "success by name after ambiguous id prefix",
			ref:  "dev",
			prepare: func(
				repo *mock.MockResourceRepository[repository.Environment],
			) {
				repo.EXPECT().
					GetIDsByID(ctx, "dev").
					Return(
						[]string{
							"dev-001",
							"dev-002",
						},
						nil,
					)

				repo.EXPECT().
					GetIDsByName(ctx, "dev").
					Return(
						[]string{"env-001"},
						nil,
					)

				repo.EXPECT().
					Get(ctx, "env-001").
					Return(
						&repository.Environment{
							ID:   "env-001",
							Spec: []byte("{}"),
						},
						nil,
					)
			},
			expectedID: "env-001",
		},
		{
			name: "reference resolution error",
			ref:  "missing",
			prepare: func(
				repo *mock.MockResourceRepository[repository.Environment],
			) {
				repo.EXPECT().
					GetIDsByID(ctx, "missing").
					Return(nil, nil)

				repo.EXPECT().
					GetIDsByName(ctx, "missing").
					Return(nil, nil)
			},
			expectedError: coreerrors.ErrEntityNotFound,
		},
		{
			name: "repository not found",
			ref:  "env",
			prepare: func(
				repo *mock.MockResourceRepository[repository.Environment],
			) {
				repo.EXPECT().
					GetIDsByID(ctx, "env").
					Return([]string{"env-001"}, nil)

				repo.EXPECT().
					Get(ctx, "env-001").
					Return(nil, repository.ErrNotFound)
			},
			expectedError: coreerrors.ErrEntityNotFound,
		},
		{
			name: "repository error",
			ref:  "env",
			prepare: func(
				repo *mock.MockResourceRepository[repository.Environment],
			) {
				repo.EXPECT().
					GetIDsByID(ctx, "env").
					Return([]string{"env-001"}, nil)

				repo.EXPECT().
					Get(ctx, "env-001").
					Return(nil, errGetEnvironment)
			},
			expectedError: errGetEnvironment,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			repo := mock.NewMockResourceRepository[repository.Environment](ctrl)

			controller := &Controller{
				environment: repo,
			}

			tt.prepare(repo)

			environment, err := controller.GetEnvironment(
				ctx,
				tt.ref,
			)

			if tt.expectedError != nil {
				if !errors.Is(err, tt.expectedError) {
					t.Fatalf(
						"expected error %v, got %v",
						tt.expectedError,
						err,
					)
				}
				return
			}

			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if environment.ID != tt.expectedID {
				t.Fatalf(
					"expected ID %q, got %q",
					tt.expectedID,
					environment.ID,
				)
			}
		})
	}
}

var errGetEnvironment = errors.New("get environment")

func TestControllerDeleteEnvironment(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string

		deleteError error

		expectedID    string
		expectedError error
	}{
		{
			name:       "success",
			expectedID: "env-001",
		},
		{
			name:          "not found",
			deleteError:   repository.ErrNotFound,
			expectedError: coreerrors.ErrEntityNotFound,
		},
		{
			name:          "conflict",
			deleteError:   repository.ErrConflict,
			expectedError: coreerrors.ErrEntityConflict,
		},
		{
			name:          "repository error",
			deleteError:   errDeleteEnvironment,
			expectedError: errDeleteEnvironment,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			repo := mock.NewMockResourceRepository[repository.Environment](ctrl)

			controller := &Controller{
				environment: repo,
			}

			repo.EXPECT().
				GetIDsByID(ctx, "env").
				Return([]string{"env-001"}, nil)

			if tt.deleteError != nil {
				repo.EXPECT().
					Delete(ctx, "env-001").
					Return(nil, tt.deleteError)
			} else {
				repo.EXPECT().
					Delete(ctx, "env-001").
					Return(
						&repository.Environment{
							ID:   "env-001",
							Spec: []byte("{}"),
						},
						nil,
					)
			}

			environment, err := controller.DeleteEnvironment(
				ctx,
				"env",
			)

			if tt.expectedError != nil {
				if !errors.Is(err, tt.expectedError) {
					t.Fatalf(
						"expected error %v, got %v",
						tt.expectedError,
						err,
					)
				}
				return
			}

			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if environment.ID != tt.expectedID {
				t.Fatalf(
					"expected ID %q, got %q",
					tt.expectedID,
					environment.ID,
				)
			}
		})
	}
}

var errDeleteEnvironment = errors.New("delete environment")

func TestControllerCreateEnvironment(t *testing.T) {
	ctx := context.Background()

	params := &CreateEnvironmentParams{
		Name:        "development",
		Description: "Development environment",
		Spec:        model.EnvironmentSpec{},
	}

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		repo := mock.NewMockResourceRepository[repository.Environment](ctrl)

		controller := &Controller{
			environment: repo,
		}

		repo.EXPECT().
			Create(ctx, gomock.Any()).
			DoAndReturn(
				func(
					_ context.Context,
					arg *repository.Environment,
				) (*repository.Environment, error) {
					if arg.ID == "" {
						t.Fatal("expected generated ID")
					}

					if arg.Version != InitialVersion {
						t.Fatalf(
							"expected version %q, got %q",
							InitialVersion,
							arg.Version,
						)
					}

					if arg.Name != params.Name {
						t.Fatalf(
							"expected name %q, got %q",
							params.Name,
							arg.Name,
						)
					}

					if arg.Description != params.Description {
						t.Fatalf(
							"expected description %q, got %q",
							params.Description,
							arg.Description,
						)
					}

					if !arg.CreatedAt.Equal(arg.UpdatedAt) {
						t.Fatalf(
							"CreatedAt %v != UpdatedAt %v",
							arg.CreatedAt,
							arg.UpdatedAt,
						)
					}

					var spec model.EnvironmentSpec
					if err := json.Unmarshal(arg.Spec, &spec); err != nil {
						t.Fatalf(
							"unmarshal repository spec: %v",
							err,
						)
					}

					return arg, nil
				},
			)

		environment, err := controller.CreateEnvironment(
			ctx,
			params,
		)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if environment.ID == "" {
			t.Fatal("expected generated ID")
		}

		if environment.Kind != model.ResourceKindEnvironment {
			t.Fatalf(
				"expected kind %q, got %q",
				model.ResourceKindEnvironment,
				environment.Kind,
			)
		}
	})

	t.Run("already exists", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		repo := mock.NewMockResourceRepository[repository.Environment](ctrl)

		controller := &Controller{
			environment: repo,
		}

		repo.EXPECT().
			Create(ctx, gomock.Any()).
			Return(nil, repository.ErrAlreadyExists)

		environment, err := controller.CreateEnvironment(
			ctx,
			params,
		)

		if environment != nil {
			t.Fatalf(
				"expected nil environment, got %+v",
				environment,
			)
		}

		if !errors.Is(
			err,
			coreerrors.ErrEntityAlreadyExists,
		) {
			t.Fatalf(
				"expected ErrEntityAlreadyExists, got %v",
				err,
			)
		}
	})

	t.Run("repository error", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		repo := mock.NewMockResourceRepository[repository.Environment](ctrl)

		controller := &Controller{
			environment: repo,
		}

		repo.EXPECT().
			Create(ctx, gomock.Any()).
			Return(nil, errCreateEnvironment)

		environment, err := controller.CreateEnvironment(
			ctx,
			params,
		)

		if environment != nil {
			t.Fatalf(
				"expected nil environment, got %+v",
				environment,
			)
		}

		if !errors.Is(err, errCreateEnvironment) {
			t.Fatalf(
				"expected %v, got %v",
				errCreateEnvironment,
				err,
			)
		}
	})
}

func TestControllerGetEnvironmentsPage(t *testing.T) {
	ctx := context.Background()

	ctrl := gomock.NewController(t)
	repo := mock.NewMockResourceRepository[repository.Environment](ctrl)

	controller, err := NewController(
		repo, nil, nil, nil, nil,
	)
	if err != nil {
		t.Fatalf("Unexpected NewController error: %q", err)
	}

	specJSON, err := json.Marshal(model.EnvironmentSpec{})
	if err != nil {
		t.Fatalf("marshal spec: %v", err)
	}

	createdAt := time.Date(
		2026, time.September, 20,
		10, 0, 0, 0,
		time.UTC,
	)

	items := []repository.Environment{
		{
			ID:          "7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3893",
			Version:     "1.0.0",
			Name:        "production",
			Description: "Production environment",
			Spec:        specJSON,
			CreatedAt:   createdAt,
			UpdatedAt:   createdAt.Add(time.Hour),
		},
		{
			ID:          "7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3892",
			Version:     "1.0.1",
			Name:        "development",
			Description: "Development environment",
			Spec:        specJSON,
			CreatedAt:   createdAt.Add(2 * time.Hour),
			UpdatedAt:   createdAt.Add(3 * time.Hour),
		},
		{
			ID:          "7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3891",
			Version:     "1.0.2",
			Name:        "alpha",
			Description: "Lookahead environment",
			Spec:        specJSON,
			CreatedAt:   createdAt.Add(4 * time.Hour),
			UpdatedAt:   createdAt.Add(5 * time.Hour),
		},
	}

	repo.EXPECT().
		List(ctx, gomock.Any()).
		DoAndReturn(
			func(
				_ context.Context,
				params *repository.ListParams,
			) ([]repository.Environment, error) {
				if params.Limit != 3 {
					t.Fatalf(
						"expected limit 3, got %d",
						params.Limit,
					)
				}

				if params.OrderBy != repository.OrderBy(
					model.OrderByName,
				) {
					t.Fatalf(
						"expected OrderBy %q, got %q",
						model.OrderByName,
						params.OrderBy,
					)
				}

				if params.OrderDirection != repository.OrderDirection(
					model.OrderDirectionDesc,
				) {
					t.Fatalf(
						"expected OrderDirection %q, got %q",
						model.OrderDirectionDesc,
						params.OrderDirection,
					)
				}

				if params.ID != "" {
					t.Fatalf(
						"expected empty ID anchor, got %q",
						params.ID,
					)
				}

				if params.ValueString != "" {
					t.Fatalf(
						"expected empty string anchor, got %q",
						params.ValueString,
					)
				}

				if !params.ValueTimestamp.IsZero() {
					t.Fatalf(
						"expected zero timestamp anchor, got %v",
						params.ValueTimestamp,
					)
				}

				return items, nil
			},
		)

	page, err := controller.GetEnvironmentsPage(
		ctx,
		&GetEnvironmentsPageParams{
			PagingParams: PagingParams{
				PageSize:       2,
				OrderBy:        model.OrderByName,
				OrderDirection: model.OrderDirectionDesc,
			},
		},
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if page == nil {
		t.Fatal("expected page")
	}

	if len(page.Items) != 2 {
		t.Fatalf(
			"expected 2 items, got %d",
			len(page.Items),
		)
	}

	expected := items[:2]

	for i := range expected {
		actual := page.Items[i]

		if actual.Kind != model.ResourceKindEnvironment {
			t.Fatalf(
				"item %d: expected kind %q, got %q",
				i,
				model.ResourceKindEnvironment,
				actual.Kind,
			)
		}

		if actual.ID != expected[i].ID {
			t.Fatalf(
				"item %d: expected ID %q, got %q",
				i,
				expected[i].ID,
				actual.ID,
			)
		}

		if actual.Version != expected[i].Version {
			t.Fatalf(
				"item %d: expected version %q, got %q",
				i,
				expected[i].Version,
				actual.Version,
			)
		}

		if actual.Name != expected[i].Name {
			t.Fatalf(
				"item %d: expected name %q, got %q",
				i,
				expected[i].Name,
				actual.Name,
			)
		}

		if actual.Description != expected[i].Description {
			t.Fatalf(
				"item %d: expected description %q, got %q",
				i,
				expected[i].Description,
				actual.Description,
			)
		}

		if !actual.CreatedAt.Equal(expected[i].CreatedAt) {
			t.Fatalf(
				"item %d: expected CreatedAt %v, got %v",
				i,
				expected[i].CreatedAt,
				actual.CreatedAt,
			)
		}

		if !actual.UpdatedAt.Equal(expected[i].UpdatedAt) {
			t.Fatalf(
				"item %d: expected UpdatedAt %v, got %v",
				i,
				expected[i].UpdatedAt,
				actual.UpdatedAt,
			)
		}
	}

	if page.Before != "" {
		t.Fatalf(
			"expected empty Before cursor, got %q",
			page.Before,
		)
	}

	if page.After == "" {
		t.Fatal("expected After cursor")
	}
}

var errCreateEnvironment = errors.New("create environment")
