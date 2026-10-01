package resource

import (
	"context"
	"errors"
	"testing"

	"github.com/e2engine/core/model"
	coreerrors "github.com/e2engine/core/pkg/errors"
)

var (
	errGetIDsByID   = errors.New("get IDs by ID")
	errGetIDsByName = errors.New("get IDs by name")
	errConvert      = errors.New("convert")
)

type fakeResourceReferenceRepository struct {
	idsByID   []string
	idsByName []string

	idErr   error
	nameErr error

	idCalls   int
	nameCalls int
}

func (r *fakeResourceReferenceRepository) GetIDsByID(
	_ context.Context,
	_ string,
) ([]string, error) {
	r.idCalls++
	return r.idsByID, r.idErr
}

func (r *fakeResourceReferenceRepository) GetIDsByName(
	_ context.Context,
	_ string,
) ([]string, error) {
	r.nameCalls++
	return r.idsByName, r.nameErr
}

func TestResolveResourceID(t *testing.T) {
	tests := []struct {
		name string

		idsByID   []string
		idsByName []string
		idErr     error
		nameErr   error

		expectedID        string
		expectedError     error
		expectedNameCalls int
	}{
		{
			name:              "unique id",
			idsByID:           []string{"id1"},
			idsByName:         []string{"id2"},
			expectedID:        "id1",
			expectedNameCalls: 0,
		},
		{
			name:              "ambiguous id unique name",
			idsByID:           []string{"id1", "id2"},
			idsByName:         []string{"id3"},
			expectedID:        "id3",
			expectedNameCalls: 1,
		},
		{
			name:              "no id unique name",
			idsByName:         []string{"id1"},
			expectedID:        "id1",
			expectedNameCalls: 1,
		},
		{
			name:              "ambiguous id no name",
			idsByID:           []string{"id1", "id2"},
			expectedError:     coreerrors.ErrEntityPrefixAmbiguous,
			expectedNameCalls: 1,
		},
		{
			name:              "ambiguous id ambiguous name",
			idsByID:           []string{"id1", "id2"},
			idsByName:         []string{"id3", "id4"},
			expectedError:     coreerrors.ErrEntityPrefixAmbiguous,
			expectedNameCalls: 1,
		},
		{
			name:              "no id ambiguous name",
			idsByName:         []string{"id1", "id2"},
			expectedError:     coreerrors.ErrEntityPrefixAmbiguous,
			expectedNameCalls: 1,
		},
		{
			name:              "not found",
			expectedError:     coreerrors.ErrEntityNotFound,
			expectedNameCalls: 1,
		},
		{
			name:              "id lookup error",
			idErr:             errGetIDsByID,
			expectedError:     errGetIDsByID,
			expectedNameCalls: 0,
		},
		{
			name:              "name lookup error",
			idsByID:           []string{"id1", "id2"},
			nameErr:           errGetIDsByName,
			expectedError:     errGetIDsByName,
			expectedNameCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeResourceReferenceRepository{
				idsByID:   tt.idsByID,
				idsByName: tt.idsByName,
				idErr:     tt.idErr,
				nameErr:   tt.nameErr,
			}

			id, err := resolveResourceID(
				context.Background(),
				repo,
				"ref",
				model.EntityKindEnvironment,
			)

			if tt.expectedError != nil {
				if !errors.Is(err, tt.expectedError) {
					t.Fatalf(
						"expected error %v, got %v",
						tt.expectedError,
						err,
					)
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}

				if id != tt.expectedID {
					t.Fatalf(
						"expected id %q, got %q",
						tt.expectedID,
						id,
					)
				}
			}

			if repo.idCalls != 1 {
				t.Fatalf(
					"expected GetIDsByID to be called once, got %d",
					repo.idCalls,
				)
			}

			if repo.nameCalls != tt.expectedNameCalls {
				t.Fatalf(
					"expected GetIDsByName to be called %d times, got %d",
					tt.expectedNameCalls,
					repo.nameCalls,
				)
			}
		})
	}
}

type fakeEntityReferenceRepository struct {
	ids []string
	err error
}

func (r *fakeEntityReferenceRepository) GetIDsByID(
	_ context.Context,
	_ string,
) ([]string, error) {
	return r.ids, r.err
}

func TestResolveEntityID(t *testing.T) {
	tests := []struct {
		name string

		ids []string
		err error

		expectedID    string
		expectedError error
	}{
		{
			name:       "unique id",
			ids:        []string{"id1"},
			expectedID: "id1",
		},
		{
			name:          "not found",
			expectedError: coreerrors.ErrEntityNotFound,
		},
		{
			name:          "ambiguous",
			ids:           []string{"id1", "id2"},
			expectedError: coreerrors.ErrEntityPrefixAmbiguous,
		},
		{
			name:          "repository error",
			err:           errGetIDsByID,
			expectedError: errGetIDsByID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeEntityReferenceRepository{
				ids: tt.ids,
				err: tt.err,
			}

			id, err := resolveEntityID(
				context.Background(),
				repo,
				"ref",
				model.EntityKindTestExecution,
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

			if id != tt.expectedID {
				t.Fatalf(
					"expected id %q, got %q",
					tt.expectedID,
					id,
				)
			}
		})
	}
}

func TestIsResourceOrderBy(t *testing.T) {
	tests := []struct {
		orderBy  model.OrderBy
		expected bool
	}{
		{model.OrderByID, true},
		{model.OrderByName, true},
		{model.OrderByVersion, true},
		{model.OrderByCreatedAt, true},
		{model.OrderByUpdatedAt, true},
		{model.OrderByStartedAt, false},
		{model.OrderByFinishedAt, false},
		{model.OrderByStatus, false},
		{model.OrderBy("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.orderBy), func(t *testing.T) {
			if actual := isResourceOrderBy(tt.orderBy); actual != tt.expected {
				t.Fatalf(
					"expected %v for %q, got %v",
					tt.expected,
					tt.orderBy,
					actual,
				)
			}
		})
	}
}

func TestIsResourceTimestampOrderBy(t *testing.T) {
	tests := []struct {
		orderBy  model.OrderBy
		expected bool
	}{
		{model.OrderByCreatedAt, true},
		{model.OrderByUpdatedAt, true},
		{model.OrderByID, false},
		{model.OrderByName, false},
		{model.OrderByVersion, false},
		{model.OrderBy("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.orderBy), func(t *testing.T) {
			if actual := isResourceTimestampOrderBy(tt.orderBy); actual != tt.expected {
				t.Fatalf(
					"expected %v for %q, got %v",
					tt.expected,
					tt.orderBy,
					actual,
				)
			}
		})
	}
}

func TestIsExecutionOrderBy(t *testing.T) {
	tests := []struct {
		orderBy  model.OrderBy
		expected bool
	}{
		{model.OrderByID, true},
		{model.OrderByStartedAt, true},
		{model.OrderByFinishedAt, true},
		{model.OrderByStatus, true},
		{model.OrderByName, false},
		{model.OrderByCreatedAt, false},
		{model.OrderByUpdatedAt, false},
		{model.OrderBy("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.orderBy), func(t *testing.T) {
			if actual := isExecutionOrderBy(tt.orderBy); actual != tt.expected {
				t.Fatalf(
					"expected %v for %q, got %v",
					tt.expected,
					tt.orderBy,
					actual,
				)
			}
		})
	}
}

func TestIsExecutionTimestampOrderBy(t *testing.T) {
	tests := []struct {
		orderBy  model.OrderBy
		expected bool
	}{
		{model.OrderByStartedAt, true},
		{model.OrderByFinishedAt, true},
		{model.OrderByID, false},
		{model.OrderByStatus, false},
		{model.OrderBy("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.orderBy), func(t *testing.T) {
			if actual := isExecutionTimestampOrderBy(tt.orderBy); actual != tt.expected {
				t.Fatalf(
					"expected %v for %q, got %v",
					tt.expected,
					tt.orderBy,
					actual,
				)
			}
		})
	}
}

func TestConvertRepositorySlice(t *testing.T) {
	type repositoryItem struct {
		value int
	}

	type modelItem struct {
		value int
	}

	convert := func(item *repositoryItem) (*modelItem, error) {
		if item.value == -1 {
			return nil, errConvert
		}

		return &modelItem{
			value: item.value * 10,
		}, nil
	}

	t.Run("forward", func(t *testing.T) {
		items := []repositoryItem{
			{value: 1},
			{value: 2},
			{value: 3},
		}

		result, err := convertRepositorySlice(items, false, convert)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		expected := []int{10, 20, 30}
		for i := range result {
			if result[i].value != expected[i] {
				t.Fatalf(
					"expected value %d at %d, got %d",
					expected[i],
					i,
					result[i].value,
				)
			}
		}
	})

	t.Run("reverse", func(t *testing.T) {
		items := []repositoryItem{
			{value: 1},
			{value: 2},
			{value: 3},
		}

		result, err := convertRepositorySlice(items, true, convert)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		expected := []int{30, 20, 10}
		for i := range result {
			if result[i].value != expected[i] {
				t.Fatalf(
					"expected value %d at %d, got %d",
					expected[i],
					i,
					result[i].value,
				)
			}
		}
	})

	t.Run("empty", func(t *testing.T) {
		result, err := convertRepositorySlice(
			[]repositoryItem{},
			false,
			convert,
		)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if len(result) != 0 {
			t.Fatalf("expected empty result, got %v", result)
		}
	})

	t.Run("conversion error", func(t *testing.T) {
		items := []repositoryItem{
			{value: 1},
			{value: -1},
			{value: 3},
		}

		_, err := convertRepositorySlice(items, false, convert)
		if !errors.Is(err, errConvert) {
			t.Fatalf("expected error %v, got %v", errConvert, err)
		}
	})
}
