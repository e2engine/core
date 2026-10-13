package match

import (
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/e2engine/core/model/matcher"
)

func TestIsStringMatched(t *testing.T) {
	tests := []struct {
		name     string
		actual   string
		expected matcher.String
		want     bool
	}{
		{
			name:   "equals",
			actual: "hello",
			expected: matcher.String{
				Operator: matcher.StringMatchOperatorEquals,
				Value:    "hello",
			},
			want: true,
		},
		{
			name:   "equals mismatch",
			actual: "world",
			expected: matcher.String{
				Operator: matcher.StringMatchOperatorEquals,
				Value:    "hello",
			},
		},
		{
			name:   "equals empty",
			actual: "",
			expected: matcher.String{
				Operator: matcher.StringMatchOperatorEquals,
				Value:    "",
			},
			want: true,
		},
		{
			name:   "equals empty mismatch",
			actual: "hello",
			expected: matcher.String{
				Operator: matcher.StringMatchOperatorEquals,
				Value:    "",
			},
		},
		{
			name:   "equals preserves whitespace",
			actual: "hello\n",
			expected: matcher.String{
				Operator: matcher.StringMatchOperatorEquals,
				Value:    "hello",
			},
		},
		{
			name:   "exists with content",
			actual: "hello",
			expected: matcher.String{
				Operator: matcher.StringMatchOperatorExists,
			},
			want: true,
		},
		{
			name:   "exists with empty content",
			actual: "",
			expected: matcher.String{
				Operator: matcher.StringMatchOperatorExists,
			},
			want: true,
		},
		{
			name:   "nonempty with content",
			actual: "hello",
			expected: matcher.String{
				Operator: matcher.StringMatchOperatorNonEmpty,
			},
			want: true,
		},
		{
			name:   "nonempty with empty content",
			actual: "",
			expected: matcher.String{
				Operator: matcher.StringMatchOperatorNonEmpty,
			},
		},
		{
			name:   "nonempty with whitespace",
			actual: " ",
			expected: matcher.String{
				Operator: matcher.StringMatchOperatorNonEmpty,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsStringMatched(
				[]byte(tt.actual),
				&tt.expected,
			)

			if got != tt.want {
				t.Errorf(
					"IsStringMatched() = %v, want %v",
					got,
					tt.want,
				)
			}
		})
	}
}

func TestIsStringSliceMatched(t *testing.T) {
	eq := func(value string) matcher.String {
		return matcher.String{
			Operator: matcher.StringMatchOperatorEquals,
			Value:    value,
		}
	}

	exists := matcher.String{
		Operator: matcher.StringMatchOperatorExists,
	}

	nonempty := matcher.String{
		Operator: matcher.StringMatchOperatorNonEmpty,
	}

	tests := []struct {
		name     string
		actual   []string
		exists   bool
		expected []matcher.String
		want     bool
	}{
		{
			name:     "exists with value",
			actual:   []string{"hello"},
			exists:   true,
			expected: []matcher.String{exists},
			want:     true,
		},
		{
			name:     "exists with empty value",
			actual:   []string{""},
			exists:   true,
			expected: []matcher.String{exists},
			want:     true,
		},
		{
			name:     "exists with empty slice",
			actual:   []string{},
			exists:   true,
			expected: []matcher.String{exists},
			want:     true,
		},
		{
			name:     "exists with absent key",
			exists:   false,
			expected: []matcher.String{exists},
		},
		{
			name:     "nonempty with value",
			actual:   []string{"hello"},
			exists:   true,
			expected: []matcher.String{nonempty},
			want:     true,
		},
		{
			name:     "nonempty with empty value",
			actual:   []string{""},
			exists:   true,
			expected: []matcher.String{nonempty},
		},
		{
			name:     "nonempty with mixed values",
			actual:   []string{"", "hello"},
			exists:   true,
			expected: []matcher.String{nonempty},
			want:     true,
		},
		{
			name:     "nonempty with empty slice",
			actual:   []string{},
			exists:   true,
			expected: []matcher.String{nonempty},
		},
		{
			name:     "nonempty with absent key",
			actual:   []string{"hello"},
			exists:   false,
			expected: []matcher.String{nonempty},
		},
		{
			name:     "equals single value",
			actual:   []string{"hello"},
			exists:   true,
			expected: []matcher.String{eq("hello")},
			want:     true,
		},
		{
			name:     "equals empty value",
			actual:   []string{""},
			exists:   true,
			expected: []matcher.String{eq("")},
			want:     true,
		},
		{
			name:     "equals mismatch",
			actual:   []string{"world"},
			exists:   true,
			expected: []matcher.String{eq("hello")},
		},
		{
			name:     "equals absent key",
			exists:   false,
			expected: []matcher.String{eq("")},
		},
		{
			name:     "equals multiple values",
			actual:   []string{"hello", "world"},
			exists:   true,
			expected: []matcher.String{eq("hello"), eq("world")},
			want:     true,
		},
		{
			name:     "equals multiple values different order",
			actual:   []string{"world", "hello"},
			exists:   true,
			expected: []matcher.String{eq("hello"), eq("world")},
			want:     true,
		},
		{
			name:     "equals missing value",
			actual:   []string{"hello"},
			exists:   true,
			expected: []matcher.String{eq("hello"), eq("world")},
		},
		{
			name:     "equals extra value",
			actual:   []string{"hello", "world"},
			exists:   true,
			expected: []matcher.String{eq("hello")},
		},
		{
			name:     "equals duplicate values",
			actual:   []string{"hello", "hello"},
			exists:   true,
			expected: []matcher.String{eq("hello"), eq("hello")},
			want:     true,
		},
		{
			name:     "equals duplicate count mismatch",
			actual:   []string{"hello", "world"},
			exists:   true,
			expected: []matcher.String{eq("hello"), eq("hello")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsStringSliceMatched(
				tt.actual,
				tt.exists,
				tt.expected,
			)

			if got != tt.want {
				t.Errorf(
					"IsStringSliceMatched() = %v, want %v",
					got,
					tt.want,
				)
			}
		})
	}
}

func TestIsJSONMatched(t *testing.T) {
	tests := []struct {
		name     string
		actual   string
		expected matcher.String
		want     bool
	}{
		{
			name:   "equals objects",
			actual: `{"name":"John","age":30}`,
			expected: matcher.String{
				Operator: matcher.StringMatchOperatorEquals,
				Value:    `{"name":"John","age":30}`,
			},
			want: true,
		},
		{
			name:   "equals different key order",
			actual: `{"age":30,"name":"John"}`,
			expected: matcher.String{
				Operator: matcher.StringMatchOperatorEquals,
				Value:    `{"name":"John","age":30}`,
			},
			want: true,
		},
		{
			name:   "equals different whitespace",
			actual: "{\n\"name\": \"John\"\n}\n",
			expected: matcher.String{
				Operator: matcher.StringMatchOperatorEquals,
				Value:    `{"name":"John"}`,
			},
			want: true,
		},
		{
			name:   "equals mismatch",
			actual: `{"name":"Jane"}`,
			expected: matcher.String{
				Operator: matcher.StringMatchOperatorEquals,
				Value:    `{"name":"John"}`,
			},
		},
		{
			name:   "equals arrays",
			actual: `[1,2,3]`,
			expected: matcher.String{
				Operator: matcher.StringMatchOperatorEquals,
				Value:    `[1,2,3]`,
			},
			want: true,
		},
		{
			name:   "equals array order mismatch",
			actual: `[3,2,1]`,
			expected: matcher.String{
				Operator: matcher.StringMatchOperatorEquals,
				Value:    `[1,2,3]`,
			},
		},
		{
			name:   "equals JSON string",
			actual: `"hello"`,
			expected: matcher.String{
				Operator: matcher.StringMatchOperatorEquals,
				Value:    `"hello"`,
			},
			want: true,
		},
		{
			name:   "equals empty object",
			actual: `{}`,
			expected: matcher.String{
				Operator: matcher.StringMatchOperatorEquals,
				Value:    `{}`,
			},
			want: true,
		},
		{
			name:   "exists with object",
			actual: `{}`,
			expected: matcher.String{
				Operator: matcher.StringMatchOperatorExists,
			},
			want: true,
		},
		{
			name:   "exists with null",
			actual: `null`,
			expected: matcher.String{
				Operator: matcher.StringMatchOperatorExists,
			},
			want: true,
		},
		{
			name:   "exists with empty body",
			actual: "",
			expected: matcher.String{
				Operator: matcher.StringMatchOperatorExists,
			},
		},
		{
			name:   "exists with invalid JSON",
			actual: `hello`,
			expected: matcher.String{
				Operator: matcher.StringMatchOperatorExists,
			},
		},
		{
			name:   "nonempty with object",
			actual: `{"name":"John"}`,
			expected: matcher.String{
				Operator: matcher.StringMatchOperatorNonEmpty,
			},
			want: true,
		},
		{
			name:   "nonempty with empty object",
			actual: `{}`,
			expected: matcher.String{
				Operator: matcher.StringMatchOperatorNonEmpty,
			},
			want: true,
		},
		{
			name:   "nonempty with null",
			actual: `null`,
			expected: matcher.String{
				Operator: matcher.StringMatchOperatorNonEmpty,
			},
			want: true,
		},
		{
			name:   "nonempty with empty JSON string",
			actual: `""`,
			expected: matcher.String{
				Operator: matcher.StringMatchOperatorNonEmpty,
			},
			want: true,
		},
		{
			name:   "nonempty with empty body",
			actual: "",
			expected: matcher.String{
				Operator: matcher.StringMatchOperatorNonEmpty,
			},
		},
		{
			name:   "nonempty with invalid JSON",
			actual: `hello`,
			expected: matcher.String{
				Operator: matcher.StringMatchOperatorNonEmpty,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsJSONMatched(
				[]byte(tt.actual),
				&tt.expected,
			)
			if err != nil {
				t.Fatalf("IsJSONMatched() error = %v", err)
			}

			if got != tt.want {
				t.Errorf(
					"IsJSONMatched() = %v, want %v",
					got,
					tt.want,
				)
			}
		})
	}
}

func TestIsGRPCMessageMatched(t *testing.T) {
	// FileDescriptorProto is used as a concrete protobuf message.
	// Its Name field has explicit presence, allowing us to test
	// populated and empty protobuf messages.
	tests := []struct {
		name     string
		actual   proto.Message
		expected matcher.GRPCMessage
		want     bool
		wantErr  bool
	}{
		{
			name: "equals populated message",
			actual: &descriptorpb.FileDescriptorProto{
				Name: proto.String("account.proto"),
			},
			expected: matcher.GRPCMessage{
				Operator: matcher.GRPCMessageMatchOperatorEquals,
				Value:    `{"name":"account.proto"}`,
			},
			want: true,
		},
		{
			name:   "equals empty message",
			actual: &descriptorpb.FileDescriptorProto{},
			expected: matcher.GRPCMessage{
				Operator: matcher.GRPCMessageMatchOperatorEquals,
				Value:    `{}`,
			},
			want: true,
		},
		{
			name: "equals mismatch",
			actual: &descriptorpb.FileDescriptorProto{
				Name: proto.String("account.proto"),
			},
			expected: matcher.GRPCMessage{
				Operator: matcher.GRPCMessageMatchOperatorEquals,
				Value:    `{"name":"payment.proto"}`,
			},
		},
		{
			name: "equals populated vs empty",
			actual: &descriptorpb.FileDescriptorProto{
				Name: proto.String("account.proto"),
			},
			expected: matcher.GRPCMessage{
				Operator: matcher.GRPCMessageMatchOperatorEquals,
				Value:    `{}`,
			},
		},
		{
			name:   "equals invalid JSON",
			actual: &descriptorpb.FileDescriptorProto{},
			expected: matcher.GRPCMessage{
				Operator: matcher.GRPCMessageMatchOperatorEquals,
				Value:    `{`,
			},
			wantErr: true,
		},
		{
			name:   "equals unknown field",
			actual: &descriptorpb.FileDescriptorProto{},
			expected: matcher.GRPCMessage{
				Operator: matcher.GRPCMessageMatchOperatorEquals,
				Value:    `{"unknownField":"value"}`,
			},
			wantErr: true,
		},
		{
			name: "nonempty populated message",
			actual: &descriptorpb.FileDescriptorProto{
				Name: proto.String("account.proto"),
			},
			expected: matcher.GRPCMessage{
				Operator: matcher.GRPCMessageMatchOperatorNonEmpty,
			},
			want: true,
		},
		{
			name:   "nonempty empty message",
			actual: &descriptorpb.FileDescriptorProto{},
			expected: matcher.GRPCMessage{
				Operator: matcher.GRPCMessageMatchOperatorNonEmpty,
			},
		},
		{
			name: "nonempty explicitly set empty string",
			actual: &descriptorpb.FileDescriptorProto{
				Name: proto.String(""),
			},
			expected: matcher.GRPCMessage{
				Operator: matcher.GRPCMessageMatchOperatorNonEmpty,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsGRPCMessageMatched(
				tt.actual,
				&tt.expected,
			)

			if (err != nil) != tt.wantErr {
				t.Fatalf(
					"IsGRPCMessageMatched() error = %v, wantErr %v",
					err,
					tt.wantErr,
				)
			}

			if err != nil {
				return
			}

			if got != tt.want {
				t.Errorf(
					"IsGRPCMessageMatched() = %v, want %v",
					got,
					tt.want,
				)
			}
		})
	}
}
