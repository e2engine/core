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

type GetTestsPageParams struct {
	PagingParams
}

const RequestKindGetTestsPage = "GetTestsPage"

func (s *Controller) GetTestsPage(ctx context.Context, params *GetTestsPageParams) (*model.TestsPage, error) {
	paging, err := s.resolvePaging(
		ctx, params.PagingParams, RequestKindGetTestsPage, isResourceOrderBy,
	)
	if err != nil {
		return nil, err
	}

	anchor, err := decodePagingAnchor(
		paging.Cursor, RequestKindGetTestsPage, isResourceTimestampOrderBy,
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

	list, err := s.test.List(ctx, arg)
	if err != nil {
		return nil, err
	}

	return buildPage(
		ctx, s, list, params.PageSize, anchor, paging,
		newTestSliceFromRepository, getTestCursorValue,
		func(test model.Test) string { return test.ID },
	)
}

func getTestCursorValue(test *model.Test, field model.OrderBy) string {
	switch field {
	case model.OrderByID:
		return test.ID
	case model.OrderByName:
		return test.Name
	case model.OrderByVersion:
		return test.Version
	case model.OrderByCreatedAt:
		return test.CreatedAt.Format(time.RFC3339Nano)
	case model.OrderByUpdatedAt:
		return test.UpdatedAt.Format(time.RFC3339Nano)
	default:
		panic("unknown test cursor field")
	}
}

type CreateTestParams struct {
	Name        string         `json:"name" validate:"min(3),max(200)"`
	Description string         `json:"description,omitempty" validate:"omitempty,min(3),max(2000)"`
	Spec        model.TestSpec `json:"spec" validate:"dive"`
}

func (s *Controller) CreateTest(ctx context.Context, params *CreateTestParams) (*model.Test, error) {
	specJSON, err := json.Marshal(params.Spec)
	if err != nil {
		return nil, errorc.With(
			errors.ErrCannotConvertSpecToJSON,
			errorc.String(keys.EntityKind, string(model.EntityKindTest)),
			errorc.Error(keys.Cause, err),
		)
	}

	id := idpkg.GenerateRandomID()
	now := time.Now().UTC()

	arg := &repository.Test{
		ID:          id,
		Version:     InitialVersion,
		Name:        params.Name,
		Description: params.Description,
		Spec:        specJSON,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	created, err := s.test.Create(ctx, arg)
	if err != nil {
		if errors.Is(err, repository.ErrAlreadyExists) {
			return nil, entityAlreadyExistsError(model.EntityKindTest, id, params.Name)
		}

		return nil, err
	}

	return newTestFromRepository(created)
}

func (s *Controller) GetTest(ctx context.Context, prefix string) (*model.Test, error) {
	id, err := s.getTestID(ctx, prefix)
	if err != nil {
		return nil, err
	}

	test, err := s.test.Get(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, entityNotFoundError(model.EntityKindTest, id, prefix)
		}
		return nil, err
	}

	return newTestFromRepository(test)
}

func (s *Controller) getTestID(
	ctx context.Context,
	ref string,
) (string, error) {
	return resolveResourceID(
		ctx,
		s.test,
		ref,
		model.EntityKindTest,
	)
}

func (s *Controller) GetTestsByTag(ctx context.Context, tag string) ([]model.Test, error) {
	tests, err := s.test.GetByTag(ctx, tag)
	if err != nil {
		return nil, err
	}

	return newTestSliceFromRepository(tests, false)
}

func (s *Controller) DeleteTest(ctx context.Context, prefix string) (*model.Test, error) {
	id, err := s.getTestID(ctx, prefix)
	if err != nil {
		return nil, err
	}

	test, err := s.test.Delete(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, entityNotFoundError(model.EntityKindTest, id, prefix)
		}
		if errors.Is(err, repository.ErrConflict) {
			return nil, entityConflictError(model.EntityKindTest, id, prefix)
		}
		return nil, err
	}

	return newTestFromRepository(test)
}

func newTestFromRepository(r *repository.Test) (*model.Test, error) {
	if r == nil {
		return nil, nil
	}

	var spec model.TestSpec
	if err := json.Unmarshal(r.Spec, &spec); err != nil {
		return nil, errorc.With(errors.ErrCannotParseSpec, errorc.Error(keys.Cause, err))
	}

	return &model.Test{
		Kind:        model.ResourceKindTest,
		ID:          r.ID,
		Version:     r.Version,
		Name:        r.Name,
		Description: r.Description,
		Spec:        spec,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}, nil
}

func newTestSliceFromRepository(
	r []repository.Test,
	restorePresentationOrder bool,
) ([]model.Test, error) {
	return convertRepositorySlice(
		r,
		restorePresentationOrder,
		newTestFromRepository,
	)
}
