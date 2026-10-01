package util

import (
	"bytes"
	"encoding/json"
	"reflect"
)

func CompactJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return string(raw)
	}
	return buf.String()
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
