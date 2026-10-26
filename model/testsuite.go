package model

import (
	"context"
	nativeerrors "errors"
	"strconv"

	"github.com/ygrebnov/errorc"
	modellib "github.com/ygrebnov/model"

	"github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/keys"
)

type TestSuite Resource[TestSuiteSpec]

var testSuiteBinding *modellib.Binding[TestSuite]

func (t *TestSuite) Validate(ctx context.Context) error {
	if t.Kind != ResourceKindTestSuite {
		return invalidTestSuiteSpec(nativeerrors.New("invalid kind"), "Kind")
	}

	if err := testSuiteBinding.Validate(ctx, t); err != nil {
		return handleValidationError(err, EntityKindTestSuite)
	}

	if fields, err := t.Spec.Selectors.Validate(); err != nil {
		return invalidTestSuiteSpec(err, fields)
	}

	return nil
}

type TestSuiteSpec struct {
	Selectors TestSelectorsSpec `json:"selectors" yaml:"selectors"`
}

func invalidTestSuiteSpec(validation error, fields string) error {
	return errorc.With(
		errors.ErrInvalidSpec,
		errorc.String(keys.SpecKind, string(EntityKindTestSuite)),
		errorc.String(keys.Fields, fields),
		errorc.String(keys.Validation, validation.Error()),
	)
}

type TestSelectorsSpec struct {
	IDs   []string `json:"ids,omitempty" yaml:"ids,omitempty" validateElem:"id"`
	Names []string `json:"names,omitempty" yaml:"names,omitempty" validateElem:"min(1),max(200)"`
	Tags  []string `json:"tags,omitempty" yaml:"tags,omitempty" validateElem:"min(1),max(200)"`
}

func (s *TestSelectorsSpec) Validate() (string, error) {
	if len(s.IDs) == 0 && len(s.Names) == 0 && len(s.Tags) == 0 {
		return "Spec.Selectors.IDs, Spec.Selectors.Names, Spec.Selectors.Tags",
			errorc.New("test selectors spec must contain at least one of ids, names, or tags")
	}

	if index, err := validateUniqueSelectors(s.IDs, "ids"); err != nil {
		return "Spec.Selectors.IDs[" + strconv.Itoa(index) + "]", err
	}

	if index, err := validateUniqueSelectors(s.Names, "names"); err != nil {
		return "Spec.Selectors.Names[" + strconv.Itoa(index) + "]", err
	}

	if index, err := validateUniqueSelectors(s.Tags, "tags"); err != nil {
		return "Spec.Selectors.Tags[" + strconv.Itoa(index) + "]", err
	}

	return "", nil
}

type TestSuitesPage = Page[TestSuite]

func validateUniqueSelectors(values []string, selector string) (int, error) {
	seen := make(map[string]struct{}, len(values))

	for i, value := range values {
		if _, ok := seen[value]; ok {
			return i, errorc.With(
				errorc.New("test selectors must be unique"),
				errorc.String(keys.TestSuiteSelector, selector),
				errorc.String(keys.Value, value),
			)
		}

		seen[value] = struct{}{}
	}

	return 0, nil
}
