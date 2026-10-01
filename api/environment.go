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
	requestKindGetEnvironmentsPage = "GetEnvironmentsPage"
	requestKindCreateEnvironment   = "CreateEnvironment"
	requestKindGetEnvironment      = "GetEnvironment"
	requestKindDeleteEnvironment   = "DeleteEnvironment"
)

type GetEnvironmentsPageParams struct {
	PageSize int `json:"page_size" default:"10" validate:"min(1),max(100)"`
	// Cursor takes precedence over OrderBy+OrderDirection
	Cursor         string               `json:"cursor,omitempty"`
	OrderBy        model.OrderBy        `json:"order_by,omitempty" default:"updatedAt" validate:"omitempty,oneof(id,name,version,createdAt,updatedAt)"`
	OrderDirection model.OrderDirection `json:"order_direction,omitempty" default:"asc" validate:"omitempty,oneof(asc,desc)"`
}

func (s *Service) GetEnvironmentsPage(ctx context.Context, params *GetEnvironmentsPageParams) (*model.EnvironmentsPage, error) {
	if params == nil {
		params = &GetEnvironmentsPageParams{} // values will be populated from 'default' tags
	}

	err := modellib.ValidateWithDefaults(ctx, params, modellib.WithRules(model.GetValidationRules()...))
	if err != nil {
		return nil, errorc.With(
			errors.ErrInvalidRequestParameters,
			errorc.String(keys.RequestKind, requestKindGetEnvironmentsPage),
			errorc.Error(keys.Validation, err),
		)
	}

	arg := &resource.GetEnvironmentsPageParams{
		PagingParams: resource.PagingParams{
			PageSize:       params.PageSize,
			Cursor:         params.Cursor,
			OrderBy:        params.OrderBy,
			OrderDirection: params.OrderDirection,
		},
	}

	return s.resources.GetEnvironmentsPage(ctx, arg)
}

func (s *Service) ValidateEnvironment(ctx context.Context, env *model.Environment) error {
	return env.Validate(ctx)
}

type CreateEnvironmentParams struct {
	Name        string                `json:"name"`
	Description string                `json:"description,omitempty"`
	Spec        model.EnvironmentSpec `json:"spec"`
}

func (s *Service) CreateEnvironment(ctx context.Context, params *CreateEnvironmentParams) (*model.Environment, error) {
	if params == nil {
		return nil, errorc.With(
			errors.ErrInvalidRequestParameters,
			errorc.String(keys.RequestKind, requestKindCreateEnvironment),
			errorc.String(keys.Validation, "missing parameters"),
		)
	}

	toValidate := &model.Environment{
		Kind:        model.ResourceKindEnvironment,
		Name:        params.Name,
		Description: params.Description,
		Version:     resource.InitialVersion,
		Spec:        params.Spec,
	}

	if err := s.ValidateEnvironment(ctx, toValidate); err != nil {
		return nil, errorc.With(
			errors.ErrInvalidRequestParameters,
			errorc.String(keys.RequestKind, requestKindCreateEnvironment),
			errorc.Error(keys.Validation, err),
		)
	}

	arg := &resource.CreateEnvironmentParams{
		Name:        params.Name,
		Description: params.Description,
		Spec:        params.Spec,
	}

	return s.resources.CreateEnvironment(ctx, arg)
}

func (s *Service) GetEnvironment(ctx context.Context, ref string) (*model.Environment, error) {
	if err := validateReference(ref, requestKindGetEnvironment); err != nil {
		return nil, err
	}

	return s.resources.GetEnvironment(ctx, ref)
}

func (s *Service) DeleteEnvironment(ctx context.Context, ref string) (*model.Environment, error) {
	if err := validateReference(ref, requestKindDeleteEnvironment); err != nil {
		return nil, err
	}

	return s.resources.DeleteEnvironment(ctx, ref)
}
