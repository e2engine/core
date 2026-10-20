package util

import (
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
