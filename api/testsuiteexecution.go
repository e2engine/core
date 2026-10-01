package core

import (
	"context"

	"github.com/ygrebnov/errorc"
	modellib "github.com/ygrebnov/model"

	"github.com/e2engine/core/internal/resource"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/keys"
)

const (
	requestKindGetTestSuiteExecutionsPage  = "GetTestSuiteExecutionsPage"
	requestKindGetTestSuiteExecution       = "GetTestSuiteExecution"
	requestKindGetTestSuiteExecutionStatus = "GetTestSuiteExecutionStatus"
	requestKindDeleteTestSuiteExecution    = "DeleteTestSuiteExecution"
)

type GetTestSuiteExecutionsPageParams struct {
	PageSize int `json:"page_size" default:"10" validate:"min(1),max(100)"`
	// Cursor takes precedence over OrderBy+OrderDirection
	Cursor         string               `json:"cursor,omitempty"`
	OrderBy        model.OrderBy        `json:"order_by,omitempty" default:"startedAt" validate:"omitempty,oneof(id,startedAt,finishedAt,status)"`
	OrderDirection model.OrderDirection `json:"order_direction,omitempty" default:"desc" validate:"omitempty,oneof(asc,desc)"`
}

func (s *Service) GetTestSuiteExecutionsPage(
	ctx context.Context,
	params *GetTestSuiteExecutionsPageParams,
) (*model.TestSuiteExecutionsPage, error) {
	if params == nil {
		params = &GetTestSuiteExecutionsPageParams{} // values will be populated from 'default' tags
	}

	err := modellib.ValidateWithDefaults(ctx, params, modellib.WithRules(model.GetValidationRules()...))
	if err != nil {
		return nil, errorc.With(
			errors.ErrInvalidRequestParameters,
			errorc.String(keys.RequestKind, requestKindGetTestSuiteExecutionsPage),
			errorc.Error(keys.Validation, err),
		)
	}

	arg := &resource.GetTestSuiteExecutionsPageParams{
		PagingParams: resource.PagingParams{
			PageSize:       params.PageSize,
			Cursor:         params.Cursor,
			OrderBy:        params.OrderBy,
			OrderDirection: params.OrderDirection,
		},
	}

	return s.resources.GetTestSuiteExecutionsPage(ctx, arg)
}

func (s *Service) GetTestSuiteExecution(ctx context.Context, ref string) (*model.TestSuiteExecution, error) {
	if err := validateReference(ref, requestKindGetTestSuiteExecution); err != nil {
		return nil, err
	}

	return s.resources.GetTestSuiteExecution(ctx, ref)
}

func (s *Service) GetTestSuiteExecutionStatus(ctx context.Context, ref string) (model.ExecutionStatus, error) {
	if err := validateReference(ref, requestKindGetTestSuiteExecutionStatus); err != nil {
		return "", err
	}

	return s.resources.GetTestSuiteExecutionStatus(ctx, ref)
}

func (s *Service) DeleteTestSuiteExecution(ctx context.Context, ref string) (*model.TestSuiteExecution, error) {
	if err := validateReference(ref, requestKindDeleteTestSuiteExecution); err != nil {
		return nil, err
	}

	return s.resources.DeleteTestSuiteExecution(ctx, ref)
}
