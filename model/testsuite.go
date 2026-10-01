package model

import (
	"context"
	nativeerrors "errors"

	"github.com/ygrebnov/errorc"
	modellib "github.com/ygrebnov/model"

	"github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/keys"
)

type TestSuite Resource[TestSuiteSpec]

var testSuiteBinding *modellib.Binding[TestSuite]

func (t *TestSuite) Validate(ctx context.Context) error {
	if t.Kind != ResourceKindTestSuite {
		return errorc.With(
			errors.ErrInvalidSpec,
			errorc.String(keys.SpecKind, string(EntityKindTestSuite)),
			errorc.String(keys.Validation, "invalid resource kind"),
		)
	}

	if err := testSuiteBinding.Validate(ctx, t); err != nil {
		return nativeerrors.Join(
			errorc.With(
				errors.ErrInvalidSpec,
				errorc.String(keys.SpecKind, string(EntityKindTestSuite)),
			),
			err,
		)
	}

	if err := t.Spec.Selectors.Validate(ctx); err != nil {
		return invalidTestSuiteSpec(err)
	}

	return nil
}

type TestSuiteSpec struct {
	Selectors TestSelectorsSpec `json:"selectors" yaml:"selectors"`
}

func invalidTestSuiteSpec(validation error) error {
	return nativeerrors.Join(
		errorc.With(
			errors.ErrInvalidSpec,
			errorc.String(keys.SpecKind, string(EntityKindTestSuite)),
		),
		validation,
	)
}

type TestSelectorsSpec struct {
	IDs   []string `json:"ids,omitempty" yaml:"ids,omitempty" validateElem:"min(1),max(200)"`
	Names []string `json:"names,omitempty" yaml:"names,omitempty" validateElem:"min(1),max(200)"`
	Tags  []string `json:"tags,omitempty" yaml:"tags,omitempty" validateElem:"min(1),max(200)"`
}

func (s *TestSelectorsSpec) Validate(_ context.Context) error {
	if len(s.IDs) == 0 && len(s.Names) == 0 && len(s.Tags) == 0 {
		return errorc.New("test selectors spec must contain at least one of ids, names, or tags")
	}
	if err := validateUniqueSelectors(s.IDs, "ids"); err != nil {
		return err
	}

	if err := validateUniqueSelectors(s.Names, "names"); err != nil {
		return err
	}

	if err := validateUniqueSelectors(s.Tags, "tags"); err != nil {
		return err
	}
	return nil
}

type TestSuitesPage = Page[TestSuite]

func validateUniqueSelectors(values []string, selector string) error {
	seen := make(map[string]struct{}, len(values))

	for _, value := range values {
		if _, ok := seen[value]; ok {
			return errorc.With(
				errorc.New("test selectors must be unique"),
				errorc.String(keys.TestSuiteSelector, selector),
				errorc.String(keys.Value, value),
			)
		}

		seen[value] = struct{}{}
	}

	return nil
}
