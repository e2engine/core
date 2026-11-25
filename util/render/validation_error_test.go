package render

import (
	"errors"
	"testing"

	"github.com/ygrebnov/errorc"
	"github.com/ygrebnov/model/validation"

	"github.com/e2engine/core/model"
	coreerrors "github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/keys"
)

func TestValidationError(t *testing.T) {
	validationErr := &validation.Error{}
	validationErr.Addf("Name", "min", errors.New("too short"))
	validationErr.Addf("Port", "min", errors.New("too small"))
	validationErr.Addf("Name", "max", errors.New("too long"))

	err := errorc.With(
		coreerrors.ErrInvalidSpec,
		errorc.String(keys.SpecKind, string(model.EntityKindEnvironment)),
		errorc.Error(keys.Validation, validationErr),
	)
	if !errors.Is(err, coreerrors.ErrInvalidSpec) {
		t.Fatalf("error = %v, want ErrInvalidSpec", err)
	}
	var got *validation.Error
	if !errors.As(err, &got) || got != validationErr {
		t.Fatalf("error = %v, want original *validation.Error in chain", err)
	}
	if value, ok := errorc.Lookup[error](err, keys.Validation); !ok || value != validationErr {
		t.Fatalf("validation field = %v, %v; want original error", value, ok)
	}
	if issues := got.Issues(); len(issues) != 3 || issues[0].Path != "Name" {
		t.Fatalf("issues = %+v, want three issues beginning with Name", issues)
	}
	const want = "invalid spec, spec.kind: environment, fields: Name, Port, validation: too short, too long, too small"
	if text := ValidationError(err); text != want {
		t.Fatalf("rendered error = %q, want %q", text, want)
	}
	wrapped := errorc.WithCause(coreerrors.ErrInvalidRequestParameters, err)
	if text := ValidationError(wrapped); text != want {
		t.Fatalf("rendered wrapped error = %q, want %q", text, want)
	}
}

func TestValidationErrorFallback(t *testing.T) {
	if text := ValidationError(nil); text != "" {
		t.Fatalf("rendered nil error = %q, want empty string", text)
	}
	plain := errors.New("other error")
	if text := ValidationError(plain); text != plain.Error() {
		t.Fatalf("rendered other error = %q, want %q", text, plain.Error())
	}
	invalid := errorc.With(coreerrors.ErrInvalidSpec, errorc.String(keys.Validation, "invalid kind"))
	if text := ValidationError(invalid); text != invalid.Error() {
		t.Fatalf("rendered unstructured error = %q, want %q", text, invalid.Error())
	}
}
