package cursor

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/errors"
)

var id1 = "28b94fc0206dfb7960905c372905772441cc41d3cde3dcd274543e6b4435367f"

func TestEncode(t *testing.T) {
	tests := []struct {
		name          string
		payload       Payload
		expected      string
		expectedError error
	}{
		{
			name: "nominal, defaults",
			payload: Payload{
				Value:    "value1",
				ID:       id1,
				Position: PositionAfter,
			},
			expected: "eyJ2ZXJzaW9uIjoxLCJ2YWx1ZSI6InZhbHVlMSIsImlkIjoiMjhiOTRmYzAyMDZkZmI3OTYwOTA1YzM3MjkwNTc3MjQ0MWNjNDFkM2NkZTNkY2QyNzQ1NDNlNmI0NDM1MzY3ZiIsInBvc2l0aW9uIjoiYWZ0ZXIiLCJvcmRlcl9ieSI6ImlkIiwib3JkZXJfZGlyZWN0aW9uIjoiYXNjIn0",
		},
		{
			name: "nominal, explicit values",
			payload: Payload{
				Version:        CurrentVersion,
				Value:          "value2",
				ID:             id1,
				Position:       PositionBefore,
				OrderBy:        model.OrderByUpdatedAt,
				OrderDirection: model.OrderDirectionDesc,
			},
		},
		{
			name: "empty value",
			payload: Payload{
				Value:    "",
				ID:       id1,
				Position: PositionAfter,
			},
			expectedError: errors.ErrInvalidCursor,
		},
		{
			name: "invalid id",
			payload: Payload{
				Value:    "value1",
				ID:       "invalid",
				Position: PositionAfter,
			},
			expectedError: errors.ErrInvalidCursor,
		},
		{
			name: "invalid position",
			payload: Payload{
				Value:    "value1",
				ID:       id1,
				Position: Position("invalid"),
			},
			expectedError: errors.ErrInvalidCursor,
		},
		{
			name: "invalid order direction",
			payload: Payload{
				Value:          "value1",
				ID:             id1,
				Position:       PositionAfter,
				OrderDirection: model.OrderDirection("invalid"),
			},
			expectedError: errors.ErrInvalidCursor,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			binding, err := GetBinding()
			if err != nil {
				t.Fatalf("expected err to be nil, got: %v", err)
			}

			encoded, err := Encode(
				context.Background(),
				binding,
				test.payload,
			)

			if test.expectedError != nil {
				if err == nil {
					t.Fatal("expected err to not be nil")
				}
				if !errors.Is(err, test.expectedError) {
					t.Fatalf(
						"expected err to be: %v, got: %v",
						test.expectedError,
						err,
					)
				}
				return
			}

			if err != nil {
				t.Fatalf("expected err to be nil, got: %v", err)
			}

			if test.expected != "" && encoded != test.expected {
				t.Fatalf(
					"expected encoded to be %v, got: %v",
					test.expected,
					encoded,
				)
			}
		})
	}
}

func TestParse(t *testing.T) {
	invalidJSON := base64.RawURLEncoding.EncodeToString(
		[]byte(`{"version":`),
	)

	tests := []struct {
		name          string
		opaque        string
		expected      Payload
		expectedError error
	}{
		{
			name:   "nominal",
			opaque: "eyJ2ZXJzaW9uIjoxLCJ2YWx1ZSI6InZhbHVlMSIsImlkIjoiMjhiOTRmYzAyMDZkZmI3OTYwOTA1YzM3MjkwNTc3MjQ0MWNjNDFkM2NkZTNkY2QyNzQ1NDNlNmI0NDM1MzY3ZiIsInBvc2l0aW9uIjoiYWZ0ZXIiLCJvcmRlcl9ieSI6ImlkIiwib3JkZXJfZGlyZWN0aW9uIjoiYXNjIn0",
			expected: Payload{
				Version:        CurrentVersion,
				Value:          "value1",
				ID:             id1,
				Position:       PositionAfter,
				OrderBy:        model.OrderByID,
				OrderDirection: model.OrderDirectionAsc,
			},
		},
		{
			name:          "invalid base64",
			opaque:        "%%%",
			expectedError: errors.ErrCannotDecodeCursor,
		},
		{
			name:          "invalid json",
			opaque:        invalidJSON,
			expectedError: errors.ErrCannotDecodeCursor,
		},
		{
			name: "unsupported version",
			opaque: encodeRawPayload(t, Payload{
				Version:        CurrentVersion + 1,
				Value:          "value1",
				ID:             id1,
				Position:       PositionAfter,
				OrderBy:        model.OrderByID,
				OrderDirection: model.OrderDirectionAsc,
			}),
			expectedError: errors.ErrInvalidCursor,
		},
		{
			name: "empty value",
			opaque: encodeRawPayload(t, Payload{
				Version:        CurrentVersion,
				ID:             id1,
				Position:       PositionAfter,
				OrderBy:        model.OrderByID,
				OrderDirection: model.OrderDirectionAsc,
			}),
			expectedError: errors.ErrInvalidCursor,
		},
		{
			name: "invalid id",
			opaque: encodeRawPayload(t, Payload{
				Version:        CurrentVersion,
				Value:          "value1",
				ID:             "invalid",
				Position:       PositionAfter,
				OrderBy:        model.OrderByID,
				OrderDirection: model.OrderDirectionAsc,
			}),
			expectedError: errors.ErrInvalidCursor,
		},
		{
			name: "invalid position",
			opaque: encodeRawPayload(t, Payload{
				Version:        CurrentVersion,
				Value:          "value1",
				ID:             id1,
				Position:       Position("invalid"),
				OrderBy:        model.OrderByID,
				OrderDirection: model.OrderDirectionAsc,
			}),
			expectedError: errors.ErrInvalidCursor,
		},
		{
			name: "invalid order direction",
			opaque: encodeRawPayload(t, Payload{
				Version:        CurrentVersion,
				Value:          "value1",
				ID:             id1,
				Position:       PositionAfter,
				OrderBy:        model.OrderByID,
				OrderDirection: model.OrderDirection("invalid"),
			}),
			expectedError: errors.ErrInvalidCursor,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			binding, err := GetBinding()
			if err != nil {
				t.Fatalf("expected err to be nil, got: %v", err)
			}

			payload, err := Parse(
				context.Background(),
				binding,
				test.opaque,
			)

			if test.expectedError != nil {
				if err == nil {
					t.Fatal("expected err to not be nil")
				}
				if !errors.Is(err, test.expectedError) {
					t.Fatalf(
						"expected err to be: %v, got: %v",
						test.expectedError,
						err,
					)
				}
				return
			}

			if err != nil {
				t.Fatalf("expected err to be nil, got: %v", err)
			}

			if payload != test.expected {
				t.Fatalf(
					"expected payload to be %v, got: %v",
					test.expected,
					payload,
				)
			}
		})
	}
}

func TestEncodeParse(t *testing.T) {
	tests := []struct {
		name     string
		payload  Payload
		expected Payload
	}{
		{
			name: "defaults",
			payload: Payload{
				Value:    "value1",
				ID:       id1,
				Position: PositionAfter,
			},
			expected: Payload{
				Version:        CurrentVersion,
				Value:          "value1",
				ID:             id1,
				Position:       PositionAfter,
				OrderBy:        model.OrderByID,
				OrderDirection: model.OrderDirectionAsc,
			},
		},
		{
			name: "explicit values",
			payload: Payload{
				Version:        CurrentVersion,
				Value:          "2026-09-18T10:00:00Z",
				ID:             id1,
				Position:       PositionBefore,
				OrderBy:        model.OrderByUpdatedAt,
				OrderDirection: model.OrderDirectionDesc,
			},
			expected: Payload{
				Version:        CurrentVersion,
				Value:          "2026-09-18T10:00:00Z",
				ID:             id1,
				Position:       PositionBefore,
				OrderBy:        model.OrderByUpdatedAt,
				OrderDirection: model.OrderDirectionDesc,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			binding, err := GetBinding()
			if err != nil {
				t.Fatalf("expected err to be nil, got: %v", err)
			}

			encoded, err := Encode(
				context.Background(),
				binding,
				test.payload,
			)
			if err != nil {
				t.Fatalf("expected err to be nil, got: %v", err)
			}

			payload, err := Parse(
				context.Background(),
				binding,
				encoded,
			)
			if err != nil {
				t.Fatalf("expected err to be nil, got: %v", err)
			}

			if payload != test.expected {
				t.Fatalf(
					"expected payload to be %v, got: %v",
					test.expected,
					payload,
				)
			}
		})
	}
}

func encodeRawPayload(t *testing.T, payload Payload) string {
	t.Helper()

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("cannot marshal payload: %v", err)
	}

	return base64.RawURLEncoding.EncodeToString(data)
}
