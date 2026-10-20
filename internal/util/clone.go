package util

import (
	"encoding/json"
	"maps"
	"slices"
)

func CloneStruct[T any](src T) (T, error) {
	var dst T

	data, err := json.Marshal(src)
	if err != nil {
		return dst, err
	}

	if err := json.Unmarshal(data, &dst); err != nil {
		return dst, err
	}

	return dst, nil
}

func CloneStringSliceMap(value map[string][]string) map[string][]string {
	result := maps.Clone(value)

	for key, items := range result {
		result[key] = slices.Clone(items)
	}

	return result
}
