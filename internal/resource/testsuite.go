package resource

import (
	"context"
	"encoding/json"
	"time"

	"github.com/ygrebnov/errorc"

	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/errors"
	idpkg "github.com/e2engine/core/pkg/id"
	"github.com/e2engine/core/pkg/keys"
	"github.com/e2engine/core/repository"
)

type GetTestSuitesPageParams struct {
	PagingParams
}

const RequestKindGetTestSuitesPage = "GetTestSuitesPage"

func (s *Controller) GetTestSuitesPage(
	ctx context.Context,
	params *GetTestSuitesPageParams,
) (*model.TestSuitesPage, error) {
	paging, err := s.resolvePaging(
		ctx, params.PagingParams, RequestKindGetTestSuitesPage, isResourceOrderBy,
	)
	if err != nil {
		return nil, err
	}

	anchor, err := decodePagingAnchor(
		paging.Cursor, RequestKindGetTestSuitesPage, isResourceTimestampOrderBy,
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

	list, err := s.testSuite.List(ctx, arg)
	if err != nil {
		return nil, err
	}

	return buildPage(
		ctx, s, list, params.PageSize, anchor, paging,
		newTestSuiteSliceFromRepository, getTestSuiteCursorValue,
		func(testSuite model.TestSuite) string { return testSuite.ID },
	)
}

func getTestSuiteCursorValue(testSuite *model.TestSuite, field model.OrderBy) string {
	switch field {
	case model.OrderByID:
		return testSuite.ID
	case model.OrderByName:
		return testSuite.Name
	case model.OrderByVersion:
		return testSuite.Version
	case model.OrderByCreatedAt:
		return testSuite.CreatedAt.Format(time.RFC3339Nano)
	case model.OrderByUpdatedAt:
		return testSuite.UpdatedAt.Format(time.RFC3339Nano)
	default:
		panic("unknown test suite cursor field")
	}
}

type CreateTestSuiteParams struct {
	Name        string              `json:"name" validate:"min(3),max(200)"`
	Description string              `json:"description,omitempty" validate:"omitempty,min(3),max(2000)"`
	Spec        model.TestSuiteSpec `json:"spec" validate:"dive"`
}

func (s *Controller) CreateTestSuite(ctx context.Context, params *CreateTestSuiteParams) (*model.TestSuite, error) {
	specJSON, err := json.Marshal(params.Spec)
	if err != nil {
		return nil, errorc.With(
			errors.ErrCannotConvertSpecToJSON,
			errorc.String(keys.EntityKind, string(model.EntityKindTestSuite)),
			errorc.Error(keys.Cause, err),
		)
	}

	id := idpkg.GenerateRandomID()
	now := time.Now().UTC()

	arg := &repository.TestSuite{
		ID:          id,
		Version:     InitialVersion,
		Name:        params.Name,
		Description: params.Description,
		Spec:        specJSON,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	created, err := s.testSuite.Create(ctx, arg)
	if err != nil {
		if errors.Is(err, repository.ErrAlreadyExists) {
			return nil, entityAlreadyExistsError(model.EntityKindTestSuite, id, params.Name)
		}

		return nil, err
	}

	return newTestSuiteFromRepository(created)
}

func (s *Controller) GetTestSuite(ctx context.Context, prefix string) (*model.TestSuite, error) {
	id, err := s.getTestSuiteID(ctx, prefix)
	if err != nil {
		return nil, err
	}

	testSuite, err := s.testSuite.Get(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, entityNotFoundError(model.EntityKindTestSuite, id, prefix)
		}
		return nil, err
	}

	return newTestSuiteFromRepository(testSuite)
}

func (s *Controller) getTestSuiteID(
	ctx context.Context,
	ref string,
) (string, error) {
	return resolveResourceID(
		ctx,
		s.testSuite,
		ref,
		model.EntityKindTestSuite,
	)
}

func (s *Controller) DeleteTestSuite(ctx context.Context, prefix string) (*model.TestSuite, error) {
	id, err := s.getTestSuiteID(ctx, prefix)
	if err != nil {
		return nil, err
	}

	testSuite, err := s.testSuite.Delete(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, entityNotFoundError(model.EntityKindTestSuite, id, prefix)
		}
		if errors.Is(err, repository.ErrConflict) {
			return nil, entityConflictError(model.EntityKindTestSuite, id, prefix)
		}
		return nil, err
	}

	return newTestSuiteFromRepository(testSuite)
}

func newTestSuiteFromRepository(r *repository.TestSuite) (*model.TestSuite, error) {
	if r == nil {
		return nil, nil
	}

	var spec model.TestSuiteSpec
	if err := json.Unmarshal(r.Spec, &spec); err != nil {
		return nil, errorc.With(errors.ErrCannotParseSpec, errorc.Error(keys.Cause, err))
	}

	return &model.TestSuite{
		Kind:        model.ResourceKindTestSuite,
		ID:          r.ID,
		Version:     r.Version,
		Name:        r.Name,
		Description: r.Description,
		Spec:        spec,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}, nil
}

func newTestSuiteSliceFromRepository(
	r []repository.TestSuite,
	restorePresentationOrder bool,
) ([]model.TestSuite, error) {
	return convertRepositorySlice(
		r,
		restorePresentationOrder,
		newTestSuiteFromRepository,
	)
}
