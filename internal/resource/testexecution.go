package resource

import (
	"context"
	"encoding/json"
	"time"

	"github.com/ygrebnov/errorc"

	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/keys"
	"github.com/e2engine/core/repository"
)

type GetTestExecutionsPageParams struct {
	PagingParams
}

const RequestKindGetTestExecutionsPage = "GetTestExecutionsPage"

func (s *Controller) GetTestExecutionsPage(
	ctx context.Context,
	params *GetTestExecutionsPageParams,
) (*model.TestExecutionsPage, error) {
	paging, err := s.resolvePaging(
		ctx, params.PagingParams, RequestKindGetTestExecutionsPage, isExecutionOrderBy,
	)
	if err != nil {
		return nil, err
	}

	anchor, err := decodePagingAnchor(
		paging.Cursor, RequestKindGetTestExecutionsPage, isExecutionTimestampOrderBy,
	)
	if err != nil {
		return nil, err
	}

	arg := &repository.ListParams{
		Limit:          params.PageSize + 1,
		ID:             anchor.ID,
		Position:       anchor.Position,
		ValueString:    anchor.ValueString,
		ValueTimestamp: anchor.ValueTimestamp,
		OrderBy:        repository.OrderBy(paging.OrderBy),
		OrderDirection: repository.OrderDirection(paging.OrderDirection),
	}

	list, err := s.testExecution.List(ctx, arg)
	if err != nil {
		return nil, err
	}

	return buildPage(
		ctx, s, list, params.PageSize, anchor, paging,
		newTestExecutionSliceFromRepository, getTestExecutionCursorValue,
		func(testExecution model.TestExecution) string { return testExecution.ID },
	)
}

func getTestExecutionCursorValue(testExecution *model.TestExecution, field model.OrderBy) string {
	switch field {
	case model.OrderByID:
		return testExecution.ID
	case model.OrderByStartedAt:
		return testExecution.StartedAt.Format(time.RFC3339Nano)
	case model.OrderByFinishedAt:
		return testExecution.FinishedAt.Format(time.RFC3339Nano)
	case model.OrderByStatus:
		return string(testExecution.Status)
	default:
		panic("unknown test execution cursor field")
	}
}

func (s *Controller) GetTestExecution(ctx context.Context, prefix string) (*model.TestExecution, error) {
	id, err := s.getTestExecutionID(ctx, prefix)
	if err != nil {
		return nil, err
	}

	test, err := s.testExecution.Get(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, entityNotFoundError(model.EntityKindTestExecution, id, prefix)
		}
		return nil, err
	}

	return newTestExecutionFromRepository(test)
}

func (s *Controller) getTestExecutionID(ctx context.Context, ref string) (string, error) {
	return resolveEntityID(
		ctx,
		s.testExecution,
		ref,
		model.EntityKindTestExecution,
	)
}

func (s *Controller) GetTestExecutionStatus(ctx context.Context, prefix string) (model.ExecutionStatus, error) {
	id, err := s.getTestExecutionID(ctx, prefix)
	if err != nil {
		return "", err
	}

	status, err := s.testExecution.GetStatus(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return "", entityNotFoundError(model.EntityKindTestExecution, id, prefix)
		}
		return "", err
	}

	return model.ExecutionStatus(status), nil
}

func (s *Controller) DeleteTestExecution(ctx context.Context, prefix string) (*model.TestExecution, error) {
	id, err := s.getTestExecutionID(ctx, prefix)
	if err != nil {
		return nil, err
	}

	test, err := s.testExecution.Delete(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, entityNotFoundError(model.EntityKindTestExecution, id, prefix)
		}
		if errors.Is(err, repository.ErrConflict) {
			return nil, entityConflictError(model.EntityKindTestExecution, id, prefix)
		}
		return nil, err
	}

	return newTestExecutionFromRepository(test)
}

func newTestExecutionFromRepository(r *repository.TestExecution) (*model.TestExecution, error) {
	if r == nil {
		return nil, nil
	}

	te := &model.TestExecution{
		ID:              r.ID,
		StartedAt:       r.StartedAt,
		FinishedAt:      r.FinishedAt,
		Status:          model.ExecutionStatus(r.Status),
		EnvironmentID:   r.EnvironmentID,
		EnvironmentName: r.EnvironmentName,
		TestID:          r.TestID,
		TestName:        r.TestName,
	}

	if len(r.Summary) > 0 {
		var summary model.TestExecutionSummary
		if err := json.Unmarshal(r.Summary, &summary); err != nil {
			return nil, errorc.With(errors.ErrCannotParseTestExecutionSummary, errorc.Error(keys.Cause, err))
		}
		te.Summary = &summary
	}

	return te, nil
}

func newTestExecutionSliceFromRepository(
	r []repository.TestExecution,
	restorePresentationOrder bool,
) ([]model.TestExecution, error) {
	return convertRepositorySlice(
		r,
		restorePresentationOrder,
		newTestExecutionFromRepository,
	)
}
