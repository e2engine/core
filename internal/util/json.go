package util

import (
	"bytes"
	"encoding/json"
	"reflect"
)

func CompactJSON(raw json.RawMessage) []byte {
	if len(raw) == 0 {
		return nil
	}

	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return raw
	}
	return buf.Bytes()
}

func IsJSONEqual(expected, actual []byte) (bool, error) {
	var expectedValue any
	if err := json.Unmarshal(expected, &expectedValue); err != nil {
		return false, err
	}

	var actualValue any
	if err := json.Unmarshal(actual, &actualValue); err != nil {
		return false, err
	}

	return reflect.DeepEqual(
		expectedValue,
		actualValue,
	), nil
}
