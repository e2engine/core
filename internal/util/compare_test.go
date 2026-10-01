package util

import (
	"reflect"
	"testing"
)

func TestIsStringSlicesEqual(t *testing.T) {
	tests := []struct {
		name     string
		left     []string
		right    []string
		expected bool
	}{
		{
			name:     "nil",
			expected: true,
		},
		{
			name:     "nil and empty",
			left:     nil,
			right:    []string{},
			expected: true,
		},
		{
			name:     "empty",
			left:     []string{},
			right:    []string{},
			expected: true,
		},
		{
			name:     "same order",
			left:     []string{"one", "two", "three"},
			right:    []string{"one", "two", "three"},
			expected: true,
		},
		{
			name:     "different order",
			left:     []string{"three", "one", "two"},
			right:    []string{"two", "three", "one"},
			expected: true,
		},
		{
			name:     "different length",
			left:     []string{"one", "two"},
			right:    []string{"one", "two", "three"},
			expected: false,
		},
		{
			name:     "different values",
			left:     []string{"one", "two", "three"},
			right:    []string{"one", "two", "four"},
			expected: false,
		},
		{
			name:     "same duplicates",
			left:     []string{"one", "two", "two"},
			right:    []string{"two", "one", "two"},
			expected: true,
		},
		{
			name:     "different duplicates",
			left:     []string{"one", "two", "two"},
			right:    []string{"one", "one", "two"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := IsStringSlicesEqual(
				tt.left,
				tt.right,
			)

			if actual != tt.expected {
				t.Fatalf(
					"expected %v, got %v",
					tt.expected,
					actual,
				)
			}
		})
	}
}

func TestIsStringSlicesEqualDoesNotModifyInputs(t *testing.T) {
	left := []string{"three", "one", "two"}
	right := []string{"two", "three", "one"}

	expectedLeft := append([]string(nil), left...)
	expectedRight := append([]string(nil), right...)

	if !IsStringSlicesEqual(left, right) {
		t.Fatal("expected slices to be equal")
	}

	if !reflect.DeepEqual(left, expectedLeft) {
		t.Fatalf(
			"left slice modified: expected %v, got %v",
			expectedLeft,
			left,
		)
	}

	if !reflect.DeepEqual(right, expectedRight) {
		t.Fatalf(
			"right slice modified: expected %v, got %v",
			expectedRight,
			right,
		)
	}
}
