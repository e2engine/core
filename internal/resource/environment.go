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

type GetEnvironmentsPageParams struct {
	PagingParams
}

const RequestKindGetEnvironmentsPage = "GetEnvironmentsPage"

func (s *Controller) GetEnvironmentsPage(
	ctx context.Context,
	params *GetEnvironmentsPageParams,
) (*model.EnvironmentsPage, error) {
	paging, err := s.resolvePaging(
		ctx, params.PagingParams, RequestKindGetEnvironmentsPage, isResourceOrderBy,
	)
	if err != nil {
		return nil, err
	}

	anchor, err := decodePagingAnchor(
		paging.Cursor, RequestKindGetEnvironmentsPage, isResourceTimestampOrderBy,
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

	list, err := s.environment.List(ctx, arg)
	if err != nil {
		return nil, err
	}

	return buildPage(
		ctx, s, list, params.PageSize, anchor, paging,
		newEnvironmentSliceFromRepository, getEnvironmentCursorValue,
		func(environment model.Environment) string { return environment.ID },
	)
}

func getEnvironmentCursorValue(env *model.Environment, field model.OrderBy) string {
	switch field {
	case model.OrderByID:
		return env.ID
	case model.OrderByName:
		return env.Name
	case model.OrderByVersion:
		return env.Version
	case model.OrderByCreatedAt:
		return env.CreatedAt.Format(time.RFC3339Nano)
	case model.OrderByUpdatedAt:
		return env.UpdatedAt.Format(time.RFC3339Nano)
	default:
		panic("unknown environment cursor field")
	}
}

type CreateEnvironmentParams struct {
	Name        string                `json:"name" validate:"min(3),max(200)"`
	Description string                `json:"description,omitempty" validate:"omitempty,min(3),max(2000)"`
	Spec        model.EnvironmentSpec `json:"spec" validate:"dive"`
}

func (s *Controller) CreateEnvironment(
	ctx context.Context,
	params *CreateEnvironmentParams,
) (*model.Environment, error) {
	specJSON, err := json.Marshal(params.Spec)
	if err != nil {
		return nil, errorc.With(
			errors.ErrCannotConvertSpecToJSON,
			errorc.String(keys.EntityKind, string(model.EntityKindEnvironment)),
			errorc.Error(keys.Cause, err),
		)
	}

	id := idpkg.GenerateRandomID()
	now := time.Now().UTC()

	arg := &repository.Environment{
		ID:          id,
		Version:     InitialVersion,
		Name:        params.Name,
		Description: params.Description,
		Spec:        specJSON,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	created, err := s.environment.Create(ctx, arg)
	if err != nil {
		if errors.Is(err, repository.ErrAlreadyExists) {
			return nil, entityAlreadyExistsError(model.EntityKindEnvironment, id, params.Name)
		}

		return nil, err
	}

	return newEnvironmentFromRepository(created)
}

func (s *Controller) GetEnvironment(ctx context.Context, prefix string) (*model.Environment, error) {
	id, err := s.getEnvironmentID(ctx, prefix)
	if err != nil {
		return nil, err
	}

	environment, err := s.environment.Get(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, entityNotFoundError(model.EntityKindEnvironment, id, prefix)
		}
		return nil, err
	}

	return newEnvironmentFromRepository(environment)
}

func (s *Controller) DeleteEnvironment(ctx context.Context, prefix string) (*model.Environment, error) {
	id, err := s.getEnvironmentID(ctx, prefix)
	if err != nil {
		return nil, err
	}

	environment, err := s.environment.Delete(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, entityNotFoundError(model.EntityKindEnvironment, id, prefix)
		}
		if errors.Is(err, repository.ErrConflict) {
			return nil, entityConflictError(model.EntityKindEnvironment, id, prefix)
		}
		return nil, err
	}

	return newEnvironmentFromRepository(environment)
}

func (s *Controller) getEnvironmentID(
	ctx context.Context,
	ref string,
) (string, error) {
	return resolveResourceID(
		ctx,
		s.environment,
		ref,
		model.EntityKindEnvironment,
	)
}

func newEnvironmentFromRepository(r *repository.Environment) (*model.Environment, error) {
	if r == nil {
		return nil, nil
	}

	var spec model.EnvironmentSpec
	if err := json.Unmarshal(r.Spec, &spec); err != nil {
		return nil, errorc.With(errors.ErrCannotParseSpec, errorc.Error(keys.Cause, err))
	}

	return &model.Environment{
		Kind:        model.ResourceKindEnvironment,
		ID:          r.ID,
		Version:     r.Version,
		Name:        r.Name,
		Description: r.Description,
		Spec:        spec,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}, nil
}

func newEnvironmentSliceFromRepository(
	r []repository.Environment,
	restorePresentationOrder bool,
) ([]model.Environment, error) {
	return convertRepositorySlice(
		r,
		restorePresentationOrder,
		newEnvironmentFromRepository,
	)
}
