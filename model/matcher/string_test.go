package matcher

import (
	"encoding/json"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestStringMarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		matcher String
		want    string
	}{
		{
			name: "equals",
			matcher: String{
				Operator: StringMatchOperatorEquals,
				Value:    "application/json",
			},
			want: `"application/json"`,
		},
		{
			name: "equals empty",
			matcher: String{
				Operator: StringMatchOperatorEquals,
				Value:    "",
			},
			want: `""`,
		},
		{
			name: "exists",
			matcher: String{
				Operator: StringMatchOperatorExists,
			},
			want: `{"operator":"exists"}`,
		},
		{
			name: "nonempty",
			matcher: String{
				Operator: StringMatchOperatorNonEmpty,
			},
			want: `{"operator":"nonempty"}`,
		},
		{
			name: "expanded matcher preserves value",
			matcher: String{
				Operator: StringMatchOperatorNonEmpty,
				Value:    "value",
			},
			want: `{"operator":"nonempty","value":"value"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.matcher)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}

			if string(got) != tt.want {
				t.Errorf("Marshal() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestStringUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		data string
		want String
	}{
		{
			name: "equals",
			data: `"application/json"`,
			want: String{
				Operator: StringMatchOperatorEquals,
				Value:    "application/json",
			},
		},
		{
			name: "equals empty",
			data: `""`,
			want: String{
				Operator: StringMatchOperatorEquals,
				Value:    "",
			},
		},
		{
			name: "exists",
			data: `{"operator":"exists"}`,
			want: String{
				Operator: StringMatchOperatorExists,
			},
		},
		{
			name: "nonempty",
			data: `{"operator":"nonempty"}`,
			want: String{
				Operator: StringMatchOperatorNonEmpty,
			},
		},
		{
			name: "expanded equals",
			data: `{"operator":"equals","value":"application/json"}`,
			want: String{
				Operator: StringMatchOperatorEquals,
				Value:    "application/json",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got String
			if err := json.Unmarshal([]byte(tt.data), &got); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Unmarshal() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestStringUnmarshalJSONInvalid(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{
			name: "number",
			data: `123`,
		},
		{
			name: "boolean",
			data: `true`,
		},
		{
			name: "array",
			data: `["value"]`,
		},
		{
			name: "invalid JSON",
			data: `{`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var matcher String
			if err := json.Unmarshal([]byte(tt.data), &matcher); err == nil {
				t.Fatal("Unmarshal() error = nil, want error")
			}
		})
	}
}

func TestStringMarshalYAML(t *testing.T) {
	tests := []struct {
		name    string
		matcher String
		want    string
	}{
		{
			name: "equals",
			matcher: String{
				Operator: StringMatchOperatorEquals,
				Value:    "application/json",
			},
			want: "application/json\n",
		},
		{
			name: "equals empty",
			matcher: String{
				Operator: StringMatchOperatorEquals,
				Value:    "",
			},
			want: `""` + "\n",
		},
		{
			name: "exists",
			matcher: String{
				Operator: StringMatchOperatorExists,
			},
			want: "operator: exists\n",
		},
		{
			name: "nonempty",
			matcher: String{
				Operator: StringMatchOperatorNonEmpty,
			},
			want: "operator: nonempty\n",
		},
		{
			name: "expanded matcher preserves value",
			matcher: String{
				Operator: StringMatchOperatorNonEmpty,
				Value:    "value",
			},
			want: "operator: nonempty\nvalue: value\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := yaml.Marshal(tt.matcher)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}

			if string(got) != tt.want {
				t.Errorf(
					"Marshal() = %q, want %q",
					string(got),
					tt.want,
				)
			}
		})
	}
}

func TestStringUnmarshalYAML(t *testing.T) {
	tests := []struct {
		name string
		data string
		want String
	}{
		{
			name: "equals",
			data: "application/json\n",
			want: String{
				Operator: StringMatchOperatorEquals,
				Value:    "application/json",
			},
		},
		{
			name: "equals empty",
			data: `""`,
			want: String{
				Operator: StringMatchOperatorEquals,
				Value:    "",
			},
		},
		{
			name: "exists",
			data: "operator: exists\n",
			want: String{
				Operator: StringMatchOperatorExists,
			},
		},
		{
			name: "nonempty",
			data: "operator: nonempty\n",
			want: String{
				Operator: StringMatchOperatorNonEmpty,
			},
		},
		{
			name: "expanded equals",
			data: "operator: equals\nvalue: application/json\n",
			want: String{
				Operator: StringMatchOperatorEquals,
				Value:    "application/json",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got String
			if err := yaml.Unmarshal([]byte(tt.data), &got); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Unmarshal() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestStringUnmarshalYAMLInvalid(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{
			name: "integer",
			data: "123\n",
		},
		{
			name: "boolean",
			data: "true\n",
		},
		{
			name: "sequence",
			data: "- value\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var matcher String
			if err := yaml.Unmarshal([]byte(tt.data), &matcher); err == nil {
				t.Fatal("Unmarshal() error = nil, want error")
			}
		})
	}
}

func TestStringRoundTripJSON(t *testing.T) {
	values := []String{
		{
			Operator: StringMatchOperatorEquals,
			Value:    "application/json",
		},
		{
			Operator: StringMatchOperatorEquals,
			Value:    "",
		},
		{
			Operator: StringMatchOperatorExists,
		},
		{
			Operator: StringMatchOperatorNonEmpty,
		},
	}

	for _, want := range values {
		data, err := json.Marshal(want)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}

		var got String
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}

		if !reflect.DeepEqual(got, want) {
			t.Errorf(
				"round trip = %#v, want %#v",
				got,
				want,
			)
		}
	}
}

func TestStringRoundTripYAML(t *testing.T) {
	values := []String{
		{
			Operator: StringMatchOperatorEquals,
			Value:    "application/json",
		},
		{
			Operator: StringMatchOperatorEquals,
			Value:    "",
		},
		{
			Operator: StringMatchOperatorExists,
		},
		{
			Operator: StringMatchOperatorNonEmpty,
		},
	}

	for _, want := range values {
		data, err := yaml.Marshal(want)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}

		var got String
		if err := yaml.Unmarshal(data, &got); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}

		if !reflect.DeepEqual(got, want) {
			t.Errorf(
				"round trip = %#v, want %#v",
				got,
				want,
			)
		}
	}
}

func TestStringString(t *testing.T) {
	tests := []struct {
		name    string
		matcher String
		want    string
	}{
		{
			name: "equals",
			matcher: String{
				Operator: StringMatchOperatorEquals,
				Value:    "hello",
			},
			want: "hello",
		},
		{
			name: "equals empty",
			matcher: String{
				Operator: StringMatchOperatorEquals,
			},
			want: "",
		},
		{
			name: "exists",
			matcher: String{
				Operator: StringMatchOperatorExists,
			},
			want: "exists",
		},
		{
			name: "nonempty",
			matcher: String{
				Operator: StringMatchOperatorNonEmpty,
			},
			want: "nonempty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.matcher.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateStringMatcher(t *testing.T) {
	tests := []struct {
		name    string
		matcher String
		wantErr bool
	}{
		{
			name: "equals",
			matcher: String{
				Operator: StringMatchOperatorEquals,
				Value:    "hello",
			},
		},
		{
			name: "equals empty",
			matcher: String{
				Operator: StringMatchOperatorEquals,
			},
		},
		{
			name: "exists",
			matcher: String{
				Operator: StringMatchOperatorExists,
			},
		},
		{
			name: "nonempty",
			matcher: String{
				Operator: StringMatchOperatorNonEmpty,
			},
		},
		{
			name: "exists with value",
			matcher: String{
				Operator: StringMatchOperatorExists,
				Value:    "value",
			},
			wantErr: true,
		},
		{
			name: "nonempty with value",
			matcher: String{
				Operator: StringMatchOperatorNonEmpty,
				Value:    "value",
			},
			wantErr: true,
		},
		{
			name: "unknown operator",
			matcher: String{
				Operator: "contains",
			},
			wantErr: true,
		},
		{
			name: "missing operator",
			matcher: String{
				Value: "hello",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateStringMatcher(tt.matcher)

			if (err != nil) != tt.wantErr {
				t.Errorf(
					"validateStringMatcher() error = %v, wantErr %v",
					err,
					tt.wantErr,
				)
			}
		})
	}
}
