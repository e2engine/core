package util

import (
	"encoding/json"
	"net/http"
	"net/url"
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

func CloneHTTPHeader(header http.Header) http.Header {
	if header == nil {
		return nil
	}

	result := make(http.Header, len(header))

	for key, values := range header {
		result[key] = append([]string(nil), values...)
	}

	return result
}

func CloneQuery(values url.Values) map[string][]string {
	if values == nil {
		return nil
	}

	result := make(map[string][]string, len(values))

	for key, value := range values {
		result[key] = append([]string(nil), value...)
	}

	return result
}

func CloneBytes(value []byte) []byte {
	if value == nil {
		return nil
	}

	return append([]byte(nil), value...)
}

func CloneStringMap(value map[string]string) map[string]string {
	if value == nil {
		return nil
	}

	result := make(map[string]string, len(value))

	for key, item := range value {
		result[key] = item
	}

	return result
}

func CloneStringSliceMap(value map[string][]string) map[string][]string {
	if value == nil {
		return nil
	}

	result := make(map[string][]string, len(value))

	for key, item := range value {
		cloned := make([]string, len(item))
		copy(cloned, item)
		result[key] = cloned
	}

	return result
}

func CloneMap(value map[string]any) map[string]any {
	if value == nil {
		return nil
	}

	result := make(map[string]any, len(value))

	for key, item := range value {
		result[key] = item
	}

	return result
}

func CloneStringSlice(value []string) []string {
	if value == nil {
		return nil
	}

	result := make([]string, len(value))
	copy(result, value)

	return result
}
