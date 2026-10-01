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

func TestGetTestCursorValue(t *testing.T) {
	createdAt := time.Date(
		2026, time.September, 21,
		8, 0, 0, 0,
		time.UTC,
	)
	updatedAt := createdAt.Add(time.Hour)

	test := &model.Test{
		ID:        "test-001",
		Name:      "test name",
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
			expected: test.ID,
		},
		{
			name:     "name",
			orderBy:  model.OrderByName,
			expected: test.Name,
		},
		{
			name:     "version",
			orderBy:  model.OrderByVersion,
			expected: test.Version,
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
			actual := getTestCursorValue(
				test,
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

func TestGetTestCursorValueUnsupported(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()

	getTestCursorValue(
		&model.Test{},
		model.OrderBy("unsupported"),
	)
}

func TestNewTestFromRepository(t *testing.T) {
	createdAt := time.Date(
		2026, time.September, 21,
		8, 0, 0, 0,
		time.UTC,
	)
	updatedAt := createdAt.Add(time.Hour)

	spec := model.TestSpec{}

	specJSON, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("marshal spec: %v", err)
	}

	tests := []struct {
		name string

		input *repository.Test

		expected      *model.Test
		expectedError error
	}{
		{
			name: "nil",
		},
		{
			name: "success",
			input: &repository.Test{
				ID:          "test-001",
				Version:     "1.0.0",
				Name:        "test name",
				Description: "test description",
				Spec:        specJSON,
				CreatedAt:   createdAt,
				UpdatedAt:   updatedAt,
			},
			expected: &model.Test{
				Kind:        model.ResourceKindTest,
				ID:          "test-001",
				Version:     "1.0.0",
				Name:        "test name",
				Description: "test description",
				Spec:        spec,
				CreatedAt:   createdAt,
				UpdatedAt:   updatedAt,
			},
		},
		{
			name: "invalid spec",
			input: &repository.Test{
				ID:   "test-001",
				Spec: []byte("{"),
			},
			expectedError: coreerrors.ErrCannotParseSpec,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := newTestFromRepository(
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
						"expected nil test, got %+v",
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

func TestNewTestSliceFromRepository(t *testing.T) {
	specJSON, err := json.Marshal(model.TestSpec{})
	if err != nil {
		t.Fatalf("marshal spec: %v", err)
	}

	items := []repository.Test{
		{
			ID:   "test-001",
			Spec: specJSON,
		},
		{
			ID:   "test-002",
			Spec: specJSON,
		},
		{
			ID:   "test-003",
			Spec: specJSON,
		},
	}

	invalid := repository.Test{
		ID:   "test-invalid",
		Spec: []byte("{"),
	}

	tests := []struct {
		name string

		input                    []repository.Test
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
			input:       []repository.Test{},
			expectedIDs: []string{},
		},
		{
			name:                     "forward",
			input:                    items,
			restorePresentationOrder: false,
			expectedIDs: []string{
				"test-001",
				"test-002",
				"test-003",
			},
		},
		{
			name:                     "reverse",
			input:                    items,
			restorePresentationOrder: true,
			expectedIDs: []string{
				"test-003",
				"test-002",
				"test-001",
			},
		},
		{
			name: "invalid first",
			input: []repository.Test{
				invalid,
				items[0],
				items[1],
			},
			expectedError: coreerrors.ErrCannotParseSpec,
		},
		{
			name: "invalid middle",
			input: []repository.Test{
				items[0],
				invalid,
				items[2],
			},
			expectedError: coreerrors.ErrCannotParseSpec,
		},
		{
			name: "invalid last while reversing",
			input: []repository.Test{
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
			actual, err := newTestSliceFromRepository(
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
					"expected %d tests, got %d",
					len(tt.expectedIDs),
					len(actual),
				)
			}

			for i := range actual {
				if actual[i].ID != tt.expectedIDs[i] {
					t.Fatalf(
						"test %d: expected ID %q, got %q",
						i,
						tt.expectedIDs[i],
						actual[i].ID,
					)
				}
			}
		})
	}
}

func TestControllerGetTest(t *testing.T) {
	specJSON, err := json.Marshal(model.TestSpec{})
	if err != nil {
		t.Fatalf("marshal spec: %v", err)
	}

	const (
		ref = "test-ref"
		id  = "test-001"
	)

	repositoryTest := &repository.Test{
		ID:      id,
		Version: InitialVersion,
		Name:    "test name",
		Spec:    specJSON,
	}

	errGetTest := errors.New("get test error")

	tests := []struct {
		name string

		setup func(*mock.MockTestRepository)

		expectedID    string
		expectedError error
	}{
		{
			name: "success by id",
			setup: func(repo *mock.MockTestRepository) {
				repo.EXPECT().
					GetIDsByID(gomock.Any(), ref).
					Return([]string{id}, nil)

				repo.EXPECT().
					Get(gomock.Any(), id).
					Return(repositoryTest, nil)
			},
			expectedID: id,
		},
		{
			name: "success by name after ambiguous id",
			setup: func(repo *mock.MockTestRepository) {
				repo.EXPECT().
					GetIDsByID(gomock.Any(), ref).
					Return(
						[]string{
							"test-001",
							"test-002",
						},
						nil,
					)

				repo.EXPECT().
					GetIDsByName(gomock.Any(), ref).
					Return([]string{id}, nil)

				repo.EXPECT().
					Get(gomock.Any(), id).
					Return(repositoryTest, nil)
			},
			expectedID: id,
		},
		{
			name: "reference not found",
			setup: func(repo *mock.MockTestRepository) {
				repo.EXPECT().
					GetIDsByID(gomock.Any(), ref).
					Return([]string{}, nil)

				repo.EXPECT().
					GetIDsByName(gomock.Any(), ref).
					Return([]string{}, nil)
			},
			expectedError: coreerrors.ErrEntityNotFound,
		},
		{
			name: "repository not found",
			setup: func(repo *mock.MockTestRepository) {
				repo.EXPECT().
					GetIDsByID(gomock.Any(), ref).
					Return([]string{id}, nil)

				repo.EXPECT().
					Get(gomock.Any(), id).
					Return(nil, repository.ErrNotFound)
			},
			expectedError: coreerrors.ErrEntityNotFound,
		},
		{
			name: "repository error",
			setup: func(repo *mock.MockTestRepository) {
				repo.EXPECT().
					GetIDsByID(gomock.Any(), ref).
					Return([]string{id}, nil)

				repo.EXPECT().
					Get(gomock.Any(), id).
					Return(nil, errGetTest)
			},
			expectedError: errGetTest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mock.NewMockTestRepository(ctrl)

			tt.setup(repo)

			controller := &Controller{
				test: repo,
			}

			actual, err := controller.GetTest(
				t.Context(),
				ref,
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
						"expected nil test, got %+v",
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

			if actual == nil {
				t.Fatal("expected test")
			}

			if actual.ID != tt.expectedID {
				t.Fatalf(
					"expected ID %q, got %q",
					tt.expectedID,
					actual.ID,
				)
			}
		})
	}
}

func TestControllerDeleteTest(t *testing.T) {
	specJSON, err := json.Marshal(model.TestSpec{})
	if err != nil {
		t.Fatalf("marshal spec: %v", err)
	}

	const (
		ref = "test-ref"
		id  = "test-001"
	)

	repositoryTest := &repository.Test{
		ID:      id,
		Version: InitialVersion,
		Name:    "test name",
		Spec:    specJSON,
	}

	errDeleteTest := errors.New("delete test error")

	tests := []struct {
		name string

		setup func(*mock.MockTestRepository)

		expectedID    string
		expectedError error
	}{
		{
			name: "success",
			setup: func(repo *mock.MockTestRepository) {
				repo.EXPECT().
					GetIDsByID(gomock.Any(), ref).
					Return([]string{id}, nil)

				repo.EXPECT().
					Delete(gomock.Any(), id).
					Return(repositoryTest, nil)
			},
			expectedID: id,
		},
		{
			name: "not found",
			setup: func(repo *mock.MockTestRepository) {
				repo.EXPECT().
					GetIDsByID(gomock.Any(), ref).
					Return([]string{id}, nil)

				repo.EXPECT().
					Delete(gomock.Any(), id).
					Return(nil, repository.ErrNotFound)
			},
			expectedError: coreerrors.ErrEntityNotFound,
		},
		{
			name: "conflict",
			setup: func(repo *mock.MockTestRepository) {
				repo.EXPECT().
					GetIDsByID(gomock.Any(), ref).
					Return([]string{id}, nil)

				repo.EXPECT().
					Delete(gomock.Any(), id).
					Return(nil, repository.ErrConflict)
			},
			expectedError: coreerrors.ErrEntityConflict,
		},
		{
			name: "repository error",
			setup: func(repo *mock.MockTestRepository) {
				repo.EXPECT().
					GetIDsByID(gomock.Any(), ref).
					Return([]string{id}, nil)

				repo.EXPECT().
					Delete(gomock.Any(), id).
					Return(nil, errDeleteTest)
			},
			expectedError: errDeleteTest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mock.NewMockTestRepository(ctrl)

			tt.setup(repo)

			controller := &Controller{
				test: repo,
			}

			actual, err := controller.DeleteTest(
				t.Context(),
				ref,
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
						"expected nil test, got %+v",
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

			if actual == nil {
				t.Fatal("expected test")
			}

			if actual.ID != tt.expectedID {
				t.Fatalf(
					"expected ID %q, got %q",
					tt.expectedID,
					actual.ID,
				)
			}
		})
	}
}

func TestControllerCreateTest(t *testing.T) {
	errCreateTest := errors.New("create test error")

	tests := []struct {
		name string

		setup func(*mock.MockTestRepository)

		expectedError error
	}{
		{
			name: "success",
			setup: func(repo *mock.MockTestRepository) {
				repo.EXPECT().
					Create(
						gomock.Any(),
						gomock.Any(),
					).
					DoAndReturn(
						func(
							_ context.Context,
							input *repository.Test,
						) (*repository.Test, error) {
							if input.ID == "" {
								t.Fatal("expected generated ID")
							}

							if input.Version != InitialVersion {
								t.Fatalf(
									"expected version %q, got %q",
									InitialVersion,
									input.Version,
								)
							}

							if input.Name != "test name" {
								t.Fatalf(
									"expected name %q, got %q",
									"test name",
									input.Name,
								)
							}

							if input.Description != "test description" {
								t.Fatalf(
									"expected description %q, got %q",
									"test description",
									input.Description,
								)
							}

							if input.CreatedAt.IsZero() {
								t.Fatal("expected created at")
							}

							if input.UpdatedAt.IsZero() {
								t.Fatal("expected updated at")
							}

							if !input.CreatedAt.Equal(input.UpdatedAt) {
								t.Fatalf(
									"expected equal timestamps, got %v and %v",
									input.CreatedAt,
									input.UpdatedAt,
								)
							}

							var spec model.TestSpec
							if err := json.Unmarshal(
								input.Spec,
								&spec,
							); err != nil {
								t.Fatalf(
									"unmarshal spec: %v",
									err,
								)
							}

							return input, nil
						},
					)
			},
		},
		{
			name: "already exists",
			setup: func(repo *mock.MockTestRepository) {
				repo.EXPECT().
					Create(
						gomock.Any(),
						gomock.Any(),
					).
					Return(
						nil,
						repository.ErrAlreadyExists,
					)
			},
			expectedError: coreerrors.ErrEntityAlreadyExists,
		},
		{
			name: "repository error",
			setup: func(repo *mock.MockTestRepository) {
				repo.EXPECT().
					Create(
						gomock.Any(),
						gomock.Any(),
					).
					Return(
						nil,
						errCreateTest,
					)
			},
			expectedError: errCreateTest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mock.NewMockTestRepository(ctrl)

			tt.setup(repo)

			controller := &Controller{
				test: repo,
			}

			actual, err := controller.CreateTest(
				t.Context(),
				&CreateTestParams{
					Name:        "test name",
					Description: "test description",
					Spec:        model.TestSpec{},
				},
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
						"expected nil test, got %+v",
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

			if actual == nil {
				t.Fatal("expected test")
			}

			if actual.Kind != model.ResourceKindTest {
				t.Fatalf(
					"expected kind %q, got %q",
					model.ResourceKindTest,
					actual.Kind,
				)
			}

			if actual.Name != "test name" {
				t.Fatalf(
					"expected name %q, got %q",
					"test name",
					actual.Name,
				)
			}

			if actual.Description != "test description" {
				t.Fatalf(
					"expected description %q, got %q",
					"test description",
					actual.Description,
				)
			}
		})
	}
}

func TestControllerGetTestsByTag(t *testing.T) {
	specJSON, err := json.Marshal(model.TestSpec{})
	if err != nil {
		t.Fatalf("marshal spec: %v", err)
	}

	errGetByTag := errors.New("get by tag error")

	tests := []struct {
		name string

		repositoryTests []repository.Test
		repositoryError error

		expectedIDs   []string
		expectedError error
	}{
		{
			name: "success",
			repositoryTests: []repository.Test{
				{
					ID:          "test-001",
					Version:     InitialVersion,
					Name:        "test one",
					Description: "first test",
					Spec:        specJSON,
				},
				{
					ID:          "test-002",
					Version:     InitialVersion,
					Name:        "test two",
					Description: "second test",
					Spec:        specJSON,
				},
			},
			expectedIDs: []string{
				"test-001",
				"test-002",
			},
		},
		{
			name:            "empty",
			repositoryTests: []repository.Test{},
			expectedIDs:     []string{},
		},
		{
			name:            "repository error",
			repositoryError: errGetByTag,
			expectedError:   errGetByTag,
		},
		{
			name: "conversion error",
			repositoryTests: []repository.Test{
				{
					ID:   "test-001",
					Spec: []byte("{"),
				},
			},
			expectedError: coreerrors.ErrCannotParseSpec,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mock.NewMockTestRepository(ctrl)

			repo.EXPECT().
				GetByTag(
					gomock.Any(),
					"smoke",
				).
				Return(
					tt.repositoryTests,
					tt.repositoryError,
				)

			controller := &Controller{
				test: repo,
			}

			actual, err := controller.GetTestsByTag(
				t.Context(),
				"smoke",
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
						"expected nil tests, got %+v",
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

			if len(actual) != len(tt.expectedIDs) {
				t.Fatalf(
					"expected %d tests, got %d",
					len(tt.expectedIDs),
					len(actual),
				)
			}

			for i := range actual {
				if actual[i].ID != tt.expectedIDs[i] {
					t.Fatalf(
						"test %d: expected ID %q, got %q",
						i,
						tt.expectedIDs[i],
						actual[i].ID,
					)
				}
			}
		})
	}
}

func TestControllerGetTestsPage(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mock.NewMockTestRepository(ctrl)

	controller, err := NewController(
		nil,
		repo,
		nil,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf(
			"new controller: %v",
			err,
		)
	}

	specJSON, err := json.Marshal(model.TestSpec{})
	if err != nil {
		t.Fatalf(
			"marshal spec: %v",
			err,
		)
	}

	createdAt := time.Date(
		2026, time.September, 21,
		8, 0, 0, 0,
		time.UTC,
	)

	updatedAt := createdAt.Add(time.Hour)

	items := []repository.Test{
		{
			ID:          "0000000000000000000000000000000000000000000000000000000000000001",
			Version:     InitialVersion,
			Name:        "test-1",
			Description: "first test",
			Spec:        specJSON,
			CreatedAt:   createdAt,
			UpdatedAt:   updatedAt,
		},
		{
			ID:          "0000000000000000000000000000000000000000000000000000000000000002",
			Version:     InitialVersion,
			Name:        "test-2",
			Description: "second test",
			Spec:        specJSON,
			CreatedAt:   createdAt.Add(time.Minute),
			UpdatedAt:   updatedAt.Add(time.Minute),
		},
		{
			ID:          "0000000000000000000000000000000000000000000000000000000000000003",
			Version:     InitialVersion,
			Name:        "test-3",
			Description: "third test",
			Spec:        specJSON,
			CreatedAt:   createdAt.Add(2 * time.Minute),
			UpdatedAt:   updatedAt.Add(2 * time.Minute),
		},
	}

	repo.EXPECT().
		List(
			gomock.Any(),
			gomock.Any(),
		).
		DoAndReturn(
			func(
				_ context.Context,
				params *repository.ListParams,
			) ([]repository.Test, error) {
				if params.Limit != 3 {
					t.Fatalf(
						"expected limit 3, got %d",
						params.Limit,
					)
				}

				if params.OrderBy != repository.OrderBy(model.OrderByName) {
					t.Fatalf(
						"expected order by %q, got %q",
						model.OrderByName,
						params.OrderBy,
					)
				}

				if params.OrderDirection != repository.OrderDirection(model.OrderDirectionDesc) {
					t.Fatalf(
						"expected direction %q, got %q",
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

	page, err := controller.GetTestsPage(
		t.Context(),
		&GetTestsPageParams{
			PagingParams: PagingParams{
				PageSize:       2,
				OrderBy:        model.OrderByName,
				OrderDirection: model.OrderDirectionDesc,
			},
		},
	)
	if err != nil {
		t.Fatalf(
			"get tests page: %v",
			err,
		)
	}

	if page == nil {
		t.Fatal("expected page")
	}

	if len(page.Items) != 2 {
		t.Fatalf(
			"expected 2 tests, got %d",
			len(page.Items),
		)
	}

	for i := 0; i < 2; i++ {
		actual := page.Items[i]
		expected := items[i]

		if actual.Kind != model.ResourceKindTest {
			t.Fatalf(
				"test %d: expected kind %q, got %q",
				i,
				model.ResourceKindTest,
				actual.Kind,
			)
		}

		if actual.ID != expected.ID {
			t.Fatalf(
				"test %d: expected ID %q, got %q",
				i,
				expected.ID,
				actual.ID,
			)
		}

		if actual.Version != expected.Version {
			t.Fatalf(
				"test %d: expected version %q, got %q",
				i,
				expected.Version,
				actual.Version,
			)
		}

		if actual.Name != expected.Name {
			t.Fatalf(
				"test %d: expected name %q, got %q",
				i,
				expected.Name,
				actual.Name,
			)
		}

		if actual.Description != expected.Description {
			t.Fatalf(
				"test %d: expected description %q, got %q",
				i,
				expected.Description,
				actual.Description,
			)
		}

		if !actual.CreatedAt.Equal(expected.CreatedAt) {
			t.Fatalf(
				"test %d: expected created at %v, got %v",
				i,
				expected.CreatedAt,
				actual.CreatedAt,
			)
		}

		if !actual.UpdatedAt.Equal(expected.UpdatedAt) {
			t.Fatalf(
				"test %d: expected updated at %v, got %v",
				i,
				expected.UpdatedAt,
				actual.UpdatedAt,
			)
		}
	}

	if page.Before != "" {
		t.Fatalf(
			"expected empty before cursor, got %q",
			page.Before,
		)
	}

	if page.After == "" {
		t.Fatal("expected after cursor")
	}
}
