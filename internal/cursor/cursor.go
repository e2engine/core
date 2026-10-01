package cursor

import (
	"context"
	"encoding/base64"
	"encoding/json"

	"github.com/ygrebnov/errorc"
	modellib "github.com/ygrebnov/model"

	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/keys"
)

type Position string

const (
	PositionAfter  Position = "after"
	PositionBefore Position = "before"
)

const CurrentVersion uint8 = 1

type Payload struct {
	Version uint8 `json:"version" default:"1" validate:"min(1)"`
	// Value is the value of the field used for ordering.
	// Validation may need to be relaxed in the future, if we introduce ordering by optional fields.
	Value string `json:"value" validate:"min(1)"`
	// ID is a stable tie-breaker.
	ID             string               `json:"id" validate:"id"`
	Position       Position             `json:"position" validate:"oneof(after,before)"`
	OrderBy        model.OrderBy        `json:"order_by" default:"id"`
	OrderDirection model.OrderDirection `json:"order_direction" default:"asc" validate:"oneof(asc,desc)"`
}

func GetBinding() (*modellib.Binding[Payload], error) {
	return modellib.NewBinding[Payload](
		modellib.WithRules(
			model.GetValidationRules()...,
		),
	)
}

func Parse(ctx context.Context, binding *modellib.Binding[Payload], opaque string) (Payload, error) {
	data, err := base64.RawURLEncoding.DecodeString(opaque)
	if err != nil {
		return Payload{}, errorc.With(
			errors.ErrCannotDecodeCursor,
			errorc.Error(keys.Cause, err),
		)
	}

	var payload Payload
	if err := json.Unmarshal(data, &payload); err != nil {
		return Payload{}, errorc.With(
			errors.ErrCannotDecodeCursor,
			errorc.Error(keys.Cause, err),
		)
	}

	if payload.Version != CurrentVersion {
		return Payload{}, errorc.With(
			errors.ErrInvalidCursor,
			errorc.Int(keys.CursorVersion, int(payload.Version)),
			errorc.String(keys.Validation, "unsupported cursor version"),
		)
	}

	if err := binding.Validate(ctx, &payload); err != nil {
		return Payload{}, errorc.With(
			errors.ErrInvalidCursor,
			errorc.Error(keys.Cause, err),
		)
	}

	return payload, nil
}

func Encode(ctx context.Context, binding *modellib.Binding[Payload], payload Payload) (string, error) {
	if err := binding.ApplyDefaults(&payload); err != nil {
		return "", errorc.With(
			errors.ErrCannotComposeCursorPayload,
			errorc.Error(keys.Cause, err),
		)
	}

	if err := binding.Validate(ctx, &payload); err != nil {
		return "", errorc.With(
			errors.ErrInvalidCursor,
			errorc.Error(keys.Cause, err),
		)
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", errorc.With(
			errors.ErrCannotEncodeCursor,
			errorc.Error(keys.Cause, err),
		)
	}

	return base64.RawURLEncoding.EncodeToString(data), nil
}
