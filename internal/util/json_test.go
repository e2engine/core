package util

import (
	"encoding/json"
	"testing"
)

func TestCompactJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    json.RawMessage
		expected string
	}{
		{
			name:     "nil",
			input:    nil,
			expected: "",
		},
		{
			name:     "empty",
			input:    json.RawMessage{},
			expected: "",
		},
		{
			name: "object",
			input: json.RawMessage(`
				{
					"name": "test",
					"enabled": true
				}
			`),
			expected: `{"name":"test","enabled":true}`,
		},
		{
			name: "array",
			input: json.RawMessage(`
				[
					"one",
					"two",
					3
				]
			`),
			expected: `["one","two",3]`,
		},
		{
			name:     "already compact",
			input:    json.RawMessage(`{"name":"test"}`),
			expected: `{"name":"test"}`,
		},
		{
			name:     "scalar",
			input:    json.RawMessage(`  "test"  `),
			expected: `"test"`,
		},
		{
			name:     "invalid returns original",
			input:    json.RawMessage(`{ "name": invalid }`),
			expected: `{ "name": invalid }`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := CompactJSON(tt.input)

			if actual != tt.expected {
				t.Fatalf(
					"expected %q, got %q",
					tt.expected,
					actual,
				)
			}
		})
	}
}

func TestIsJSONEqual(t *testing.T) {
	tests := []struct {
		name string

		expected []byte
		actual   []byte

		equal       bool
		expectError bool
	}{
		{
			name:     "objects equal",
			expected: []byte(`{"name":"test","enabled":true}`),
			actual:   []byte(`{"name":"test","enabled":true}`),
			equal:    true,
		},
		{
			name:     "object key order ignored",
			expected: []byte(`{"name":"test","enabled":true}`),
			actual:   []byte(`{"enabled":true,"name":"test"}`),
			equal:    true,
		},
		{
			name: "whitespace ignored",
			expected: []byte(`
				{
					"name": "test",
					"enabled": true
				}
			`),
			actual: []byte(`{"name":"test","enabled":true}`),
			equal:  true,
		},
		{
			name:     "different object value",
			expected: []byte(`{"name":"test"}`),
			actual:   []byte(`{"name":"other"}`),
			equal:    false,
		},
		{
			name:     "different object field",
			expected: []byte(`{"name":"test"}`),
			actual:   []byte(`{"name":"test","enabled":true}`),
			equal:    false,
		},
		{
			name:     "arrays equal",
			expected: []byte(`[1,2,3]`),
			actual:   []byte(`[1,2,3]`),
			equal:    true,
		},
		{
			name:     "array order significant",
			expected: []byte(`[1,2,3]`),
			actual:   []byte(`[3,2,1]`),
			equal:    false,
		},
		{
			name: "nested equal",
			expected: []byte(
				`{"items":[{"id":1},{"id":2}],"meta":{"count":2}}`,
			),
			actual: []byte(
				`{"meta":{"count":2},"items":[{"id":1},{"id":2}]}`,
			),
			equal: true,
		},
		{
			name:     "numbers equal",
			expected: []byte(`1`),
			actual:   []byte(`1.0`),
			equal:    true,
		},
		{
			name:     "null equal",
			expected: []byte(`null`),
			actual:   []byte(`null`),
			equal:    true,
		},
		{
			name:     "null and object differ",
			expected: []byte(`null`),
			actual:   []byte(`{}`),
			equal:    false,
		},
		{
			name:        "invalid expected",
			expected:    []byte(`{invalid}`),
			actual:      []byte(`{}`),
			expectError: true,
		},
		{
			name:        "invalid actual",
			expected:    []byte(`{}`),
			actual:      []byte(`{invalid}`),
			expectError: true,
		},
		{
			name:        "empty expected",
			expected:    nil,
			actual:      []byte(`{}`),
			expectError: true,
		},
		{
			name:        "empty actual",
			expected:    []byte(`{}`),
			actual:      nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			equal, err := IsJSONEqual(
				tt.expected,
				tt.actual,
			)

			if tt.expectError {
				if err == nil {
					t.Fatal("expected error")
				}
				if equal {
					t.Fatal("expected false on error")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if equal != tt.equal {
				t.Fatalf(
					"expected equality %v, got %v",
					tt.equal,
					equal,
				)
			}
		})
	}
}
