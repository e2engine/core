package render

import (
	"errors"
	"strings"

	"github.com/ygrebnov/errorc"
	"github.com/ygrebnov/model/validation"

	coreerrors "github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/keys"
)

// ValidationError formats a model validation error as a concise invalid
// spec message. It accepts errors wrapped by callers and falls back to the
// original error text when no structured spec validation is available.
func ValidationError(err error) string {
	if err == nil {
		return ""
	}

	var validationErr *validation.Error
	if !errors.Is(err, coreerrors.ErrInvalidSpec) ||
		!errors.As(err, &validationErr) || validationErr == nil {
		return err.Error()
	}

	kind, ok := errorc.Lookup[string](err, keys.SpecKind)
	if !ok {
		return err.Error()
	}

	paths := validationErr.Fields()
	if len(paths) == 0 {
		return err.Error()
	}

	messages := make([]string, 0, validationErr.Len())
	for _, path := range paths {
		for _, issue := range validationErr.ForField(path) {
			if issue.Err != nil {
				messages = append(messages, issue.Err.Error())
			}
		}
	}
	if len(messages) == 0 {
		return err.Error()
	}

	return errorc.With(
		coreerrors.ErrInvalidSpec,
		errorc.String(keys.SpecKind, kind),
		errorc.String(keys.Fields, strings.Join(paths, ", ")),
		errorc.String(keys.Validation, strings.Join(messages, ", ")),
	).Error()
}
