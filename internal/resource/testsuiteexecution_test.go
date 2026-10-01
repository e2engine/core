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

func TestGetTestSuiteExecutionCursorValue(t *testing.T) {
	startedAt := time.Date(
		2026, time.September, 21,
		8, 0, 0, 0,
		time.UTC,
	)
	finishedAt := startedAt.Add(time.Minute)

	execution := &model.TestSuiteExecution{
		ID:         "test-suite-execution-001",
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
			expected: execution.ID,
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
			expected: string(execution.Status),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := getTestSuiteExecutionCursorValue(
				execution,
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

func TestGetTestSuiteExecutionCursorValueUnsupported(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()

	getTestSuiteExecutionCursorValue(
		&model.TestSuiteExecution{},
		model.OrderBy("unsupported"),
	)
}

func TestNewTestSuiteExecutionFromRepository(t *testing.T) {
	startedAt := time.Date(
		2026, time.September, 21,
		8, 0, 0, 0,
		time.UTC,
	)
	finishedAt := startedAt.Add(time.Minute)

	summary := model.TestSuiteExecutionSummary{}

	summaryJSON, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("marshal summary: %v", err)
	}

	tests := []struct {
		name string

		input *repository.TestSuiteExecution

		expected      *model.TestSuiteExecution
		expectedError error
	}{
		{
			name: "nil",
		},
		{
			name: "without summary",
			input: &repository.TestSuiteExecution{
				ID:              "test-suite-execution-001",
				StartedAt:       startedAt,
				FinishedAt:      finishedAt,
				Status:          "finished",
				EnvironmentID:   "environment-001",
				EnvironmentName: "environment",
				TestSuiteID:     "test-suite-001",
				TestSuiteName:   "test suite",
				TestsCount:      3,
			},
			expected: &model.TestSuiteExecution{
				ID:              "test-suite-execution-001",
				StartedAt:       startedAt,
				FinishedAt:      finishedAt,
				Status:          model.ExecutionStatus("finished"),
				EnvironmentID:   "environment-001",
				EnvironmentName: "environment",
				TestSuiteID:     "test-suite-001",
				TestSuiteName:   "test suite",
				TestsCount:      3,
			},
		},
		{
			name: "with summary",
			input: &repository.TestSuiteExecution{
				ID:              "test-suite-execution-001",
				StartedAt:       startedAt,
				FinishedAt:      finishedAt,
				Status:          "finished",
				EnvironmentID:   "environment-001",
				EnvironmentName: "environment",
				TestSuiteID:     "test-suite-001",
				TestSuiteName:   "test suite",
				TestsCount:      3,
				Summary:         summaryJSON,
			},
			expected: &model.TestSuiteExecution{
				ID:              "test-suite-execution-001",
				StartedAt:       startedAt,
				FinishedAt:      finishedAt,
				Status:          model.ExecutionStatus("finished"),
				EnvironmentID:   "environment-001",
				EnvironmentName: "environment",
				TestSuiteID:     "test-suite-001",
				TestSuiteName:   "test suite",
				TestsCount:      3,
				Summary:         &summary,
			},
		},
		{
			name: "invalid summary",
			input: &repository.TestSuiteExecution{
				ID:      "test-suite-execution-001",
				Summary: []byte("{"),
			},
			expectedError: coreerrors.ErrCannotParseTestSuiteExecutionSummary,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := newTestSuiteExecutionFromRepository(
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
						"expected nil test suite execution, got %+v",
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

func TestNewTestSuiteExecutionSliceFromRepository(t *testing.T) {
	summaryJSON, err := json.Marshal(
		model.TestSuiteExecutionSummary{},
	)
	if err != nil {
		t.Fatalf("marshal summary: %v", err)
	}

	items := []repository.TestSuiteExecution{
		{
			ID:      "test-suite-execution-001",
			Summary: summaryJSON,
		},
		{
			ID:      "test-suite-execution-002",
			Summary: summaryJSON,
		},
		{
			ID:      "test-suite-execution-003",
			Summary: summaryJSON,
		},
	}

	invalid := repository.TestSuiteExecution{
		ID:      "test-suite-execution-invalid",
		Summary: []byte("{"),
	}

	tests := []struct {
		name string

		input                    []repository.TestSuiteExecution
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
			input:       []repository.TestSuiteExecution{},
			expectedIDs: []string{},
		},
		{
			name:                     "forward",
			input:                    items,
			restorePresentationOrder: false,
			expectedIDs: []string{
				"test-suite-execution-001",
				"test-suite-execution-002",
				"test-suite-execution-003",
			},
		},
		{
			name:                     "reverse",
			input:                    items,
			restorePresentationOrder: true,
			expectedIDs: []string{
				"test-suite-execution-003",
				"test-suite-execution-002",
				"test-suite-execution-001",
			},
		},
		{
			name: "conversion error",
			input: []repository.TestSuiteExecution{
				items[0],
				invalid,
				items[2],
			},
			expectedError: coreerrors.ErrCannotParseTestSuiteExecutionSummary,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := newTestSuiteExecutionSliceFromRepository(
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
					"expected %d test suite executions, got %d",
					len(tt.expectedIDs),
					len(actual),
				)
			}

			for i := range actual {
				if actual[i].ID != tt.expectedIDs[i] {
					t.Fatalf(
						"execution %d: expected ID %q, got %q",
						i,
						tt.expectedIDs[i],
						actual[i].ID,
					)
				}
			}
		})
	}
}

func TestControllerGetTestSuiteExecution(t *testing.T) {
	const (
		ref = "suite-execution-ref"
		id  = "test-suite-execution-001"
	)

	parent := &repository.TestSuiteExecution{
		ID:              id,
		Status:          "finished",
		EnvironmentID:   "environment-001",
		EnvironmentName: "environment",
		TestSuiteID:     "test-suite-001",
		TestSuiteName:   "test suite",
		TestsCount:      2,
	}

	children := []repository.TestExecution{
		{
			ID:     "test-execution-001",
			TestID: "test-001",
		},
		{
			ID:     "test-execution-002",
			TestID: "test-002",
		},
	}

	errGetParent := errors.New("get parent error")
	errGetChildren := errors.New("get children error")

	tests := []struct {
		name string

		setup func(
			*mock.MockTestSuiteExecutionRepository,
			*mock.MockTestExecutionRepository,
		)

		expectedID      string
		expectedTestIDs []string
		expectedError   error
	}{
		{
			name: "success",
			setup: func(
				suiteRepo *mock.MockTestSuiteExecutionRepository,
				testRepo *mock.MockTestExecutionRepository,
			) {
				suiteRepo.EXPECT().
					GetIDsByID(gomock.Any(), ref).
					Return([]string{id}, nil)

				suiteRepo.EXPECT().
					Get(gomock.Any(), id).
					Return(parent, nil)

				testRepo.EXPECT().
					GetByTestSuiteExecutionID(
						gomock.Any(),
						id,
					).
					Return(children, nil)
			},
			expectedID: id,
			expectedTestIDs: []string{
				"test-execution-001",
				"test-execution-002",
			},
		},
		{
			name: "reference not found",
			setup: func(
				suiteRepo *mock.MockTestSuiteExecutionRepository,
				_ *mock.MockTestExecutionRepository,
			) {
				suiteRepo.EXPECT().
					GetIDsByID(gomock.Any(), ref).
					Return([]string{}, nil)
			},
			expectedError: coreerrors.ErrEntityNotFound,
		},
		{
			name: "reference ambiguous",
			setup: func(
				suiteRepo *mock.MockTestSuiteExecutionRepository,
				_ *mock.MockTestExecutionRepository,
			) {
				suiteRepo.EXPECT().
					GetIDsByID(gomock.Any(), ref).
					Return(
						[]string{
							"test-suite-execution-001",
							"test-suite-execution-002",
						},
						nil,
					)
			},
			expectedError: coreerrors.ErrEntityPrefixAmbiguous,
		},
		{
			name: "parent not found",
			setup: func(
				suiteRepo *mock.MockTestSuiteExecutionRepository,
				_ *mock.MockTestExecutionRepository,
			) {
				suiteRepo.EXPECT().
					GetIDsByID(gomock.Any(), ref).
					Return([]string{id}, nil)

				suiteRepo.EXPECT().
					Get(gomock.Any(), id).
					Return(nil, repository.ErrNotFound)
			},
			expectedError: coreerrors.ErrEntityNotFound,
		},
		{
			name: "parent repository error",
			setup: func(
				suiteRepo *mock.MockTestSuiteExecutionRepository,
				_ *mock.MockTestExecutionRepository,
			) {
				suiteRepo.EXPECT().
					GetIDsByID(gomock.Any(), ref).
					Return([]string{id}, nil)

				suiteRepo.EXPECT().
					Get(gomock.Any(), id).
					Return(nil, errGetParent)
			},
			expectedError: errGetParent,
		},
		{
			name: "children repository error",
			setup: func(
				suiteRepo *mock.MockTestSuiteExecutionRepository,
				testRepo *mock.MockTestExecutionRepository,
			) {
				suiteRepo.EXPECT().
					GetIDsByID(gomock.Any(), ref).
					Return([]string{id}, nil)

				suiteRepo.EXPECT().
					Get(gomock.Any(), id).
					Return(parent, nil)

				testRepo.EXPECT().
					GetByTestSuiteExecutionID(
						gomock.Any(),
						id,
					).
					Return(nil, errGetChildren)
			},
			expectedError: errGetChildren,
		},
		{
			name: "child conversion error",
			setup: func(
				suiteRepo *mock.MockTestSuiteExecutionRepository,
				testRepo *mock.MockTestExecutionRepository,
			) {
				suiteRepo.EXPECT().
					GetIDsByID(gomock.Any(), ref).
					Return([]string{id}, nil)

				suiteRepo.EXPECT().
					Get(gomock.Any(), id).
					Return(parent, nil)

				testRepo.EXPECT().
					GetByTestSuiteExecutionID(
						gomock.Any(),
						id,
					).
					Return(
						[]repository.TestExecution{
							{
								ID:      "test-execution-001",
								Summary: []byte("{"),
							},
						},
						nil,
					)
			},
			expectedError: coreerrors.ErrCannotParseTestExecutionSummary,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			suiteRepo := mock.NewMockTestSuiteExecutionRepository(ctrl)
			testRepo := mock.NewMockTestExecutionRepository(ctrl)

			tt.setup(
				suiteRepo,
				testRepo,
			)

			controller := &Controller{
				testSuiteExecution: suiteRepo,
				testExecution:      testRepo,
			}

			actual, err := controller.GetTestSuiteExecution(
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
						"expected nil test suite execution, got %+v",
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
				t.Fatal("expected test suite execution")
			}

			if actual.ID != tt.expectedID {
				t.Fatalf(
					"expected ID %q, got %q",
					tt.expectedID,
					actual.ID,
				)
			}

			if len(actual.Tests) != len(tt.expectedTestIDs) {
				t.Fatalf(
					"expected %d tests, got %d",
					len(tt.expectedTestIDs),
					len(actual.Tests),
				)
			}

			for i := range actual.Tests {
				if actual.Tests[i].ID != tt.expectedTestIDs[i] {
					t.Fatalf(
						"test %d: expected ID %q, got %q",
						i,
						tt.expectedTestIDs[i],
						actual.Tests[i].ID,
					)
				}
			}
		})
	}
}

func TestControllerGetTestSuiteExecutionStatus(t *testing.T) {
	const (
		ref = "suite-execution-ref"
		id  = "test-suite-execution-001"
	)

	errGetStatus := errors.New(
		"get test suite execution status error",
	)

	tests := []struct {
		name string

		setup func(
			*mock.MockTestSuiteExecutionRepository,
		)

		expectedStatus model.ExecutionStatus
		expectedError  error
	}{
		{
			name: "success",
			setup: func(
				repo *mock.MockTestSuiteExecutionRepository,
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
				repo *mock.MockTestSuiteExecutionRepository,
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
				repo *mock.MockTestSuiteExecutionRepository,
			) {
				repo.EXPECT().
					GetIDsByID(gomock.Any(), ref).
					Return(
						[]string{
							"test-suite-execution-001",
							"test-suite-execution-002",
						},
						nil,
					)
			},
			expectedError: coreerrors.ErrEntityPrefixAmbiguous,
		},
		{
			name: "repository not found",
			setup: func(
				repo *mock.MockTestSuiteExecutionRepository,
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
				repo *mock.MockTestSuiteExecutionRepository,
			) {
				repo.EXPECT().
					GetIDsByID(gomock.Any(), ref).
					Return([]string{id}, nil)

				repo.EXPECT().
					GetStatus(gomock.Any(), id).
					Return("", errGetStatus)
			},
			expectedError: errGetStatus,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			repo := mock.NewMockTestSuiteExecutionRepository(ctrl)

			tt.setup(repo)

			controller := &Controller{
				testSuiteExecution: repo,
			}

			actual, err := controller.GetTestSuiteExecutionStatus(
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

func TestControllerDeleteTestSuiteExecution(t *testing.T) {
	const (
		ref = "suite-execution-ref"
		id  = "test-suite-execution-001"
	)

	parent := &repository.TestSuiteExecution{
		ID:     id,
		Status: "finished",
	}

	errDelete := errors.New(
		"delete test suite execution error",
	)

	tests := []struct {
		name string

		setup func(
			*mock.MockTestSuiteExecutionRepository,
		)

		expectedID    string
		expectedError error
	}{
		{
			name: "success",
			setup: func(
				repo *mock.MockTestSuiteExecutionRepository,
			) {
				repo.EXPECT().
					GetIDsByID(gomock.Any(), ref).
					Return([]string{id}, nil)

				repo.EXPECT().
					Delete(gomock.Any(), id).
					Return(parent, nil)
			},
			expectedID: id,
		},
		{
			name: "not found",
			setup: func(
				repo *mock.MockTestSuiteExecutionRepository,
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
				repo *mock.MockTestSuiteExecutionRepository,
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
				repo *mock.MockTestSuiteExecutionRepository,
			) {
				repo.EXPECT().
					GetIDsByID(gomock.Any(), ref).
					Return([]string{id}, nil)

				repo.EXPECT().
					Delete(gomock.Any(), id).
					Return(nil, errDelete)
			},
			expectedError: errDelete,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			repo := mock.NewMockTestSuiteExecutionRepository(ctrl)

			tt.setup(repo)

			controller := &Controller{
				testSuiteExecution: repo,
			}

			actual, err := controller.DeleteTestSuiteExecution(
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
						"expected nil test suite execution, got %+v",
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
				t.Fatal("expected test suite execution")
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

func TestControllerGetTestSuiteExecutionsPage(t *testing.T) {
	ctrl := gomock.NewController(t)

	repo := mock.NewMockTestSuiteExecutionRepository(ctrl)

	controller, err := NewController(nil, nil, nil, nil, repo)
	if err != nil {
		t.Fatalf("Unexpected NewController error: %q", err)
	}

	startedAt := time.Date(
		2026, time.September, 21,
		8, 0, 0, 0,
		time.UTC,
	)

	items := []repository.TestSuiteExecution{
		{
			ID:         "0000000000000000000000000000000000000000000000000000000000000001",
			StartedAt:  startedAt,
			FinishedAt: startedAt.Add(time.Minute),
			Status:     "finished",
			TestsCount: 2,
		},
		{
			ID:         "0000000000000000000000000000000000000000000000000000000000000002",
			StartedAt:  startedAt.Add(time.Minute),
			FinishedAt: startedAt.Add(2 * time.Minute),
			Status:     "finished",
			TestsCount: 3,
		},
		{
			ID:         "0000000000000000000000000000000000000000000000000000000000000003",
			StartedAt:  startedAt.Add(2 * time.Minute),
			FinishedAt: startedAt.Add(3 * time.Minute),
			Status:     "finished",
			TestsCount: 4,
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
			) ([]repository.TestSuiteExecution, error) {
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

	page, err := controller.GetTestSuiteExecutionsPage(
		t.Context(),
		&GetTestSuiteExecutionsPageParams{
			PagingParams: PagingParams{
				PageSize:       2,
				OrderBy:        model.OrderByStartedAt,
				OrderDirection: model.OrderDirectionDesc,
			},
		},
	)
	if err != nil {
		t.Fatalf(
			"get test suite executions page: %v",
			err,
		)
	}

	if page == nil {
		t.Fatal("expected page")
	}

	if len(page.Items) != 2 {
		t.Fatalf(
			"expected 2 test suite executions, got %d",
			len(page.Items),
		)
	}

	for i := 0; i < 2; i++ {
		if page.Items[i].ID != items[i].ID {
			t.Fatalf(
				"execution %d: expected ID %q, got %q",
				i,
				items[i].ID,
				page.Items[i].ID,
			)
		}

		if page.Items[i].TestsCount != items[i].TestsCount {
			t.Fatalf(
				"execution %d: expected tests count %d, got %d",
				i,
				items[i].TestsCount,
				page.Items[i].TestsCount,
			)
		}

		if len(page.Items[i].Tests) != 0 {
			t.Fatalf(
				"execution %d: expected no child tests in page response, got %d",
				i,
				len(page.Items[i].Tests),
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
