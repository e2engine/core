package resource

import (
	"context"
	"time"

	"github.com/ygrebnov/errorc"

	cursorpkg "github.com/e2engine/core/internal/cursor"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/keys"
	"github.com/e2engine/core/repository"
)

type PagingParams struct {
	PageSize       int                  `json:"page_size"`
	Cursor         string               `json:"cursor"`
	OrderBy        model.OrderBy        `json:"order_by"`
	OrderDirection model.OrderDirection `json:"order_direction"`
}

type resolvedPaging struct {
	Cursor         *cursorpkg.Payload
	OrderBy        model.OrderBy
	OrderDirection model.OrderDirection
}

func (s *Controller) resolvePaging(
	ctx context.Context,
	params PagingParams,
	requestKind string,
	isOrderBySupported func(model.OrderBy) bool,
) (*resolvedPaging, error) {
	result := &resolvedPaging{
		OrderBy:        params.OrderBy,
		OrderDirection: params.OrderDirection,
	}

	if !isOrderBySupported(params.OrderBy) {
		return nil, errorc.With(
			errors.ErrInvalidRequestParameters,
			errorc.String(keys.RequestKind, requestKind),
			errorc.String(keys.OrderBy, string(params.OrderBy)),
			errorc.String(keys.Validation, "invalid OrderBy"),
		)
	}

	if !isOrderDirectionSupported(params.OrderDirection) {
		return nil, errorc.With(
			errors.ErrInvalidRequestParameters,
			errorc.String(keys.RequestKind, requestKind),
			errorc.String(keys.OrderDirection, string(params.OrderDirection)),
			errorc.String(keys.Validation, "invalid OrderDirection"),
		)
	}

	if params.Cursor == "" {
		return result, nil
	}

	parsed, err := cursorpkg.Parse(ctx, s.cursorBinding, params.Cursor)
	if err != nil {
		return nil, errorc.With(
			errors.ErrInvalidRequestParameters,
			errorc.String(keys.RequestKind, requestKind),
			errorc.Error(keys.Validation, err),
		)
	}

	if !isOrderBySupported(parsed.OrderBy) {
		return nil, errorc.With(
			errors.ErrInvalidCursor,
			errorc.String(keys.RequestKind, requestKind),
			errorc.String(keys.CursorOrderBy, string(parsed.OrderBy)),
			errorc.String(keys.Validation, "invalid OrderBy"),
		)
	}

	result.Cursor = &parsed
	result.OrderBy = parsed.OrderBy
	result.OrderDirection = parsed.OrderDirection

	return result, nil
}

func isOrderDirectionSupported(
	direction model.OrderDirection,
) bool {
	switch direction {
	case model.OrderDirectionAsc,
		model.OrderDirectionDesc:
		return true
	default:
		return false
	}
}

type PagingAnchor struct {
	ID             string
	Position       repository.Position
	ValueString    string
	ValueTimestamp time.Time
}

func decodePagingAnchor(
	payload *cursorpkg.Payload,
	requestKind string,
	isTimestampOrderBy func(model.OrderBy) bool,
) (*PagingAnchor, error) {
	if payload == nil {
		return &PagingAnchor{
			Position: repository.PositionAfter,
		}, nil
	}

	anchor := &PagingAnchor{
		ID:       payload.ID,
		Position: repository.Position(payload.Position),
	}

	if !isTimestampOrderBy(payload.OrderBy) {
		anchor.ValueString = payload.Value
		return anchor, nil
	}

	value, err := time.Parse(time.RFC3339Nano, payload.Value)
	if err != nil {
		return nil, errorc.With(
			errors.ErrInvalidCursor,
			errorc.String(keys.RequestKind, requestKind),
			errorc.String(keys.CursorTimestampValue, payload.Value),
			errorc.Error(keys.Validation, err),
		)
	}

	anchor.ValueTimestamp = value

	return anchor, nil
}

func buildPage[R repository.Entity, M model.Entity](
	ctx context.Context,
	s *Controller,
	list []R,
	pageSize int,
	anchor *PagingAnchor,
	paging *resolvedPaging,
	getMSliceFromRSlice func([]R, bool) ([]M, error),
	getCursorValue func(*M, model.OrderBy) string,
	getID func(M) string,
) (*model.Page[M], error) {
	if len(list) == 0 {
		return &model.Page[M]{}, nil
	}

	itemCount := len(list)
	hasMore := itemCount > pageSize

	if hasMore {
		itemCount = pageSize
		list = list[:itemCount]
	}

	mList, err := getMSliceFromRSlice(
		list, anchor.Position == repository.PositionBefore,
	)
	if err != nil {
		return nil, err
	}

	page := &model.Page[M]{
		Items: mList,
	}

	encodeCursor := func(
		position cursorpkg.Position,
		item M,
	) (string, error) {
		return cursorpkg.Encode(
			ctx,
			s.cursorBinding,
			cursorpkg.Payload{
				ID:             getID(item),
				Value:          getCursorValue(&item, paging.OrderBy),
				OrderBy:        paging.OrderBy,
				OrderDirection: paging.OrderDirection,
				Position:       position,
			},
		)
	}

	if hasMore {
		if anchor.Position == repository.PositionBefore {
			page.Before, err = encodeCursor(
				cursorpkg.PositionBefore,
				page.Items[0],
			)
		} else {
			page.After, err = encodeCursor(
				cursorpkg.PositionAfter,
				page.Items[itemCount-1],
			)
		}

		if err != nil {
			return nil, err
		}
	}

	if paging.Cursor != nil {
		if anchor.Position == repository.PositionBefore {
			page.After, err = encodeCursor(
				cursorpkg.PositionAfter,
				page.Items[itemCount-1],
			)
		} else {
			page.Before, err = encodeCursor(
				cursorpkg.PositionBefore,
				page.Items[0],
			)
		}

		if err != nil {
			return nil, err
		}
	}

	return page, nil
}
