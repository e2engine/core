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
	requestKindGetTestExecutionsPage  = "GetTestExecutionsPage"
	requestKindGetTestExecution       = "GetTestExecution"
	requestKindGetTestExecutionStatus = "GetTestExecutionStatus"
	requestKindDeleteTestExecution    = "DeleteTestExecution"
)

type GetTestExecutionsPageParams struct {
	PageSize int `json:"page_size" default:"10" validate:"min(1),max(100)"`
	// Cursor takes precedence over OrderBy+OrderDirection
	Cursor         string               `json:"cursor,omitempty"`
	OrderBy        model.OrderBy        `json:"order_by,omitempty" default:"startedAt" validate:"omitempty,oneof(id,startedAt,finishedAt,status)"`
	OrderDirection model.OrderDirection `json:"order_direction,omitempty" default:"desc" validate:"omitempty,oneof(asc,desc)"`
}

func (s *Service) GetTestExecutionsPage(ctx context.Context, params *GetTestExecutionsPageParams) (*model.TestExecutionsPage, error) {
	if params == nil {
		params = &GetTestExecutionsPageParams{} // values will be populated from 'default' tags
	}

	err := modellib.ValidateWithDefaults(ctx, params, modellib.WithRules(model.GetValidationRules()...))
	if err != nil {
		return nil, errorc.With(
			errors.ErrInvalidRequestParameters,
			errorc.String(keys.RequestKind, requestKindGetTestExecutionsPage),
			errorc.Error(keys.Validation, err),
		)
	}

	arg := &resource.GetTestExecutionsPageParams{
		PagingParams: resource.PagingParams{
			PageSize:       params.PageSize,
			Cursor:         params.Cursor,
			OrderBy:        params.OrderBy,
			OrderDirection: params.OrderDirection,
		},
	}

	return s.resources.GetTestExecutionsPage(ctx, arg)
}

func (s *Service) GetTestExecution(ctx context.Context, ref string) (*model.TestExecution, error) {
	if err := validateReference(ref, requestKindGetTestExecution); err != nil {
		return nil, err
	}

	return s.resources.GetTestExecution(ctx, ref)
}

func (s *Service) GetTestExecutionStatus(ctx context.Context, ref string) (model.ExecutionStatus, error) {
	if err := validateReference(ref, requestKindGetTestExecutionStatus); err != nil {
		return "", err
	}

	return s.resources.GetTestExecutionStatus(ctx, ref)
}

func (s *Service) DeleteTestExecution(ctx context.Context, ref string) (*model.TestExecution, error) {
	if err := validateReference(ref, requestKindDeleteTestExecution); err != nil {
		return nil, err
	}

	return s.resources.DeleteTestExecution(ctx, ref)
}
