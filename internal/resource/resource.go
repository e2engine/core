package resource

import (
	"context"
	"strings"

	"github.com/ygrebnov/errorc"

	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/keys"
)

type resourceReferenceRepository interface {
	GetIDsByID(
		ctx context.Context,
		id string,
	) ([]string, error)

	GetIDsByName(
		ctx context.Context,
		name string,
	) ([]string, error)
}

func resolveResourceID(
	ctx context.Context,
	repo resourceReferenceRepository,
	ref string,
	kind model.EntityKind,
) (string, error) {
	ids, err := repo.GetIDsByID(ctx, ref)
	if err != nil {
		return "", err
	}

	if len(ids) == 1 {
		return ids[0], nil
	}

	nameIDs, err := repo.GetIDsByName(ctx, ref)
	if err != nil {
		return "", err
	}

	if len(nameIDs) == 1 {
		return nameIDs[0], nil
	}

	if len(ids) > 1 {
		return "", errorc.With(
			errors.ErrEntityPrefixAmbiguous,
			errorc.String(keys.EntityID, ref),
			errorc.String(keys.EntityKind, string(kind)),
			errorc.String(keys.EntityIDs, strings.Join(ids, ",")),
		)
	}

	if len(nameIDs) > 1 {
		return "", errorc.With(
			errors.ErrEntityPrefixAmbiguous,
			errorc.String(keys.EntityName, ref),
			errorc.String(keys.EntityKind, string(kind)),
			errorc.String(keys.EntityIDs, strings.Join(nameIDs, ",")),
		)
	}

	return "", errorc.With(
		errors.ErrEntityNotFound,
		errorc.String(keys.EntityKind, string(kind)),
		errorc.String(keys.EntityRef, ref),
	)
}

type entityReferenceRepository interface {
	GetIDsByID(
		ctx context.Context,
		id string,
	) ([]string, error)
}

func resolveEntityID(
	ctx context.Context,
	repo entityReferenceRepository,
	ref string,
	kind model.EntityKind,
) (string, error) {
	ids, err := repo.GetIDsByID(ctx, ref)
	if err != nil {
		return "", err
	}

	switch len(ids) {
	case 0:
		return "", errorc.With(
			errors.ErrEntityNotFound,
			errorc.String(keys.EntityKind, string(kind)),
			errorc.String(keys.EntityRef, ref),
		)

	case 1:
		return ids[0], nil

	default:
		return "", errorc.With(
			errors.ErrEntityPrefixAmbiguous,
			errorc.String(keys.EntityID, ref),
			errorc.String(keys.EntityKind, string(kind)),
			errorc.String(keys.EntityIDs, strings.Join(ids, ",")),
		)
	}
}

func entityNotFoundError(
	kind model.EntityKind,
	id string,
	ref string,
) error {
	return errorc.With(
		errors.ErrEntityNotFound,
		errorc.String(keys.EntityID, id),
		errorc.String(keys.EntityRef, ref),
		errorc.String(keys.EntityKind, string(kind)),
	)
}

func entityConflictError(
	kind model.EntityKind,
	id string,
	ref string,
) error {
	return errorc.With(
		errors.ErrEntityConflict,
		errorc.String(keys.EntityID, id),
		errorc.String(keys.EntityRef, ref),
		errorc.String(keys.EntityKind, string(kind)),
	)
}

func entityAlreadyExistsError(
	kind model.EntityKind,
	id string,
	name string,
) error {
	return errorc.With(
		errors.ErrEntityAlreadyExists,
		errorc.String(keys.EntityID, id),
		errorc.String(keys.EntityName, name),
		errorc.String(keys.EntityKind, string(kind)),
	)
}

func isResourceOrderBy(orderBy model.OrderBy) bool {
	switch orderBy {
	case model.OrderByID,
		model.OrderByName,
		model.OrderByVersion,
		model.OrderByCreatedAt,
		model.OrderByUpdatedAt:
		return true
	default:
		return false
	}
}

func isResourceTimestampOrderBy(orderBy model.OrderBy) bool {
	switch orderBy {
	case model.OrderByCreatedAt,
		model.OrderByUpdatedAt:
		return true
	default:
		return false
	}
}

func isExecutionOrderBy(orderBy model.OrderBy) bool {
	switch orderBy {
	case model.OrderByID,
		model.OrderByStartedAt,
		model.OrderByFinishedAt,
		model.OrderByStatus:
		return true
	default:
		return false
	}
}

func isExecutionTimestampOrderBy(orderBy model.OrderBy) bool {
	switch orderBy {
	case model.OrderByStartedAt,
		model.OrderByFinishedAt:
		return true
	default:
		return false
	}
}

func convertRepositorySlice[R any, M any](
	items []R,
	reverse bool,
	convert func(*R) (*M, error),
) ([]M, error) {
	result := make([]M, 0, len(items))

	for i := range items {
		index := i
		if reverse {
			index = len(items) - 1 - i
		}

		item, err := convert(&items[index])
		if err != nil {
			return nil, err
		}

		result = append(result, *item)
	}

	return result, nil
}
