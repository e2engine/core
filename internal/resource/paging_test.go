package resource

import (
	"context"
	"testing"
	"time"

	cursorpkg "github.com/e2engine/core/internal/cursor"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/repository"
)

const pagingTestRequestKind = "PagingTest"

func TestResolvePaging(t *testing.T) {
	t.Parallel()

	controller := newPagingTestController(t)

	tests := []struct {
		name          string
		params        PagingParams
		isSupported   func(model.OrderBy) bool
		wantOrderBy   model.OrderBy
		wantDirection model.OrderDirection
		wantErr       error
	}{
		{
			name: "first page",
			params: PagingParams{
				PageSize:       10,
				OrderBy:        model.OrderByName,
				OrderDirection: model.OrderDirectionAsc,
			},
			isSupported: func(orderBy model.OrderBy) bool {
				return orderBy == model.OrderByName
			},
			wantOrderBy:   model.OrderByName,
			wantDirection: model.OrderDirectionAsc,
		},
		{
			name: "unsupported order by",
			params: PagingParams{
				PageSize:       10,
				OrderBy:        model.OrderByStartedAt,
				OrderDirection: model.OrderDirectionAsc,
			},
			isSupported: func(orderBy model.OrderBy) bool {
				return orderBy == model.OrderByName
			},
			wantErr: errors.ErrInvalidRequestParameters,
		},
		{
			name: "unsupported order direction",
			params: PagingParams{
				PageSize:       10,
				OrderBy:        model.OrderByName,
				OrderDirection: model.OrderDirection("invalid"),
			},
			isSupported: func(orderBy model.OrderBy) bool {
				return orderBy == model.OrderByName
			},
			wantErr: errors.ErrInvalidRequestParameters,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := controller.resolvePaging(
				context.Background(),
				tt.params,
				pagingTestRequestKind,
				tt.isSupported,
			)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("resolve paging: %v", err)
			}

			if got.Cursor != nil {
				t.Fatalf("expected nil cursor, got %+v", got.Cursor)
			}

			if got.OrderBy != tt.wantOrderBy {
				t.Errorf(
					"expected order by %q, got %q",
					tt.wantOrderBy,
					got.OrderBy,
				)
			}

			if got.OrderDirection != tt.wantDirection {
				t.Errorf(
					"expected order direction %q, got %q",
					tt.wantDirection,
					got.OrderDirection,
				)
			}
		})
	}
}

func TestResolvePagingCursorOverridesRequestOrdering(t *testing.T) {
	t.Parallel()

	controller := newPagingTestController(t)
	ctx := context.Background()

	encoded, err := cursorpkg.Encode(
		ctx,
		controller.cursorBinding,
		cursorpkg.Payload{
			ID:             "7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3896",
			Value:          "beta",
			OrderBy:        model.OrderByName,
			OrderDirection: model.OrderDirectionDesc,
			Position:       cursorpkg.PositionAfter,
		},
	)
	if err != nil {
		t.Fatalf("encode cursor: %v", err)
	}

	got, err := controller.resolvePaging(
		ctx,
		PagingParams{
			PageSize:       10,
			Cursor:         encoded,
			OrderBy:        model.OrderByID,
			OrderDirection: model.OrderDirectionAsc,
		},
		pagingTestRequestKind,
		func(orderBy model.OrderBy) bool {
			return orderBy == model.OrderByID ||
				orderBy == model.OrderByName
		},
	)
	if err != nil {
		t.Fatalf("resolve paging: %v", err)
	}

	if got.Cursor == nil {
		t.Fatal("expected parsed cursor")
	}

	if got.OrderBy != model.OrderByName {
		t.Errorf(
			"expected cursor order by %q, got %q",
			model.OrderByName,
			got.OrderBy,
		)
	}

	if got.OrderDirection != model.OrderDirectionDesc {
		t.Errorf(
			"expected cursor order direction %q, got %q",
			model.OrderDirectionDesc,
			got.OrderDirection,
		)
	}

	if got.Cursor.ID != "7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3896" {
		t.Errorf(
			"expected cursor ID %q, got %q",
			"7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3896",
			got.Cursor.ID,
		)
	}
}

func TestResolvePagingRejectsUnsupportedCursorOrderBy(t *testing.T) {
	t.Parallel()

	controller := newPagingTestController(t)
	ctx := context.Background()

	encoded, err := cursorpkg.Encode(
		ctx,
		controller.cursorBinding,
		cursorpkg.Payload{
			ID:             "7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3896",
			Value:          "name",
			OrderBy:        model.OrderByName,
			OrderDirection: model.OrderDirectionAsc,
			Position:       cursorpkg.PositionAfter,
		},
	)
	if err != nil {
		t.Fatalf("encode cursor: %v", err)
	}

	_, err = controller.resolvePaging(
		ctx,
		PagingParams{
			PageSize:       10,
			Cursor:         encoded,
			OrderBy:        model.OrderByID,
			OrderDirection: model.OrderDirectionAsc,
		},
		pagingTestRequestKind,
		isExecutionOrderBy,
	)
	if !errors.Is(err, errors.ErrInvalidCursor) {
		t.Fatalf(
			"expected %v, got %v",
			errors.ErrInvalidCursor,
			err,
		)
	}
}

func TestDecodePagingAnchor(t *testing.T) {
	t.Parallel()

	t.Run("initial page", func(t *testing.T) {
		t.Parallel()

		got, err := decodePagingAnchor(
			nil,
			pagingTestRequestKind,
			isResourceTimestampOrderBy,
		)
		if err != nil {
			t.Fatalf("decode paging anchor: %v", err)
		}

		if got.Position != repository.PositionAfter {
			t.Errorf(
				"expected position %q, got %q",
				repository.PositionAfter,
				got.Position,
			)
		}

		if got.ID != "" {
			t.Errorf("expected empty ID, got %q", got.ID)
		}
	})

	t.Run("string value", func(t *testing.T) {
		t.Parallel()

		got, err := decodePagingAnchor(
			&cursorpkg.Payload{
				ID:       "environment-2",
				Value:    "beta",
				OrderBy:  model.OrderByName,
				Position: cursorpkg.PositionBefore,
			},
			pagingTestRequestKind,
			isResourceTimestampOrderBy,
		)
		if err != nil {
			t.Fatalf("decode paging anchor: %v", err)
		}

		if got.ID != "environment-2" {
			t.Errorf(
				"expected ID %q, got %q",
				"environment-2",
				got.ID,
			)
		}

		if got.Position != repository.PositionBefore {
			t.Errorf(
				"expected position %q, got %q",
				repository.PositionBefore,
				got.Position,
			)
		}

		if got.ValueString != "beta" {
			t.Errorf(
				"expected string value %q, got %q",
				"beta",
				got.ValueString,
			)
		}

		if !got.ValueTimestamp.IsZero() {
			t.Errorf(
				"expected zero timestamp, got %v",
				got.ValueTimestamp,
			)
		}
	})

	t.Run("timestamp value", func(t *testing.T) {
		t.Parallel()

		want := time.Date(
			2026, time.September, 20,
			12, 34, 56, 123456789,
			time.UTC,
		)

		got, err := decodePagingAnchor(
			&cursorpkg.Payload{
				ID:       "environment-2",
				Value:    want.Format(time.RFC3339Nano),
				OrderBy:  model.OrderByUpdatedAt,
				Position: cursorpkg.PositionAfter,
			},
			pagingTestRequestKind,
			isResourceTimestampOrderBy,
		)
		if err != nil {
			t.Fatalf("decode paging anchor: %v", err)
		}

		if !got.ValueTimestamp.Equal(want) {
			t.Errorf(
				"expected timestamp %v, got %v",
				want,
				got.ValueTimestamp,
			)
		}

		if got.ValueString != "" {
			t.Errorf(
				"expected empty string value, got %q",
				got.ValueString,
			)
		}
	})

	t.Run("invalid timestamp", func(t *testing.T) {
		t.Parallel()

		_, err := decodePagingAnchor(
			&cursorpkg.Payload{
				ID:       "environment-2",
				Value:    "invalid",
				OrderBy:  model.OrderByUpdatedAt,
				Position: cursorpkg.PositionAfter,
			},
			pagingTestRequestKind,
			isResourceTimestampOrderBy,
		)

		if !errors.Is(err, errors.ErrInvalidCursor) {
			t.Fatalf(
				"expected %v, got %v",
				errors.ErrInvalidCursor,
				err,
			)
		}
	})
}

func TestBuildPageEmpty(t *testing.T) {
	t.Parallel()

	controller := newPagingTestController(t)

	page, err := buildEnvironmentPage(
		context.Background(),
		controller,
		nil,
		3,
		&PagingAnchor{
			Position: repository.PositionAfter,
		},
		&resolvedPaging{
			OrderBy:        model.OrderByID,
			OrderDirection: model.OrderDirectionAsc,
		},
	)
	if err != nil {
		t.Fatalf("build page: %v", err)
	}

	if len(page.Items) != 0 {
		t.Fatalf("expected no items, got %d", len(page.Items))
	}

	if page.Before != "" {
		t.Errorf("expected no before cursor, got %q", page.Before)
	}

	if page.After != "" {
		t.Errorf("expected no after cursor, got %q", page.After)
	}
}

func TestBuildPageForward(t *testing.T) {
	t.Parallel()

	controller := newPagingTestController(t)
	ctx := context.Background()

	t.Run("first page with more items", func(t *testing.T) {
		t.Parallel()

		page, err := buildEnvironmentPage(
			ctx,
			controller,
			pagingTestEnvironments(
				"7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3891",
				"7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3892",
				"7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3893",
				"7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3894",
			),
			3,
			&PagingAnchor{
				Position: repository.PositionAfter,
			},
			&resolvedPaging{
				OrderBy:        model.OrderByID,
				OrderDirection: model.OrderDirectionAsc,
			},
		)
		if err != nil {
			t.Fatalf("build page: %v", err)
		}

		assertEnvironmentIDs(
			t,
			page.Items,
			"7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3891",
			"7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3892",
			"7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3893",
		)

		if page.Before != "" {
			t.Errorf(
				"expected no before cursor, got %q",
				page.Before,
			)
		}

		assertPageCursor(
			t,
			ctx,
			controller,
			page.After,
			"7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3893",
			cursorpkg.PositionAfter,
			model.OrderByID,
			model.OrderDirectionAsc,
		)
	})

	t.Run("middle page", func(t *testing.T) {
		t.Parallel()

		page, err := buildEnvironmentPage(
			ctx,
			controller,
			pagingTestEnvironments(
				"7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3894",
				"7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3895",
				"7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3896",
				"7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3897",
			),
			3,
			&PagingAnchor{
				ID:       "7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3893",
				Position: repository.PositionAfter,
			},
			&resolvedPaging{
				Cursor: &cursorpkg.Payload{
					ID:       "7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3893",
					Position: cursorpkg.PositionAfter,
				},
				OrderBy:        model.OrderByID,
				OrderDirection: model.OrderDirectionAsc,
			},
		)
		if err != nil {
			t.Fatalf("build page: %v", err)
		}

		assertEnvironmentIDs(
			t,
			page.Items,
			"7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3894",
			"7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3895",
			"7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3896",
		)

		assertPageCursor(
			t,
			ctx,
			controller,
			page.Before,
			"7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3894",
			cursorpkg.PositionBefore,
			model.OrderByID,
			model.OrderDirectionAsc,
		)

		assertPageCursor(
			t,
			ctx,
			controller,
			page.After,
			"7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3896",
			cursorpkg.PositionAfter,
			model.OrderByID,
			model.OrderDirectionAsc,
		)
	})

	t.Run("last page", func(t *testing.T) {
		t.Parallel()

		page, err := buildEnvironmentPage(
			ctx,
			controller,
			pagingTestEnvironments(
				"7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3897",
				"7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3898",
			),
			3,
			&PagingAnchor{
				ID:       "7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3896",
				Position: repository.PositionAfter,
			},
			&resolvedPaging{
				Cursor: &cursorpkg.Payload{
					ID:       "7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3896",
					Position: cursorpkg.PositionAfter,
				},
				OrderBy:        model.OrderByID,
				OrderDirection: model.OrderDirectionAsc,
			},
		)
		if err != nil {
			t.Fatalf("build page: %v", err)
		}

		assertEnvironmentIDs(
			t,
			page.Items,
			"7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3897",
			"7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3898",
		)

		assertPageCursor(
			t,
			ctx,
			controller,
			page.Before,
			"7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3897",
			cursorpkg.PositionBefore,
			model.OrderByID,
			model.OrderDirectionAsc,
		)

		if page.After != "" {
			t.Errorf(
				"expected no after cursor, got %q",
				page.After,
			)
		}
	})
}

func TestBuildPageBackward(t *testing.T) {
	t.Parallel()

	controller := newPagingTestController(t)
	ctx := context.Background()

	t.Run("middle page", func(t *testing.T) {
		t.Parallel()

		// A repository "before" query returns rows in reverse
		// traversal order. The fourth row is the over-fetched row.
		page, err := buildEnvironmentPage(
			ctx,
			controller,
			pagingTestEnvironments(""+
				"7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3896",
				"7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3895",
				"7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3894",
				"7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3893",
			),
			3,
			&PagingAnchor{
				ID:       "7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3897",
				Position: repository.PositionBefore,
			},
			&resolvedPaging{
				Cursor: &cursorpkg.Payload{
					ID:       "7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3897",
					Position: cursorpkg.PositionBefore,
				},
				OrderBy:        model.OrderByID,
				OrderDirection: model.OrderDirectionAsc,
			},
		)
		if err != nil {
			t.Fatalf("build page: %v", err)
		}

		assertEnvironmentIDs(
			t,
			page.Items,
			"7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3894",
			"7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3895",
			"7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3896",
		)

		assertPageCursor(
			t,
			ctx,
			controller,
			page.Before,
			"7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3894",
			cursorpkg.PositionBefore,
			model.OrderByID,
			model.OrderDirectionAsc,
		)

		assertPageCursor(
			t,
			ctx,
			controller,
			page.After,
			"7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3896",
			cursorpkg.PositionAfter,
			model.OrderByID,
			model.OrderDirectionAsc,
		)
	})

	t.Run("first page reached", func(t *testing.T) {
		t.Parallel()

		page, err := buildEnvironmentPage(
			ctx,
			controller,
			pagingTestEnvironments(
				"7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3893",
				"7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3892",
				"7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3891",
			),
			3,
			&PagingAnchor{
				ID:       "7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3894",
				Position: repository.PositionBefore,
			},
			&resolvedPaging{
				Cursor: &cursorpkg.Payload{
					ID:       "7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3894",
					Position: cursorpkg.PositionBefore,
				},
				OrderBy:        model.OrderByID,
				OrderDirection: model.OrderDirectionAsc,
			},
		)
		if err != nil {
			t.Fatalf("build page: %v", err)
		}

		assertEnvironmentIDs(
			t,
			page.Items,
			"7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3891",
			"7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3892",
			"7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3893",
		)

		if page.Before != "" {
			t.Errorf(
				"expected no before cursor, got %q",
				page.Before,
			)
		}

		assertPageCursor(
			t,
			ctx,
			controller,
			page.After,
			"7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3893",
			cursorpkg.PositionAfter,
			model.OrderByID,
			model.OrderDirectionAsc,
		)
	})
}

func newPagingTestController(t *testing.T) *Controller {
	t.Helper()

	controller, err := NewController(
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("create controller: %v", err)
	}

	return controller
}

func buildEnvironmentPage(
	ctx context.Context,
	controller *Controller,
	list []repository.Environment,
	pageSize int,
	anchor *PagingAnchor,
	paging *resolvedPaging,
) (*model.Page[model.Environment], error) {
	return buildPage(
		ctx,
		controller,
		list,
		pageSize,
		anchor,
		paging,
		convertPagingTestEnvironments,
		func(environment *model.Environment, _ model.OrderBy) string {
			return environment.ID
		},
		func(environment model.Environment) string {
			return environment.ID
		},
	)
}

func convertPagingTestEnvironments(
	list []repository.Environment,
	reverse bool,
) ([]model.Environment, error) {
	result := make([]model.Environment, 0, len(list))

	for i := range list {
		index := i
		if reverse {
			index = len(list) - 1 - i
		}

		result = append(
			result,
			model.Environment{
				ID: list[index].ID,
			},
		)
	}

	return result, nil
}

func pagingTestEnvironments(
	ids ...string,
) []repository.Environment {
	result := make([]repository.Environment, 0, len(ids))

	for _, id := range ids {
		result = append(
			result,
			repository.Environment{
				ID: id,
			},
		)
	}

	return result
}

func assertEnvironmentIDs(
	t *testing.T,
	items []model.Environment,
	want ...string,
) {
	t.Helper()

	if len(items) != len(want) {
		t.Fatalf(
			"expected %d items, got %d",
			len(want),
			len(items),
		)
	}

	for i := range want {
		if items[i].ID != want[i] {
			t.Errorf(
				"item %d: expected ID %q, got %q",
				i,
				want[i],
				items[i].ID,
			)
		}
	}
}

func assertPageCursor(
	t *testing.T,
	ctx context.Context,
	controller *Controller,
	encoded string,
	wantID string,
	wantPosition cursorpkg.Position,
	wantOrderBy model.OrderBy,
	wantDirection model.OrderDirection,
) {
	t.Helper()

	if encoded == "" {
		t.Fatal("expected cursor")
	}

	payload, err := cursorpkg.Parse(
		ctx,
		controller.cursorBinding,
		encoded,
	)
	if err != nil {
		t.Fatalf("parse cursor: %v", err)
	}

	if payload.ID != wantID {
		t.Errorf(
			"expected cursor ID %q, got %q",
			wantID,
			payload.ID,
		)
	}

	if payload.Position != wantPosition {
		t.Errorf(
			"expected cursor position %q, got %q",
			wantPosition,
			payload.Position,
		)
	}

	if payload.OrderBy != wantOrderBy {
		t.Errorf(
			"expected cursor order by %q, got %q",
			wantOrderBy,
			payload.OrderBy,
		)
	}

	if payload.OrderDirection != wantDirection {
		t.Errorf(
			"expected cursor direction %q, got %q",
			wantDirection,
			payload.OrderDirection,
		)
	}
}
