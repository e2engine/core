package matcher

import (
	"encoding/json"

	"github.com/ygrebnov/errorc"
	modellib "github.com/ygrebnov/model"
	"gopkg.in/yaml.v3"

	"github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/keys"
)

type GRPCMessage struct {
	Operator GRPCMessageMatchOperator `json:"operator" yaml:"operator"`
	Value    string                   `json:"value,omitempty" yaml:"value,omitempty"`
}

func (m GRPCMessage) String() string {
	if m.Operator == GRPCMessageMatchOperatorEquals {
		return m.Value
	}

	return string(m.Operator)
}

type GRPCMessageMatchOperator string

const (
	GRPCMessageMatchOperatorEquals   GRPCMessageMatchOperator = "equals"
	GRPCMessageMatchOperatorNonEmpty GRPCMessageMatchOperator = "nonempty"
)

func (m GRPCMessage) MarshalJSON() ([]byte, error) {
	if m.Operator == GRPCMessageMatchOperatorEquals {
		return json.Marshal(m.Value)
	}

	type alias GRPCMessage
	return json.Marshal(alias(m))
}

func (m *GRPCMessage) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err == nil {
		*m = GRPCMessage{
			Operator: GRPCMessageMatchOperatorEquals,
			Value:    value,
		}
		return nil
	}

	type alias GRPCMessage
	var matcher alias
	if err := json.Unmarshal(data, &matcher); err != nil {
		return errorc.With(
			errors.ErrInvalidGRPCMessageMatcher,
			errorc.String(keys.Validation, "gRPC message matcher must be a string or object"),
		)
	}

	*m = GRPCMessage(matcher)
	return nil
}

func (m GRPCMessage) MarshalYAML() (any, error) {
	if m.Operator == GRPCMessageMatchOperatorEquals {
		return m.Value, nil
	}

	type alias GRPCMessage
	return alias(m), nil
}

func (m *GRPCMessage) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode &&
		node.Tag == "!!str" {
		*m = GRPCMessage{
			Operator: GRPCMessageMatchOperatorEquals,
			Value:    node.Value,
		}
		return nil
	}

	if node.Kind != yaml.MappingNode {
		return errorc.With(
			errors.ErrInvalidGRPCMessageMatcher,
			errorc.String(keys.Validation, "gRPC message matcher must be a string or object"),
		)
	}

	type alias GRPCMessage
	var matcher alias
	if err := node.Decode(&matcher); err != nil {
		return err
	}

	*m = GRPCMessage(matcher)
	return nil
}

func GetGRPCMessageMatcherRule() (modellib.Rule, error) {
	return modellib.NewRule[*GRPCMessage](
		"grpcmessagematcher",
		func(value *GRPCMessage, _ ...string) error {
			if value == nil {
				return nil
			}
			return validateGRPCMessageMatcher(*value)
		},
	)
}

func validateGRPCMessageMatcher(value GRPCMessage) error {
	switch value.Operator {
	case GRPCMessageMatchOperatorEquals:
		if !json.Valid([]byte(value.Value)) {
			return errorc.With(
				errors.ErrInvalidSpec,
				errorc.String(
					keys.Validation,
					"value must contain valid JSON",
				),
			)
		}
		return nil

	case GRPCMessageMatchOperatorNonEmpty:
		if value.Value != "" {
			return errorc.With(
				errors.ErrInvalidSpec,
				errorc.String(
					keys.Validation,
					"value must be empty when operator is 'nonempty'",
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
