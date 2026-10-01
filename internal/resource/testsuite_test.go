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

func TestGetTestSuiteCursorValue(t *testing.T) {
	createdAt := time.Date(
		2026, time.September, 21,
		8, 0, 0, 0,
		time.UTC,
	)
	updatedAt := createdAt.Add(time.Hour)

	testSuite := &model.TestSuite{
		ID:        "test-suite-001",
		Name:      "test suite",
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
			expected: testSuite.ID,
		},
		{
			name:     "name",
			orderBy:  model.OrderByName,
			expected: testSuite.Name,
		},
		{
			name:     "version",
			orderBy:  model.OrderByVersion,
			expected: testSuite.Version,
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
			actual := getTestSuiteCursorValue(
				testSuite,
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

func TestGetTestSuiteCursorValueUnsupported(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()

	getTestSuiteCursorValue(
		&model.TestSuite{},
		model.OrderBy("unsupported"),
	)
}

func TestNewTestSuiteFromRepository(t *testing.T) {
	createdAt := time.Date(
		2026, time.September, 21,
		8, 0, 0, 0,
		time.UTC,
	)
	updatedAt := createdAt.Add(time.Hour)

	spec := model.TestSuiteSpec{}

	specJSON, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("marshal spec: %v", err)
	}

	tests := []struct {
		name string

		input *repository.TestSuite

		expected      *model.TestSuite
		expectedError error
	}{
		{
			name: "nil",
		},
		{
			name: "success",
			input: &repository.TestSuite{
				ID:          "test-suite-001",
				Version:     "1.0.0",
				Name:        "test suite",
				Description: "test suite description",
				Spec:        specJSON,
				CreatedAt:   createdAt,
				UpdatedAt:   updatedAt,
			},
			expected: &model.TestSuite{
				Kind:        model.ResourceKindTestSuite,
				ID:          "test-suite-001",
				Version:     "1.0.0",
				Name:        "test suite",
				Description: "test suite description",
				Spec:        spec,
				CreatedAt:   createdAt,
				UpdatedAt:   updatedAt,
			},
		},
		{
			name: "invalid spec",
			input: &repository.TestSuite{
				ID:   "test-suite-001",
				Spec: []byte("{"),
			},
			expectedError: coreerrors.ErrCannotParseSpec,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := newTestSuiteFromRepository(
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
						"expected nil test suite, got %+v",
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

func TestNewTestSuiteSliceFromRepository(t *testing.T) {
	specJSON, err := json.Marshal(model.TestSuiteSpec{})
	if err != nil {
		t.Fatalf("marshal spec: %v", err)
	}

	items := []repository.TestSuite{
		{
			ID:   "test-suite-001",
			Spec: specJSON,
		},
		{
			ID:   "test-suite-002",
			Spec: specJSON,
		},
		{
			ID:   "test-suite-003",
			Spec: specJSON,
		},
	}

	invalid := repository.TestSuite{
		ID:   "test-suite-invalid",
		Spec: []byte("{"),
	}

	tests := []struct {
		name string

		input                    []repository.TestSuite
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
			input:       []repository.TestSuite{},
			expectedIDs: []string{},
		},
		{
			name:                     "forward",
			input:                    items,
			restorePresentationOrder: false,
			expectedIDs: []string{
				"test-suite-001",
				"test-suite-002",
				"test-suite-003",
			},
		},
		{
			name:                     "reverse",
			input:                    items,
			restorePresentationOrder: true,
			expectedIDs: []string{
				"test-suite-003",
				"test-suite-002",
				"test-suite-001",
			},
		},
		{
			name: "invalid first",
			input: []repository.TestSuite{
				invalid,
				items[0],
				items[1],
			},
			expectedError: coreerrors.ErrCannotParseSpec,
		},
		{
			name: "invalid middle",
			input: []repository.TestSuite{
				items[0],
				invalid,
				items[2],
			},
			expectedError: coreerrors.ErrCannotParseSpec,
		},
		{
			name: "invalid last while reversing",
			input: []repository.TestSuite{
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
			actual, err := newTestSuiteSliceFromRepository(
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
					"expected %d test suites, got %d",
					len(tt.expectedIDs),
					len(actual),
				)
			}

			for i := range actual {
				if actual[i].ID != tt.expectedIDs[i] {
					t.Fatalf(
						"test suite %d: expected ID %q, got %q",
						i,
						tt.expectedIDs[i],
						actual[i].ID,
					)
				}
			}
		})
	}
}

func TestControllerGetTestSuite(t *testing.T) {
	specJSON, err := json.Marshal(model.TestSuiteSpec{})
	if err != nil {
		t.Fatalf("marshal spec: %v", err)
	}

	const (
		ref = "test-suite-ref"
		id  = "test-suite-001"
	)

	repositoryTestSuite := &repository.TestSuite{
		ID:      id,
		Version: InitialVersion,
		Name:    "test suite",
		Spec:    specJSON,
	}

	errGetTestSuite := errors.New("get test suite error")

	tests := []struct {
		name string

		setup func(
			*mock.MockResourceRepository[repository.TestSuite],
		)

		expectedID    string
		expectedError error
	}{
		{
			name: "success by id",
			setup: func(
				repo *mock.MockResourceRepository[repository.TestSuite],
			) {
				repo.EXPECT().
					GetIDsByID(gomock.Any(), ref).
					Return([]string{id}, nil)

				repo.EXPECT().
					Get(gomock.Any(), id).
					Return(repositoryTestSuite, nil)
			},
			expectedID: id,
		},
		{
			name: "success by name after ambiguous id",
			setup: func(
				repo *mock.MockResourceRepository[repository.TestSuite],
			) {
				repo.EXPECT().
					GetIDsByID(gomock.Any(), ref).
					Return(
						[]string{
							"test-suite-001",
							"test-suite-002",
						},
						nil,
					)

				repo.EXPECT().
					GetIDsByName(gomock.Any(), ref).
					Return([]string{id}, nil)

				repo.EXPECT().
					Get(gomock.Any(), id).
					Return(repositoryTestSuite, nil)
			},
			expectedID: id,
		},
		{
			name: "reference not found",
			setup: func(
				repo *mock.MockResourceRepository[repository.TestSuite],
			) {
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
			setup: func(
				repo *mock.MockResourceRepository[repository.TestSuite],
			) {
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
			setup: func(
				repo *mock.MockResourceRepository[repository.TestSuite],
			) {
				repo.EXPECT().
					GetIDsByID(gomock.Any(), ref).
					Return([]string{id}, nil)

				repo.EXPECT().
					Get(gomock.Any(), id).
					Return(nil, errGetTestSuite)
			},
			expectedError: errGetTestSuite,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			repo := mock.NewMockResourceRepository[repository.TestSuite](
				ctrl,
			)

			tt.setup(repo)

			controller := &Controller{
				testSuite: repo,
			}

			actual, err := controller.GetTestSuite(
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
						"expected nil test suite, got %+v",
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
				t.Fatal("expected test suite")
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

func TestControllerDeleteTestSuite(t *testing.T) {
	specJSON, err := json.Marshal(model.TestSuiteSpec{})
	if err != nil {
		t.Fatalf("marshal spec: %v", err)
	}

	const (
		ref = "test-suite-ref"
		id  = "test-suite-001"
	)

	repositoryTestSuite := &repository.TestSuite{
		ID:      id,
		Version: InitialVersion,
		Name:    "test suite",
		Spec:    specJSON,
	}

	errDeleteTestSuite := errors.New("delete test suite error")

	tests := []struct {
		name string

		setup func(
			*mock.MockResourceRepository[repository.TestSuite],
		)

		expectedID    string
		expectedError error
	}{
		{
			name: "success",
			setup: func(
				repo *mock.MockResourceRepository[repository.TestSuite],
			) {
				repo.EXPECT().
					GetIDsByID(gomock.Any(), ref).
					Return([]string{id}, nil)

				repo.EXPECT().
					Delete(gomock.Any(), id).
					Return(repositoryTestSuite, nil)
			},
			expectedID: id,
		},
		{
			name: "not found",
			setup: func(
				repo *mock.MockResourceRepository[repository.TestSuite],
			) {
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
			setup: func(
				repo *mock.MockResourceRepository[repository.TestSuite],
			) {
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
			setup: func(
				repo *mock.MockResourceRepository[repository.TestSuite],
			) {
				repo.EXPECT().
					GetIDsByID(gomock.Any(), ref).
					Return([]string{id}, nil)

				repo.EXPECT().
					Delete(gomock.Any(), id).
					Return(nil, errDeleteTestSuite)
			},
			expectedError: errDeleteTestSuite,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			repo := mock.NewMockResourceRepository[repository.TestSuite](
				ctrl,
			)

			tt.setup(repo)

			controller := &Controller{
				testSuite: repo,
			}

			actual, err := controller.DeleteTestSuite(
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
						"expected nil test suite, got %+v",
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
				t.Fatal("expected test suite")
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

func TestControllerCreateTestSuite(t *testing.T) {
	errCreateTestSuite := errors.New(
		"create test suite error",
	)

	tests := []struct {
		name string

		setup func(
			*mock.MockResourceRepository[repository.TestSuite],
		)

		expectedError error
	}{
		{
			name: "success",
			setup: func(
				repo *mock.MockResourceRepository[repository.TestSuite],
			) {
				repo.EXPECT().
					Create(
						gomock.Any(),
						gomock.Any(),
					).
					DoAndReturn(
						func(
							_ context.Context,
							input *repository.TestSuite,
						) (*repository.TestSuite, error) {
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

							if input.Name != "test suite" {
								t.Fatalf(
									"expected name %q, got %q",
									"test suite",
									input.Name,
								)
							}

							if input.Description != "test suite description" {
								t.Fatalf(
									"expected description %q, got %q",
									"test suite description",
									input.Description,
								)
							}

							if input.CreatedAt.IsZero() {
								t.Fatal(
									"expected created at",
								)
							}

							if input.UpdatedAt.IsZero() {
								t.Fatal(
									"expected updated at",
								)
							}

							if !input.CreatedAt.Equal(
								input.UpdatedAt,
							) {
								t.Fatalf(
									"expected equal timestamps, got %v and %v",
									input.CreatedAt,
									input.UpdatedAt,
								)
							}

							var spec model.TestSuiteSpec
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
			setup: func(
				repo *mock.MockResourceRepository[repository.TestSuite],
			) {
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
			setup: func(
				repo *mock.MockResourceRepository[repository.TestSuite],
			) {
				repo.EXPECT().
					Create(
						gomock.Any(),
						gomock.Any(),
					).
					Return(
						nil,
						errCreateTestSuite,
					)
			},
			expectedError: errCreateTestSuite,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			repo := mock.NewMockResourceRepository[repository.TestSuite](
				ctrl,
			)

			tt.setup(repo)

			controller := &Controller{
				testSuite: repo,
			}

			actual, err := controller.CreateTestSuite(
				t.Context(),
				&CreateTestSuiteParams{
					Name:        "test suite",
					Description: "test suite description",
					Spec:        model.TestSuiteSpec{},
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
						"expected nil test suite, got %+v",
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
				t.Fatal("expected test suite")
			}

			if actual.Kind != model.ResourceKindTestSuite {
				t.Fatalf(
					"expected kind %q, got %q",
					model.ResourceKindTestSuite,
					actual.Kind,
				)
			}

			if actual.Name != "test suite" {
				t.Fatalf(
					"expected name %q, got %q",
					"test suite",
					actual.Name,
				)
			}

			if actual.Description != "test suite description" {
				t.Fatalf(
					"expected description %q, got %q",
					"test suite description",
					actual.Description,
				)
			}
		})
	}
}

func TestControllerGetTestSuitesPage(t *testing.T) {
	ctrl := gomock.NewController(t)

	repo := mock.NewMockResourceRepository[repository.TestSuite](
		ctrl,
	)

	controller, err := NewController(
		nil,
		nil,
		repo,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf(
			"new controller: %v",
			err,
		)
	}

	specJSON, err := json.Marshal(
		model.TestSuiteSpec{},
	)
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

	items := []repository.TestSuite{
		{
			ID:          "0000000000000000000000000000000000000000000000000000000000000001",
			Version:     InitialVersion,
			Name:        "test-suite-1",
			Description: "first test suite",
			Spec:        specJSON,
			CreatedAt:   createdAt,
			UpdatedAt:   updatedAt,
		},
		{
			ID:          "0000000000000000000000000000000000000000000000000000000000000002",
			Version:     InitialVersion,
			Name:        "test-suite-2",
			Description: "second test suite",
			Spec:        specJSON,
			CreatedAt:   createdAt.Add(time.Minute),
			UpdatedAt:   updatedAt.Add(time.Minute),
		},
		{
			ID:          "0000000000000000000000000000000000000000000000000000000000000003",
			Version:     InitialVersion,
			Name:        "test-suite-3",
			Description: "third test suite",
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
			) ([]repository.TestSuite, error) {
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
						"expected order by %q, got %q",
						model.OrderByName,
						params.OrderBy,
					)
				}

				if params.OrderDirection != repository.OrderDirection(
					model.OrderDirectionDesc,
				) {
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

	page, err := controller.GetTestSuitesPage(
		t.Context(),
		&GetTestSuitesPageParams{
			PagingParams: PagingParams{
				PageSize:       2,
				OrderBy:        model.OrderByName,
				OrderDirection: model.OrderDirectionDesc,
			},
		},
	)
	if err != nil {
		t.Fatalf(
			"get test suites page: %v",
			err,
		)
	}

	if page == nil {
		t.Fatal("expected page")
	}

	if len(page.Items) != 2 {
		t.Fatalf(
			"expected 2 test suites, got %d",
			len(page.Items),
		)
	}

	for i := 0; i < 2; i++ {
		actual := page.Items[i]
		expected := items[i]

		if actual.Kind != model.ResourceKindTestSuite {
			t.Fatalf(
				"test suite %d: expected kind %q, got %q",
				i,
				model.ResourceKindTestSuite,
				actual.Kind,
			)
		}

		if actual.ID != expected.ID {
			t.Fatalf(
				"test suite %d: expected ID %q, got %q",
				i,
				expected.ID,
				actual.ID,
			)
		}

		if actual.Version != expected.Version {
			t.Fatalf(
				"test suite %d: expected version %q, got %q",
				i,
				expected.Version,
				actual.Version,
			)
		}

		if actual.Name != expected.Name {
			t.Fatalf(
				"test suite %d: expected name %q, got %q",
				i,
				expected.Name,
				actual.Name,
			)
		}

		if actual.Description != expected.Description {
			t.Fatalf(
				"test suite %d: expected description %q, got %q",
				i,
				expected.Description,
				actual.Description,
			)
		}

		if !actual.CreatedAt.Equal(expected.CreatedAt) {
			t.Fatalf(
				"test suite %d: expected created at %v, got %v",
				i,
				expected.CreatedAt,
				actual.CreatedAt,
			)
		}

		if !actual.UpdatedAt.Equal(expected.UpdatedAt) {
			t.Fatalf(
				"test suite %d: expected updated at %v, got %v",
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
