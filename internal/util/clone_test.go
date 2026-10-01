package util

import (
	"net/http"
	"net/url"
	"reflect"
	"testing"
)

func TestCloneStruct(t *testing.T) {
	type nested struct {
		Value string
	}

	type source struct {
		Name   string
		Items  []string
		Values map[string]string
		Nested *nested
	}

	original := source{
		Name:   "test",
		Items:  []string{"one", "two"},
		Values: map[string]string{"key": "value"},
		Nested: &nested{
			Value: "nested",
		},
	}

	cloned, err := CloneStruct(original)
	if err != nil {
		t.Fatalf("clone struct: %v", err)
	}

	if !reflect.DeepEqual(cloned, original) {
		t.Fatalf(
			"expected %+v, got %+v",
			original,
			cloned,
		)
	}

	cloned.Items[0] = "changed"
	cloned.Values["key"] = "changed"
	cloned.Nested.Value = "changed"

	if original.Items[0] != "one" {
		t.Fatal("slice was not cloned")
	}

	if original.Values["key"] != "value" {
		t.Fatal("map was not cloned")
	}

	if original.Nested.Value != "nested" {
		t.Fatal("nested pointer was not cloned")
	}
}

func TestCloneStructMarshalError(t *testing.T) {
	type source struct {
		Channel chan int
	}

	_, err := CloneStruct(source{
		Channel: make(chan int),
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCloneHTTPHeader(t *testing.T) {
	tests := []struct {
		name  string
		input http.Header
	}{
		{
			name: "nil",
		},
		{
			name:  "empty",
			input: http.Header{},
		},
		{
			name: "values",
			input: http.Header{
				"Content-Type": {"application/json"},
				"Accept":       {"application/json", "text/plain"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cloned := CloneHTTPHeader(tt.input)

			if !reflect.DeepEqual(cloned, tt.input) {
				t.Fatalf(
					"expected %v, got %v",
					tt.input,
					cloned,
				)
			}

			if tt.input == nil {
				if cloned != nil {
					t.Fatal("expected nil")
				}
				return
			}

			cloned.Set("X-Test", "value")

			if tt.input.Get("X-Test") != "" {
				t.Fatal("map was not cloned")
			}

			if len(tt.input["Accept"]) > 0 {
				cloned["Accept"][0] = "changed"

				if tt.input["Accept"][0] == "changed" {
					t.Fatal("header values were not cloned")
				}
			}
		})
	}
}

func TestCloneQuery(t *testing.T) {
	tests := []struct {
		name  string
		input url.Values
	}{
		{
			name: "nil",
		},
		{
			name:  "empty",
			input: url.Values{},
		},
		{
			name: "values",
			input: url.Values{
				"key": {"one", "two"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cloned := CloneQuery(tt.input)

			if !reflect.DeepEqual(cloned, map[string][]string(tt.input)) {
				t.Fatalf(
					"expected %v, got %v",
					tt.input,
					cloned,
				)
			}

			if tt.input == nil {
				if cloned != nil {
					t.Fatal("expected nil")
				}
				return
			}

			cloned["new"] = []string{"value"}

			if _, ok := tt.input["new"]; ok {
				t.Fatal("map was not cloned")
			}

			if len(tt.input["key"]) > 0 {
				cloned["key"][0] = "changed"

				if tt.input["key"][0] == "changed" {
					t.Fatal("query values were not cloned")
				}
			}
		})
	}
}

func TestCloneBytes(t *testing.T) {
	if CloneBytes(nil) != nil {
		t.Fatal("expected nil")
	}

	original := []byte{1, 2, 3}
	cloned := CloneBytes(original)

	if !reflect.DeepEqual(cloned, original) {
		t.Fatalf(
			"expected %v, got %v",
			original,
			cloned,
		)
	}

	cloned[0] = 9

	if original[0] == 9 {
		t.Fatal("bytes were not cloned")
	}
}

func TestCloneStringMap(t *testing.T) {
	if CloneStringMap(nil) != nil {
		t.Fatal("expected nil")
	}

	original := map[string]string{
		"one": "1",
		"two": "2",
	}
	cloned := CloneStringMap(original)

	if !reflect.DeepEqual(cloned, original) {
		t.Fatalf(
			"expected %v, got %v",
			original,
			cloned,
		)
	}

	cloned["one"] = "changed"
	cloned["three"] = "3"

	if original["one"] != "1" {
		t.Fatal("map value was not cloned")
	}

	if _, ok := original["three"]; ok {
		t.Fatal("map was not cloned")
	}
}

func TestCloneStringSliceMap(t *testing.T) {
	if CloneStringSliceMap(nil) != nil {
		t.Fatal("expected nil")
	}

	original := map[string][]string{
		"one": {"a", "b"},
		"two": {"c"},
	}
	cloned := CloneStringSliceMap(original)

	if !reflect.DeepEqual(cloned, original) {
		t.Fatalf(
			"expected %v, got %v",
			original,
			cloned,
		)
	}

	cloned["three"] = []string{"d"}

	if _, ok := original["three"]; ok {
		t.Fatal("map was not cloned")
	}

	cloned["one"][0] = "changed"

	if original["one"][0] == "changed" {
		t.Fatal("map slice value was not cloned")
	}
}

func TestCloneMap(t *testing.T) {
	if CloneMap(nil) != nil {
		t.Fatal("expected nil")
	}

	nested := []string{"one"}

	original := map[string]any{
		"name":   "test",
		"nested": nested,
	}
	cloned := CloneMap(original)

	if !reflect.DeepEqual(cloned, original) {
		t.Fatalf(
			"expected %v, got %v",
			original,
			cloned,
		)
	}

	cloned["name"] = "changed"

	if original["name"] != "test" {
		t.Fatal("map was not cloned")
	}

	// CloneMap intentionally performs a shallow clone.
	clonedNested := cloned["nested"].([]string)
	clonedNested[0] = "changed"

	if nested[0] != "changed" {
		t.Fatal("expected nested value to remain shared")
	}
}

func TestCloneStringSlice(t *testing.T) {
	if CloneStringSlice(nil) != nil {
		t.Fatal("expected nil")
	}

	original := []string{"one", "two"}
	cloned := CloneStringSlice(original)

	if !reflect.DeepEqual(cloned, original) {
		t.Fatalf(
			"expected %v, got %v",
			original,
			cloned,
		)
	}

	cloned[0] = "changed"

	if original[0] == "changed" {
		t.Fatal("slice was not cloned")
	}
}
