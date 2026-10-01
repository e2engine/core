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

type GetTestSuiteExecutionsPageParams struct {
	PagingParams
}

const RequestKindGetTestSuiteExecutionsPage = "GetTestSuiteExecutionsPage"

func (s *Controller) GetTestSuiteExecutionsPage(
	ctx context.Context,
	params *GetTestSuiteExecutionsPageParams,
) (*model.TestSuiteExecutionsPage, error) {
	paging, err := s.resolvePaging(
		ctx, params.PagingParams, RequestKindGetTestSuiteExecutionsPage, isExecutionOrderBy,
	)
	if err != nil {
		return nil, err
	}

	anchor, err := decodePagingAnchor(
		paging.Cursor, RequestKindGetTestSuiteExecutionsPage, isExecutionTimestampOrderBy,
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

	list, err := s.testSuiteExecution.List(ctx, arg)
	if err != nil {
		return nil, err
	}

	return buildPage(
		ctx, s, list, params.PageSize, anchor, paging,
		newTestSuiteExecutionSliceFromRepository, getTestSuiteExecutionCursorValue,
		func(testSuiteExecution model.TestSuiteExecution) string { return testSuiteExecution.ID },
	)
}

func getTestSuiteExecutionCursorValue(testSuiteExecution *model.TestSuiteExecution, field model.OrderBy) string {
	switch field {
	case model.OrderByID:
		return testSuiteExecution.ID
	case model.OrderByStartedAt:
		return testSuiteExecution.StartedAt.Format(time.RFC3339Nano)
	case model.OrderByFinishedAt:
		return testSuiteExecution.FinishedAt.Format(time.RFC3339Nano)
	case model.OrderByStatus:
		return string(testSuiteExecution.Status)
	default:
		panic("unknown test suite execution cursor field")
	}
}

func (s *Controller) GetTestSuiteExecution(ctx context.Context, prefix string) (*model.TestSuiteExecution, error) {
	id, err := s.getTestSuiteExecutionID(ctx, prefix)
	if err != nil {
		return nil, err
	}

	testSuiteExecution, err := s.testSuiteExecution.Get(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, entityNotFoundError(model.EntityKindTestSuiteExecution, id, prefix)
		}
		return nil, err
	}

	result, err := newTestSuiteExecutionFromRepository(testSuiteExecution)
	if err != nil {
		return nil, err
	}

	dbTests, err := s.testExecution.GetByTestSuiteExecutionID(
		ctx,
		testSuiteExecution.ID,
	)
	if err != nil {
		return nil, err
	}

	tests, err := newTestExecutionSliceFromRepository(dbTests, false)
	if err != nil {
		return nil, err
	}

	result.Tests = tests
	return result, nil
}

func (s *Controller) getTestSuiteExecutionID(ctx context.Context, ref string) (string, error) {
	return resolveEntityID(
		ctx,
		s.testSuiteExecution,
		ref,
		model.EntityKindTestSuiteExecution,
	)
}

func (s *Controller) GetTestSuiteExecutionStatus(ctx context.Context, prefix string) (model.ExecutionStatus, error) {
	id, err := s.getTestSuiteExecutionID(ctx, prefix)
	if err != nil {
		return "", err
	}

	status, err := s.testSuiteExecution.GetStatus(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return "", entityNotFoundError(model.EntityKindTestSuiteExecution, id, prefix)
		}
		return "", err
	}

	return model.ExecutionStatus(status), nil
}

func (s *Controller) DeleteTestSuiteExecution(ctx context.Context, prefix string) (*model.TestSuiteExecution, error) {
	id, err := s.getTestSuiteExecutionID(ctx, prefix)
	if err != nil {
		return nil, err
	}

	testSuiteExecution, err := s.testSuiteExecution.Delete(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, entityNotFoundError(model.EntityKindTestSuiteExecution, id, prefix)
		}
		if errors.Is(err, repository.ErrConflict) {
			return nil, entityConflictError(model.EntityKindTestSuiteExecution, id, prefix)
		}
		return nil, err
	}

	return newTestSuiteExecutionFromRepository(testSuiteExecution)
}

func newTestSuiteExecutionFromRepository(r *repository.TestSuiteExecution) (*model.TestSuiteExecution, error) {
	if r == nil {
		return nil, nil
	}

	tse := &model.TestSuiteExecution{
		ID:              r.ID,
		StartedAt:       r.StartedAt,
		FinishedAt:      r.FinishedAt,
		Status:          model.ExecutionStatus(r.Status),
		EnvironmentID:   r.EnvironmentID,
		EnvironmentName: r.EnvironmentName,
		TestSuiteID:     r.TestSuiteID,
		TestSuiteName:   r.TestSuiteName,
		TestsCount:      r.TestsCount,
	}

	if len(r.Summary) > 0 {
		var summary model.TestSuiteExecutionSummary
		if err := json.Unmarshal(r.Summary, &summary); err != nil {
			return nil, errorc.With(
				errors.ErrCannotParseTestSuiteExecutionSummary,
				errorc.Error(keys.Cause, err),
			)
		}
		tse.Summary = &summary
	}

	return tse, nil
}

func newTestSuiteExecutionSliceFromRepository(
	r []repository.TestSuiteExecution,
	restorePresentationOrder bool,
) ([]model.TestSuiteExecution, error) {
	return convertRepositorySlice(
		r,
		restorePresentationOrder,
		newTestSuiteExecutionFromRepository,
	)
}
