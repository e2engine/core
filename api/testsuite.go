package core

import (
	"context"
	"slices"

	"github.com/ygrebnov/errorc"
	modellib "github.com/ygrebnov/model"

	"github.com/e2engine/core/execute"
	"github.com/e2engine/core/internal/resource"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/keys"
)

const (
	requestKindGetTestSuitesPage = "GetTestSuitesPage"
	requestKindCreateTestSuite   = "CreateTestSuite"
	requestKindGetTestSuite      = "GetTestSuite"
	requestKindDeleteTestSuite   = "DeleteTestSuite"
)

type GetTestSuitesPageParams struct {
	PageSize int `json:"page_size" default:"10" validate:"min(1),max(100)"`
	// Cursor takes precedence over OrderBy+OrderDirection
	Cursor         string               `json:"cursor,omitempty"`
	OrderBy        model.OrderBy        `json:"order_by,omitempty" default:"updatedAt" validate:"omitempty,oneof(id,name,version,createdAt,updatedAt)"`
	OrderDirection model.OrderDirection `json:"order_direction,omitempty" default:"asc" validate:"omitempty,oneof(asc,desc)"`
}

func (s *Service) GetTestSuitesPage(ctx context.Context, params *GetTestSuitesPageParams) (*model.TestSuitesPage, error) {
	if params == nil {
		params = &GetTestSuitesPageParams{} // values will be populated from 'default' tags
	}

	err := modellib.ValidateWithDefaults(ctx, params, modellib.WithRules(model.GetValidationRules()...))
	if err != nil {
		return nil, errorc.With(
			errors.ErrInvalidRequestParameters,
			errorc.String(keys.RequestKind, requestKindGetTestSuitesPage),
			errorc.Error(keys.Validation, err),
		)
	}

	arg := &resource.GetTestSuitesPageParams{
		PagingParams: resource.PagingParams{
			PageSize:       params.PageSize,
			Cursor:         params.Cursor,
			OrderBy:        params.OrderBy,
			OrderDirection: params.OrderDirection,
		},
	}

	return s.resources.GetTestSuitesPage(ctx, arg)
}

func (s *Service) ValidateTestSuite(ctx context.Context, ts *model.TestSuite) error {
	return ts.Validate(ctx)
}

type CreateTestSuiteParams struct {
	Name        string              `json:"name"`
	Description string              `json:"description,omitempty"`
	Spec        model.TestSuiteSpec `json:"spec"`
}

func (s *Service) CreateTestSuite(ctx context.Context, params *CreateTestSuiteParams) (*model.TestSuite, error) {
	if params == nil {
		return nil, errorc.With(
			errors.ErrInvalidRequestParameters,
			errorc.String(keys.RequestKind, requestKindCreateTestSuite),
			errorc.String(keys.Validation, "missing parameters"),
		)
	}

	toValidate := &model.TestSuite{
		Kind:        model.ResourceKindTestSuite,
		Name:        params.Name,
		Description: params.Description,
		Version:     resource.InitialVersion,
		Spec:        params.Spec,
	}

	if err := s.ValidateTestSuite(ctx, toValidate); err != nil {
		return nil, errorc.With(
			errors.ErrInvalidRequestParameters,
			errorc.String(keys.RequestKind, requestKindCreateTestSuite),
			errorc.Error(keys.Validation, err),
		)
	}

	arg := &resource.CreateTestSuiteParams{
		Name:        params.Name,
		Description: params.Description,
		Spec:        params.Spec,
	}

	return s.resources.CreateTestSuite(ctx, arg)
}

func (s *Service) GetTestSuite(ctx context.Context, ref string) (*model.TestSuite, error) {
	if err := validateReference(ref, requestKindGetTestSuite); err != nil {
		return nil, err
	}

	return s.resources.GetTestSuite(ctx, ref)
}

func (s *Service) DeleteTestSuite(ctx context.Context, ref string) (*model.TestSuite, error) {
	if err := validateReference(ref, requestKindDeleteTestSuite); err != nil {
		return nil, err
	}

	return s.resources.DeleteTestSuite(ctx, ref)
}

type TestSuiteRunRequest struct {
	EnvironmentRef string `json:"environment_ref" yaml:"environment_ref"`
	TestSuiteRef   string `json:"testsuite_ref" yaml:"testsuite_ref"`
}

func (s *Service) RunTestSuite(ctx context.Context, req TestSuiteRunRequest) (string, error) {
	env, err := s.GetEnvironment(ctx, req.EnvironmentRef)
	if err != nil {
		return "", err
	}

	testSuite, err := s.GetTestSuite(ctx, req.TestSuiteRef)
	if err != nil {
		return "", err
	}

	tests, err := s.resolveTestSuiteTests(ctx, testSuite.Spec.Selectors)
	if err != nil {
		return "", err
	}

	arg := execute.TestSuiteRunRequest{
		Environment: env,
		TestSuite:   testSuite,
		Tests:       tests,
	}

	return s.launcher.StartTestSuite(ctx, arg)
}

func (s *Service) resolveTestSuiteTests(
	ctx context.Context,
	selectors model.TestSelectorsSpec,
) ([]model.Test, error) {
	tests := make(map[string]model.Test)

	for _, ref := range selectors.IDs {
		test, err := s.GetTest(ctx, ref)
		if err != nil {
			return nil, err
		}

		tests[test.ID] = *test
	}

	for _, ref := range selectors.Names {
		test, err := s.GetTest(ctx, ref)
		if err != nil {
			return nil, err
		}

		tests[test.ID] = *test
	}

	for _, tag := range selectors.Tags {
		matched, err := s.resources.GetTestsByTag(ctx, tag)
		if err != nil {
			return nil, err
		}

		for i := range matched {
			tests[matched[i].ID] = matched[i]
		}
	}

	if len(tests) == 0 {
		return nil, errors.ErrNoTestsResolved
	}

	if len(tests) > s.runtimeCfg.MaxResolvedTests {
		return nil, errorc.With(
			errors.ErrExceededTestsLimit,
			errorc.Int(keys.TestSuiteMaxTestsNumber, s.runtimeCfg.MaxResolvedTests),
			errorc.Int(keys.TestSuiteResolvedTestsNumber, len(tests)),
		)
	}

	ids := make([]string, 0, len(tests))
	for id := range tests {
		ids = append(ids, id)
	}

	slices.Sort(ids)

	result := make([]model.Test, 0, len(ids))
	for _, id := range ids {
		result = append(result, tests[id])
	}

	return result, nil
}
