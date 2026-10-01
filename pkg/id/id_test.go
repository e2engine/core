package id

import (
	"encoding/hex"
	"testing"
)

func TestTruncateID(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want string
	}{
		{
			name: "full ID",
			id:   "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			want: "0123456789ab",
		},
		{
			name: "short ID",
			id:   "0123456789",
			want: "0123456789",
		},
		{
			name: "exact short length",
			id:   "0123456789ab",
			want: "0123456789ab",
		},
		{
			name: "prefixed ID",
			id:   "sha256:0123456789abcdef",
			want: "0123456789ab",
		},
		{
			name: "empty ID",
			id:   "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TruncateID(tt.id); got != tt.want {
				t.Fatalf(
					"TruncateID(%q) = %q, want %q",
					tt.id,
					got,
					tt.want,
				)
			}
		})
	}
}

func TestGenerateRandomID(t *testing.T) {
	id := GenerateRandomID()

	if len(id) != fullLen {
		t.Fatalf(
			"GenerateRandomID() length = %d, want %d",
			len(id),
			fullLen,
		)
	}

	if _, err := hex.DecodeString(id); err != nil {
		t.Fatalf(
			"GenerateRandomID() = %q, want hexadecimal ID: %v",
			id,
			err,
		)
	}

	if allNum(TruncateID(id)) {
		t.Fatalf(
			"GenerateRandomID() short ID = %q, want at least one non-numeric character",
			TruncateID(id),
		)
	}
}

func TestAllNum(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want bool
	}{
		{name: "numbers", id: "0123456789", want: true},
		{name: "contains lowercase letter", id: "01234a6789", want: false},
		{name: "contains uppercase letter", id: "01234A6789", want: false},
		{name: "contains symbol", id: "01234-6789", want: false},
		{name: "empty", id: "", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := allNum(tt.id); got != tt.want {
				t.Fatalf(
					"allNum(%q) = %v, want %v",
					tt.id,
					got,
					tt.want,
				)
			}
		})
	}
}
