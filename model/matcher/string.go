package matcher

import (
	"encoding/json"
	"fmt"

	"github.com/ygrebnov/errorc"
	modellib "github.com/ygrebnov/model"
	"gopkg.in/yaml.v3"

	"github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/keys"
)

type String struct {
	Operator StringMatchOperator `json:"operator" yaml:"operator"`
	Value    string              `json:"value,omitempty" yaml:"value,omitempty"`
}

func (m String) String() string {
	if m.Operator == StringMatchOperatorEquals {
		return m.Value
	}

	return string(m.Operator)
}

type StringMatchOperator string

const (
	StringMatchOperatorEquals   StringMatchOperator = "equals"
	StringMatchOperatorExists   StringMatchOperator = "exists"
	StringMatchOperatorNonEmpty StringMatchOperator = "nonempty"
)

func (m String) MarshalJSON() ([]byte, error) {
	if m.Operator == StringMatchOperatorEquals {
		return json.Marshal(m.Value)
	}

	type alias String
	return json.Marshal(alias(m))
}

func (m *String) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err == nil {
		*m = String{
			Operator: StringMatchOperatorEquals,
			Value:    value,
		}
		return nil
	}

	type alias String
	var matcher alias
	if err := json.Unmarshal(data, &matcher); err != nil {
		return fmt.Errorf(
			"string matcher must be a string or object: %w",
			err,
		)
	}

	*m = String(matcher)
	return nil
}

func (m String) MarshalYAML() (any, error) {
	if m.Operator == StringMatchOperatorEquals {
		return m.Value, nil
	}

	type alias String
	return alias(m), nil
}

func (m *String) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode &&
		node.Tag == "!!str" {
		*m = String{
			Operator: StringMatchOperatorEquals,
			Value:    node.Value,
		}
		return nil
	}

	if node.Kind != yaml.MappingNode {
		return fmt.Errorf(
			"string matcher must be a string or object",
		)
	}

	type alias String
	var matcher alias
	if err := node.Decode(&matcher); err != nil {
		return err
	}

	*m = String(matcher)
	return nil
}

func GetStringMatchersRule() (modellib.Rule, error) {
	return modellib.NewRule[[]String](
		"stringmatchers",
		func(values []String, _ ...string) error {
			if len(values) == 0 {
				return errorc.With(
					errors.ErrInvalidSpec,
					errorc.String(
						keys.Validation,
						"at least one string matcher must be specified",
					),
				)
			}

			operator := values[0].Operator

			for _, value := range values {
				if err := validateStringMatcher(value); err != nil {
					return err
				}

				if value.Operator != operator {
					return errorc.With(
						errors.ErrInvalidSpec,
						errorc.String(
							keys.Validation,
							"string matcher operators cannot be mixed",
						),
					)
				}
			}

			if operator != StringMatchOperatorEquals &&
				len(values) != 1 {
				return errorc.With(
					errors.ErrInvalidSpec,
					errorc.String(
						keys.Validation,
						"exists and nonempty operators can only be specified once",
					),
				)
			}

			return nil
		},
	)
}

func GetStringMatcherRule() (modellib.Rule, error) {
	return modellib.NewRule[*String](
		"stringmatcher",
		func(value *String, _ ...string) error {
			if value == nil {
				return nil
			}
			return validateStringMatcher(*value)
		},
	)
}

func validateStringMatcher(value String) error {
	switch value.Operator {
	case StringMatchOperatorEquals:
		// Any string, including an empty string, is valid.
		return nil

	case StringMatchOperatorExists,
		StringMatchOperatorNonEmpty:
		if value.Value != "" {
			return errorc.With(
				errors.ErrInvalidSpec,
				errorc.String(
					keys.MatcherOperator,
					string(value.Operator),
				),
				errorc.String(
					keys.Validation,
					"value must be empty for this operator",
				),
			)
		}

		return nil

	default:
		return errorc.With(
			errors.ErrInvalidSpec,
			errorc.String(
				keys.MatcherOperator,
				string(value.Operator),
			),
			errorc.String(
				keys.Validation,
				"invalid operator",
			),
		)
	}
}
