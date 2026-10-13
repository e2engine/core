package matcher

import (
	"encoding/json"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestGRPCMessageMarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		matcher GRPCMessage
		want    string
	}{
		{
			name: "equals",
			matcher: GRPCMessage{
				Operator: GRPCMessageMatchOperatorEquals,
				Value:    `{"id":"user-123","name":"John"}`,
			},
			want: `"{\"id\":\"user-123\",\"name\":\"John\"}"`,
		},
		{
			name: "equals empty message",
			matcher: GRPCMessage{
				Operator: GRPCMessageMatchOperatorEquals,
				Value:    `{}`,
			},
			want: `"{}"`,
		},
		{
			name: "nonempty",
			matcher: GRPCMessage{
				Operator: GRPCMessageMatchOperatorNonEmpty,
			},
			want: `{"operator":"nonempty"}`,
		},
		{
			name: "expanded matcher preserves value",
			matcher: GRPCMessage{
				Operator: GRPCMessageMatchOperatorNonEmpty,
				Value:    `{"id":"user-123"}`,
			},
			want: `{"operator":"nonempty","value":"{\"id\":\"user-123\"}"}`,
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

func TestGRPCMessageUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		data string
		want GRPCMessage
	}{
		{
			name: "equals",
			data: `"{\"id\":\"user-123\",\"name\":\"John\"}"`,
			want: GRPCMessage{
				Operator: GRPCMessageMatchOperatorEquals,
				Value:    `{"id":"user-123","name":"John"}`,
			},
		},
		{
			name: "equals empty message",
			data: `"{}"`,
			want: GRPCMessage{
				Operator: GRPCMessageMatchOperatorEquals,
				Value:    `{}`,
			},
		},
		{
			name: "nonempty",
			data: `{"operator":"nonempty"}`,
			want: GRPCMessage{
				Operator: GRPCMessageMatchOperatorNonEmpty,
			},
		},
		{
			name: "expanded equals",
			data: `{"operator":"equals","value":"{\"id\":\"user-123\"}"}`,
			want: GRPCMessage{
				Operator: GRPCMessageMatchOperatorEquals,
				Value:    `{"id":"user-123"}`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got GRPCMessage
			if err := json.Unmarshal([]byte(tt.data), &got); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Unmarshal() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestGRPCMessageUnmarshalJSONInvalid(t *testing.T) {
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
			var matcher GRPCMessage
			if err := json.Unmarshal([]byte(tt.data), &matcher); err == nil {
				t.Fatal("Unmarshal() error = nil, want error")
			}
		})
	}
}

func TestGRPCMessageMarshalYAML(t *testing.T) {
	tests := []struct {
		name    string
		matcher GRPCMessage
		want    string
	}{
		{
			name: "equals",
			matcher: GRPCMessage{
				Operator: GRPCMessageMatchOperatorEquals,
				Value:    `{"id":"user-123","name":"John"}`,
			},
			want: `'{"id":"user-123","name":"John"}'` + "\n",
		},
		{
			name: "equals empty message",
			matcher: GRPCMessage{
				Operator: GRPCMessageMatchOperatorEquals,
				Value:    `{}`,
			},
			want: "'{}'\n",
		},
		{
			name: "nonempty",
			matcher: GRPCMessage{
				Operator: GRPCMessageMatchOperatorNonEmpty,
			},
			want: "operator: nonempty\n",
		},
		{
			name: "expanded matcher preserves value",
			matcher: GRPCMessage{
				Operator: GRPCMessageMatchOperatorNonEmpty,
				Value:    `{"id":"user-123"}`,
			},
			want: "operator: nonempty\nvalue: '{\"id\":\"user-123\"}'\n",
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

func TestGRPCMessageUnmarshalYAML(t *testing.T) {
	tests := []struct {
		name string
		data string
		want GRPCMessage
	}{
		{
			name: "equals",
			data: `message: '{"id":"user-123","name":"John"}'`,
			want: GRPCMessage{
				Operator: GRPCMessageMatchOperatorEquals,
				Value:    `{"id":"user-123","name":"John"}`,
			},
		},
		{
			name: "equals empty message",
			data: `message: '{}'`,
			want: GRPCMessage{
				Operator: GRPCMessageMatchOperatorEquals,
				Value:    `{}`,
			},
		},
		{
			name: "nonempty",
			data: `
message:
  operator: nonempty
`,
			want: GRPCMessage{
				Operator: GRPCMessageMatchOperatorNonEmpty,
			},
		},
		{
			name: "expanded equals",
			data: `
message:
  operator: equals
  value: '{"id":"user-123"}'
`,
			want: GRPCMessage{
				Operator: GRPCMessageMatchOperatorEquals,
				Value:    `{"id":"user-123"}`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var document struct {
				Message GRPCMessage `yaml:"message"`
			}

			if err := yaml.Unmarshal([]byte(tt.data), &document); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}

			if !reflect.DeepEqual(document.Message, tt.want) {
				t.Errorf(
					"Unmarshal() = %#v, want %#v",
					document.Message,
					tt.want,
				)
			}
		})
	}
}

func TestGRPCMessageUnmarshalYAMLInvalid(t *testing.T) {
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
			var matcher GRPCMessage
			if err := yaml.Unmarshal([]byte(tt.data), &matcher); err == nil {
				t.Fatal("Unmarshal() error = nil, want error")
			}
		})
	}
}

func TestGRPCMessageRoundTripJSON(t *testing.T) {
	values := []GRPCMessage{
		{
			Operator: GRPCMessageMatchOperatorEquals,
			Value:    `{"id":"user-123","name":"John"}`,
		},
		{
			Operator: GRPCMessageMatchOperatorEquals,
			Value:    `{}`,
		},
		{
			Operator: GRPCMessageMatchOperatorNonEmpty,
		},
	}

	for _, want := range values {
		data, err := json.Marshal(want)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}

		var got GRPCMessage
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}

		if !reflect.DeepEqual(got, want) {
			t.Errorf("round trip = %#v, want %#v", got, want)
		}
	}
}

func TestGRPCMessageRoundTripYAML(t *testing.T) {
	values := []GRPCMessage{
		{
			Operator: GRPCMessageMatchOperatorEquals,
			Value:    `{"id":"user-123","name":"John"}`,
		},
		{
			Operator: GRPCMessageMatchOperatorEquals,
			Value:    `{}`,
		},
		{
			Operator: GRPCMessageMatchOperatorNonEmpty,
		},
	}

	for _, want := range values {
		data, err := yaml.Marshal(want)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}

		var got GRPCMessage
		if err := yaml.Unmarshal(data, &got); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}

		if !reflect.DeepEqual(got, want) {
			t.Errorf("round trip = %#v, want %#v", got, want)
		}
	}
}

func TestGRPCMessageString(t *testing.T) {
	tests := []struct {
		name    string
		matcher GRPCMessage
		want    string
	}{
		{
			name: "equals",
			matcher: GRPCMessage{
				Operator: GRPCMessageMatchOperatorEquals,
				Value:    `{"id":"user-123"}`,
			},
			want: `{"id":"user-123"}`,
		},
		{
			name: "equals empty message",
			matcher: GRPCMessage{
				Operator: GRPCMessageMatchOperatorEquals,
				Value:    `{}`,
			},
			want: `{}`,
		},
		{
			name: "nonempty",
			matcher: GRPCMessage{
				Operator: GRPCMessageMatchOperatorNonEmpty,
			},
			want: "nonempty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.matcher.String(); got != tt.want {
				t.Errorf(
					"String() = %q, want %q",
					got,
					tt.want,
				)
			}
		})
	}
}

func TestValidateGRPCMessageMatcher(t *testing.T) {
	tests := []struct {
		name    string
		matcher GRPCMessage
		wantErr bool
	}{
		{
			name: "equals valid JSON object",
			matcher: GRPCMessage{
				Operator: GRPCMessageMatchOperatorEquals,
				Value:    `{"id":"user-123"}`,
			},
		},
		{
			name: "equals empty protobuf message",
			matcher: GRPCMessage{
				Operator: GRPCMessageMatchOperatorEquals,
				Value:    `{}`,
			},
		},
		{
			name: "equals invalid JSON",
			matcher: GRPCMessage{
				Operator: GRPCMessageMatchOperatorEquals,
				Value:    `{`,
			},
			wantErr: true,
		},
		{
			name: "equals empty string",
			matcher: GRPCMessage{
				Operator: GRPCMessageMatchOperatorEquals,
			},
			wantErr: true,
		},
		{
			name: "nonempty",
			matcher: GRPCMessage{
				Operator: GRPCMessageMatchOperatorNonEmpty,
			},
		},
		{
			name: "nonempty with value",
			matcher: GRPCMessage{
				Operator: GRPCMessageMatchOperatorNonEmpty,
				Value:    `{}`,
			},
			wantErr: true,
		},
		{
			name: "unknown operator",
			matcher: GRPCMessage{
				Operator: "contains",
			},
			wantErr: true,
		},
		{
			name: "missing operator",
			matcher: GRPCMessage{
				Value: `{}`,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGRPCMessageMatcher(tt.matcher)
			if (err != nil) != tt.wantErr {
				t.Errorf(
					"validateGRPCMessageMatcher() error = %v, wantErr %v",
					err,
					tt.wantErr,
				)
			}
		})
	}
}
