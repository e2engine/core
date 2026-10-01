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

func TestGetTestExecutionCursorValue(t *testing.T) {
	startedAt := time.Date(
		2026, time.September, 21,
		8, 0, 0, 0,
		time.UTC,
	)
	finishedAt := startedAt.Add(time.Minute)

	testExecution := &model.TestExecution{
		ID:         "test-execution-001",
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
		Status:     model.ExecutionStatus("finished"),
	}

	tests := []struct {
		name     string
		orderBy  model.OrderBy
		expected string
	}{
		{
			name:     "id",
			orderBy:  model.OrderByID,
			expected: testExecution.ID,
		},
		{
			name:     "started at",
			orderBy:  model.OrderByStartedAt,
			expected: startedAt.Format(time.RFC3339Nano),
		},
		{
			name:     "finished at",
			orderBy:  model.OrderByFinishedAt,
			expected: finishedAt.Format(time.RFC3339Nano),
		},
		{
			name:     "status",
			orderBy:  model.OrderByStatus,
			expected: string(testExecution.Status),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := getTestExecutionCursorValue(
				testExecution,
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

func TestGetTestExecutionCursorValueUnsupported(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()

	getTestExecutionCursorValue(
		&model.TestExecution{},
		model.OrderBy("unsupported"),
	)
}

func TestNewTestExecutionFromRepository(t *testing.T) {
	startedAt := time.Date(
		2026, time.September, 21,
		8, 0, 0, 0,
		time.UTC,
	)
	finishedAt := startedAt.Add(time.Minute)

	summary := model.TestExecutionSummary{}

	summaryJSON, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("marshal summary: %v", err)
	}

	tests := []struct {
		name string

		input *repository.TestExecution

		expected      *model.TestExecution
		expectedError error
	}{
		{
			name: "nil",
		},
		{
			name: "without summary",
			input: &repository.TestExecution{
				ID:              "test-execution-001",
				StartedAt:       startedAt,
				FinishedAt:      finishedAt,
				Status:          "finished",
				EnvironmentID:   "environment-001",
				EnvironmentName: "environment",
				TestID:          "test-001",
				TestName:        "test",
			},
			expected: &model.TestExecution{
				ID:              "test-execution-001",
				StartedAt:       startedAt,
				FinishedAt:      finishedAt,
				Status:          model.ExecutionStatus("finished"),
				EnvironmentID:   "environment-001",
				EnvironmentName: "environment",
				TestID:          "test-001",
				TestName:        "test",
			},
		},
		{
			name: "with summary",
			input: &repository.TestExecution{
				ID:              "test-execution-001",
				StartedAt:       startedAt,
				FinishedAt:      finishedAt,
				Status:          "finished",
				EnvironmentID:   "environment-001",
				EnvironmentName: "environment",
				TestID:          "test-001",
				TestName:        "test",
				Summary:         summaryJSON,
			},
			expected: &model.TestExecution{
				ID:              "test-execution-001",
				StartedAt:       startedAt,
				FinishedAt:      finishedAt,
				Status:          model.ExecutionStatus("finished"),
				EnvironmentID:   "environment-001",
				EnvironmentName: "environment",
				TestID:          "test-001",
				TestName:        "test",
				Summary:         &summary,
			},
		},
		{
			name: "invalid summary",
			input: &repository.TestExecution{
				ID:      "test-execution-001",
				Summary: []byte("{"),
			},
			expectedError: coreerrors.ErrCannotParseTestExecutionSummary,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := newTestExecutionFromRepository(
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
						"expected nil test execution, got %+v",
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

func TestNewTestExecutionSliceFromRepository(t *testing.T) {
	summaryJSON, err := json.Marshal(
		model.TestExecutionSummary{},
	)
	if err != nil {
		t.Fatalf("marshal summary: %v", err)
	}

	items := []repository.TestExecution{
		{
			ID:      "test-execution-001",
			Summary: summaryJSON,
		},
		{
			ID:      "test-execution-002",
			Summary: summaryJSON,
		},
		{
			ID:      "test-execution-003",
			Summary: summaryJSON,
		},
	}

	invalid := repository.TestExecution{
		ID:      "test-execution-invalid",
		Summary: []byte("{"),
	}

	tests := []struct {
		name string

		input                    []repository.TestExecution
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
			input:       []repository.TestExecution{},
			expectedIDs: []string{},
		},
		{
			name:                     "forward",
			input:                    items,
			restorePresentationOrder: false,
			expectedIDs: []string{
				"test-execution-001",
				"test-execution-002",
				"test-execution-003",
			},
		},
		{
			name:                     "reverse",
			input:                    items,
			restorePresentationOrder: true,
			expectedIDs: []string{
				"test-execution-003",
				"test-execution-002",
				"test-execution-001",
			},
		},
		{
			name: "conversion error",
			input: []repository.TestExecution{
				items[0],
				invalid,
				items[2],
			},
			expectedError: coreerrors.ErrCannotParseTestExecutionSummary,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := newTestExecutionSliceFromRepository(
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
					"expected %d test executions, got %d",
					len(tt.expectedIDs),
					len(actual),
				)
			}

			for i := range actual {
				if actual[i].ID != tt.expectedIDs[i] {
					t.Fatalf(
						"test execution %d: expected ID %q, got %q",
						i,
						tt.expectedIDs[i],
						actual[i].ID,
					)
				}
			}
		})
	}
}

func TestControllerGetTestExecution(t *testing.T) {
	const (
		ref = "execution-ref"
		id  = "test-execution-001"
	)

	repositoryTestExecution := &repository.TestExecution{
		ID:     id,
		Status: "finished",
	}

	errGetTestExecution := errors.New(
		"get test execution error",
	)

	tests := []struct {
		name string

		setup func(
			*mock.MockTestExecutionRepository,
		)

		expectedID    string
		expectedError error
	}{
		{
			name: "success",
			setup: func(
				repo *mock.MockTestExecutionRepository,
			) {
				repo.EXPECT().
					GetIDsByID(gomock.Any(), ref).
					Return([]string{id}, nil)

				repo.EXPECT().
					Get(gomock.Any(), id).
					Return(repositoryTestExecution, nil)
			},
			expectedID: id,
		},
		{
			name: "reference not found",
			setup: func(
				repo *mock.MockTestExecutionRepository,
			) {
				repo.EXPECT().
					GetIDsByID(gomock.Any(), ref).
					Return([]string{}, nil)
			},
			expectedError: coreerrors.ErrEntityNotFound,
		},
		{
			name: "reference ambiguous",
			setup: func(
				repo *mock.MockTestExecutionRepository,
			) {
				repo.EXPECT().
					GetIDsByID(gomock.Any(), ref).
					Return(
						[]string{
							"test-execution-001",
							"test-execution-002",
						},
						nil,
					)
			},
			expectedError: coreerrors.ErrEntityPrefixAmbiguous,
		},
		{
			name: "repository not found",
			setup: func(
				repo *mock.MockTestExecutionRepository,
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
				repo *mock.MockTestExecutionRepository,
			) {
				repo.EXPECT().
					GetIDsByID(gomock.Any(), ref).
					Return([]string{id}, nil)

				repo.EXPECT().
					Get(gomock.Any(), id).
					Return(nil, errGetTestExecution)
			},
			expectedError: errGetTestExecution,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			repo := mock.NewMockTestExecutionRepository(
				ctrl,
			)

			tt.setup(repo)

			controller := &Controller{
				testExecution: repo,
			}

			actual, err := controller.GetTestExecution(
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
						"expected nil test execution, got %+v",
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
				t.Fatal("expected test execution")
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

func TestControllerGetTestExecutionStatus(t *testing.T) {
	const (
		ref = "execution-ref"
		id  = "test-execution-001"
	)

	errGetTestExecutionStatus := errors.New(
		"get test execution status error",
	)

	tests := []struct {
		name string

		setup func(
			*mock.MockTestExecutionRepository,
		)

		expectedStatus model.ExecutionStatus
		expectedError  error
	}{
		{
			name: "success",
			setup: func(
				repo *mock.MockTestExecutionRepository,
			) {
				repo.EXPECT().
					GetIDsByID(gomock.Any(), ref).
					Return([]string{id}, nil)

				repo.EXPECT().
					GetStatus(gomock.Any(), id).
					Return(string(model.ExecutionStatusPassed), nil)
			},
			expectedStatus: model.ExecutionStatusPassed,
		},
		{
			name: "reference not found",
			setup: func(
				repo *mock.MockTestExecutionRepository,
			) {
				repo.EXPECT().
					GetIDsByID(gomock.Any(), ref).
					Return([]string{}, nil)
			},
			expectedError: coreerrors.ErrEntityNotFound,
		},
		{
			name: "reference ambiguous",
			setup: func(
				repo *mock.MockTestExecutionRepository,
			) {
				repo.EXPECT().
					GetIDsByID(gomock.Any(), ref).
					Return(
						[]string{
							"test-execution-001",
							"test-execution-002",
						},
						nil,
					)
			},
			expectedError: coreerrors.ErrEntityPrefixAmbiguous,
		},
		{
			name: "repository not found",
			setup: func(
				repo *mock.MockTestExecutionRepository,
			) {
				repo.EXPECT().
					GetIDsByID(gomock.Any(), ref).
					Return([]string{id}, nil)

				repo.EXPECT().
					GetStatus(gomock.Any(), id).
					Return("", repository.ErrNotFound)
			},
			expectedError: coreerrors.ErrEntityNotFound,
		},
		{
			name: "repository error",
			setup: func(
				repo *mock.MockTestExecutionRepository,
			) {
				repo.EXPECT().
					GetIDsByID(gomock.Any(), ref).
					Return([]string{id}, nil)

				repo.EXPECT().
					GetStatus(gomock.Any(), id).
					Return("", errGetTestExecutionStatus)
			},
			expectedError: errGetTestExecutionStatus,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			repo := mock.NewMockTestExecutionRepository(
				ctrl,
			)

			tt.setup(repo)

			controller := &Controller{
				testExecution: repo,
			}

			actual, err := controller.GetTestExecutionStatus(
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

				if actual != "" {
					t.Fatalf(
						"expected empty status, got %q",
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

			if actual != tt.expectedStatus {
				t.Fatalf(
					"expected status %q, got %q",
					tt.expectedStatus,
					actual,
				)
			}
		})
	}
}

func TestControllerDeleteTestExecution(t *testing.T) {
	const (
		ref = "execution-ref"
		id  = "test-execution-001"
	)

	repositoryTestExecution := &repository.TestExecution{
		ID:     id,
		Status: "finished",
	}

	errDeleteTestExecution := errors.New(
		"delete test execution error",
	)

	tests := []struct {
		name string

		setup func(
			repo *mock.MockTestExecutionRepository,
		)

		expectedID    string
		expectedError error
	}{
		{
			name: "success",
			setup: func(
				repo *mock.MockTestExecutionRepository,
			) {
				repo.EXPECT().
					GetIDsByID(gomock.Any(), ref).
					Return([]string{id}, nil)

				repo.EXPECT().
					Delete(gomock.Any(), id).
					Return(repositoryTestExecution, nil)
			},
			expectedID: id,
		},
		{
			name: "not found",
			setup: func(
				repo *mock.MockTestExecutionRepository,
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
				repo *mock.MockTestExecutionRepository,
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
				repo *mock.MockTestExecutionRepository,
			) {
				repo.EXPECT().
					GetIDsByID(gomock.Any(), ref).
					Return([]string{id}, nil)

				repo.EXPECT().
					Delete(gomock.Any(), id).
					Return(nil, errDeleteTestExecution)
			},
			expectedError: errDeleteTestExecution,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			repo := mock.NewMockTestExecutionRepository(
				ctrl,
			)

			tt.setup(repo)

			controller := &Controller{
				testExecution: repo,
			}

			actual, err := controller.DeleteTestExecution(
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
						"expected nil test execution, got %+v",
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
				t.Fatal("expected test execution")
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

func TestControllerGetTestExecutionsPage(t *testing.T) {
	ctrl := gomock.NewController(t)

	repo := mock.NewMockTestExecutionRepository(
		ctrl,
	)

	controller, err := NewController(nil, nil, nil, repo, nil)
	if err != nil {
		t.Fatalf("Unexpected NewController error: %q", err)
	}

	startedAt := time.Date(
		2026, time.September, 21,
		8, 0, 0, 0,
		time.UTC,
	)

	items := []repository.TestExecution{
		{
			ID:         "0000000000000000000000000000000000000000000000000000000000000001",
			StartedAt:  startedAt,
			FinishedAt: startedAt.Add(time.Minute),
			Status:     "finished",
		},
		{
			ID:         "0000000000000000000000000000000000000000000000000000000000000002",
			StartedAt:  startedAt.Add(time.Minute),
			FinishedAt: startedAt.Add(2 * time.Minute),
			Status:     "finished",
		},
		{
			ID:         "0000000000000000000000000000000000000000000000000000000000000003",
			StartedAt:  startedAt.Add(2 * time.Minute),
			FinishedAt: startedAt.Add(3 * time.Minute),
			Status:     "finished",
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
			) ([]repository.TestExecution, error) {
				if params.Limit != 3 {
					t.Fatalf(
						"expected limit 3, got %d",
						params.Limit,
					)
				}

				if params.OrderBy != repository.OrderBy(
					model.OrderByStartedAt,
				) {
					t.Fatalf(
						"expected order by %q, got %q",
						model.OrderByStartedAt,
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

	page, err := controller.GetTestExecutionsPage(
		t.Context(),
		&GetTestExecutionsPageParams{
			PagingParams: PagingParams{
				PageSize:       2,
				OrderBy:        model.OrderByStartedAt,
				OrderDirection: model.OrderDirectionDesc,
			},
		},
	)
	if err != nil {
		t.Fatalf(
			"get test executions page: %v",
			err,
		)
	}

	if page == nil {
		t.Fatal("expected page")
	}

	if len(page.Items) != 2 {
		t.Fatalf(
			"expected 2 test executions, got %d",
			len(page.Items),
		)
	}

	for i := 0; i < 2; i++ {
		if page.Items[i].ID != items[i].ID {
			t.Fatalf(
				"test execution %d: expected ID %q, got %q",
				i,
				items[i].ID,
				page.Items[i].ID,
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
