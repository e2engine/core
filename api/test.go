package core

import (
	"context"

	"github.com/ygrebnov/errorc"
	modellib "github.com/ygrebnov/model"

	"github.com/e2engine/core/execute"
	"github.com/e2engine/core/internal/resource"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/keys"
)

const (
	requestKindGetTestsPage = "GetTestsPage"
	requestKindCreateTest   = "CreateTest"
	requestKindGetTest      = "GetTest"
	requestKindDeleteTest   = "DeleteTest"
)

type GetTestsPageParams struct {
	PageSize int `json:"page_size" default:"10" validate:"min(1),max(100)"`
	// Cursor takes precedence over OrderBy+OrderDirection
	Cursor         string               `json:"cursor,omitempty"`
	OrderBy        model.OrderBy        `json:"order_by,omitempty" default:"updatedAt" validate:"omitempty,oneof(id,name,version,createdAt,updatedAt)"`
	OrderDirection model.OrderDirection `json:"order_direction,omitempty" default:"asc" validate:"omitempty,oneof(asc,desc)"`
}

func (s *Service) GetTestsPage(ctx context.Context, params *GetTestsPageParams) (*model.TestsPage, error) {
	if params == nil {
		params = &GetTestsPageParams{} // values will be populated from 'default' tags
	}

	err := modellib.ValidateWithDefaults(ctx, params, modellib.WithRules(model.GetValidationRules()...))
	if err != nil {
		return nil, errorc.With(
			errors.ErrInvalidRequestParameters,
			errorc.String(keys.RequestKind, requestKindGetTestsPage),
			errorc.Error(keys.Validation, err),
		)
	}

	arg := &resource.GetTestsPageParams{
		PagingParams: resource.PagingParams{
			PageSize:       params.PageSize,
			Cursor:         params.Cursor,
			OrderBy:        params.OrderBy,
			OrderDirection: params.OrderDirection,
		},
	}

	return s.resources.GetTestsPage(ctx, arg)
}

func (s *Service) ValidateTest(ctx context.Context, test *model.Test) error {
	return test.Validate(ctx)
}

type CreateTestParams struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Spec        model.TestSpec `json:"spec"`
}

func (s *Service) CreateTest(ctx context.Context, params *CreateTestParams) (*model.Test, error) {
	if params == nil {
		return nil, errorc.With(
			errors.ErrInvalidRequestParameters,
			errorc.String(keys.RequestKind, requestKindCreateTest),
			errorc.String(keys.Validation, "missing parameters"),
		)
	}

	toValidate := &model.Test{
		Kind:        model.ResourceKindTest,
		Name:        params.Name,
		Description: params.Description,
		Version:     resource.InitialVersion,
		Spec:        params.Spec,
	}

	if err := s.ValidateTest(ctx, toValidate); err != nil {
		return nil, errorc.With(
			errors.ErrInvalidRequestParameters,
			errorc.String(keys.RequestKind, requestKindCreateTest),
			errorc.Error(keys.Validation, err),
		)
	}

	arg := &resource.CreateTestParams{
		Name:        params.Name,
		Description: params.Description,
		Spec:        params.Spec,
	}

	return s.resources.CreateTest(ctx, arg)
}

func (s *Service) GetTest(ctx context.Context, ref string) (*model.Test, error) {
	if err := validateReference(ref, requestKindGetTest); err != nil {
		return nil, err
	}

	return s.resources.GetTest(ctx, ref)
}

func (s *Service) DeleteTest(ctx context.Context, ref string) (*model.Test, error) {
	if err := validateReference(ref, requestKindDeleteTest); err != nil {
		return nil, err
	}

	return s.resources.DeleteTest(ctx, ref)
}

type TestRunRequest struct {
	EnvironmentRef string `json:"environment_ref" yaml:"environment_ref"`
	TestRef        string `json:"test_ref" yaml:"test_ref"`
}

func (s *Service) RunTest(ctx context.Context, req TestRunRequest) (string, error) {
	env, err := s.GetEnvironment(ctx, req.EnvironmentRef)
	if err != nil {
		return "", err
	}

	test, err := s.GetTest(ctx, req.TestRef)
	if err != nil {
		return "", err
	}

	arg := execute.TestRunRequest{
		Environment: env,
		Test:        test,
	}

	return s.launcher.StartTest(ctx, arg)
}
