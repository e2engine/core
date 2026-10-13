package match

import (
	"encoding/json"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"

	"github.com/e2engine/core/internal/util"
	"github.com/e2engine/core/model/matcher"
)

func IsStringMatched(
	actual []byte,
	expected *matcher.String,
) bool {
	if expected == nil {
		panic("string matcher must not be nil")
	}

	switch expected.Operator {
	case matcher.StringMatchOperatorExists:
		return true

	case matcher.StringMatchOperatorNonEmpty:
		return len(actual) > 0

	case matcher.StringMatchOperatorEquals:
		return string(actual) == expected.Value

	default:
		// Validation guarantees a supported operator.
		panic("invalid string matcher operator")
	}
}

func IsStringSliceMatched(
	actual []string,
	exists bool,
	expected []matcher.String,
) bool {
	if len(expected) == 0 {
		panic("string matchers must not be empty")
	}

	if !exists {
		return false
	}

	switch expected[0].Operator {
	case matcher.StringMatchOperatorExists:
		return true

	case matcher.StringMatchOperatorNonEmpty:
		for _, value := range actual {
			if value != "" {
				return true
			}
		}
		return false

	case matcher.StringMatchOperatorEquals:
		expectedValues := make([]string, len(expected))
		for i := range expected {
			expectedValues[i] = expected[i].Value
		}

		return util.IsStringSlicesEqual(actual, expectedValues)

	default:
		panic("invalid string matcher operator")
	}
}

func IsJSONMatched(
	actual []byte,
	expected *matcher.String,
) (bool, error) {
	if expected == nil {
		panic("JSON matcher must not be nil")
	}

	switch expected.Operator {
	case matcher.StringMatchOperatorExists:
		return json.Valid(actual), nil

	case matcher.StringMatchOperatorNonEmpty:
		return len(actual) > 0 && json.Valid(actual), nil

	case matcher.StringMatchOperatorEquals:
		return util.IsJSONEqual(
			[]byte(expected.Value),
			actual,
		)

	default:
		// Validation guarantees a supported operator.
		panic("invalid string matcher operator")
	}
}

func IsGRPCMessageMatched(
	actual proto.Message,
	expected *matcher.GRPCMessage,
) (bool, error) {
	if expected == nil {
		panic("gRPC message matcher must not be nil")
	}
	switch expected.Operator {
	case matcher.GRPCMessageMatchOperatorNonEmpty:
		nonempty := false
		actual.ProtoReflect().Range(
			func(_ protoreflect.FieldDescriptor, _ protoreflect.Value) bool {
				nonempty = true
				return false
			},
		)
		return nonempty, nil
	case matcher.GRPCMessageMatchOperatorEquals:
		expectedMessage := dynamicpb.NewMessage(
			actual.ProtoReflect().Descriptor(),
		)
		if err := protojson.Unmarshal(
			[]byte(expected.Value),
			expectedMessage,
		); err != nil {
			return false, err
		}
		return proto.Equal(actual, expectedMessage), nil
	default:
		panic("invalid gRPC message matcher operator")
	}
}
