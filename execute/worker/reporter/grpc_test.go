package reporter

import (
	"reflect"
	"testing"

	"github.com/e2engine/core/model"
	"github.com/e2engine/core/model/matcher"
)

func TestReporterAddGRPCRequest(t *testing.T) {
	tests := []struct {
		name string

		dumpPayload bool
		wantMessage map[string]any
	}{
		{
			name: "payload disabled",
			wantMessage: map[string]any{
				"<dump disabled>": nil,
			},
		},
		{
			name:        "payload enabled",
			dumpPayload: true,
			wantMessage: map[string]any{
				"name": "test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newReporterForTest(
				&Config{
					DumpPayload: tt.dumpPayload,
				},
				&model.TestSpec{},
			)

			req := &model.GRPCRequest{
				Target:  "127.0.0.1:8080",
				Service: "example.Service",
				Method:  "Create",
				Metadata: map[string][]string{
					"x-test": {"one", "two"},
				},
				Message: map[string]any{
					"name": "test",
				},
			}

			r.AddGRPCRequest(req)

			if r.Result().Summary.Request == nil {
				t.Fatal("expected request summary")
			}

			actual := r.Result().Summary.Request.GRPC

			if actual == nil {
				t.Fatal("expected gRPC request summary")
			}

			if actual.Target != req.Target {
				t.Errorf(
					"expected target %q, got %q",
					req.Target,
					actual.Target,
				)
			}

			if actual.Service != req.Service {
				t.Errorf(
					"expected service %q, got %q",
					req.Service,
					actual.Service,
				)
			}

			if actual.Method != req.Method {
				t.Errorf(
					"expected method %q, got %q",
					req.Method,
					actual.Method,
				)
			}

			if !reflect.DeepEqual(
				actual.Metadata,
				req.Metadata,
			) {
				t.Errorf(
					"expected metadata %+v, got %+v",
					req.Metadata,
					actual.Metadata,
				)
			}

			if !reflect.DeepEqual(
				actual.Message,
				tt.wantMessage,
			) {
				t.Errorf(
					"expected message %+v, got %+v",
					tt.wantMessage,
					actual.Message,
				)
			}

			req.Metadata["x-test"][0] = "changed"
			req.Message["name"] = "changed"

			if actual.Metadata["x-test"][0] != "one" {
				t.Error("stored metadata changed with source")
			}

			if tt.dumpPayload &&
				actual.Message["name"] != "test" {
				t.Error("stored message changed with source")
			}
		})
	}
}

func TestReporterAddMetadataDeviation(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		cfg          Config
		wantExpected string
		wantActual   string
	}{
		{
			name:         "default sensitive metadata",
			key:          "Authorization",
			wantExpected: redacted,
			wantActual:   redacted,
		},
		{
			name: "custom sensitive metadata",
			key:  "X-Private",
			cfg: Config{
				RedactMetadata: []string{" x-private "},
			},
			wantExpected: redacted,
			wantActual:   redacted,
		},
		{
			name:         "ordinary metadata",
			key:          "x-public",
			wantExpected: "expected-token",
			wantActual:   "actual-token",
		},
		{
			name: "redaction explicitly disabled",
			key:  "authorization",
			cfg: Config{
				SkipRedactingHeadersMetadata: true,
			},
			wantExpected: "expected-token",
			wantActual:   "actual-token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newReporterForTest(
				&tt.cfg,
				&model.TestSpec{},
			)

			original := &model.TestExecutionDeviation{
				Field:    "metadata." + tt.key,
				Expected: "expected-token",
				Actual:   "actual-token",
			}

			r.AddMetadataDeviation(tt.key, original)

			deviations := r.Result().Summary.Deviations
			if len(deviations) != 1 {
				t.Fatalf(
					"expected 1 deviation, got %d",
					len(deviations),
				)
			}

			got := deviations[0]

			if got.Field != original.Field ||
				got.Expected != tt.wantExpected ||
				got.Actual != tt.wantActual {
				t.Errorf(
					"unexpected deviation: %+v",
					got,
				)
			}

			// Redaction must not modify the original deviation.
			if original.Expected != "expected-token" ||
				original.Actual != "actual-token" {
				t.Errorf(
					"original deviation was modified: %+v",
					original,
				)
			}
		})
	}
}

func TestReporterAddMetadataDeviationNil(t *testing.T) {
	r := newReporterForTest(
		&Config{},
		&model.TestSpec{},
	)

	r.AddMetadataDeviation("authorization", nil)

	if len(r.Result().Summary.Deviations) != 0 {
		t.Errorf(
			"expected no deviations, got %+v",
			r.Result().Summary.Deviations,
		)
	}
}

func TestReporterAddGRPCRequestMetadataRedaction(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want map[string][]string
	}{
		{
			name: "default sensitive metadata",
			want: map[string][]string{
				"authorization": {redacted},
				"Authorization": {redacted},
				"cookie":        {redacted},
				"x-api-key":     {redacted},
				"x-test":        {"visible"},
			},
		},
		{
			name: "custom metadata",
			cfg: Config{
				RedactMetadata: []string{" X-Test "},
			},
			want: map[string][]string{
				"authorization": {redacted},
				"Authorization": {redacted},
				"cookie":        {redacted},
				"x-api-key":     {redacted},
				"x-test":        {redacted},
			},
		},
		{
			name: "redaction explicitly disabled",
			cfg: Config{
				SkipRedactingHeadersMetadata: true,
			},
			want: map[string][]string{
				"authorization": {"Bearer secret"},
				"Authorization": {"Bearer another-secret"},
				"cookie":        {"sid=secret"},
				"x-api-key":     {"api-secret"},
				"x-test":        {"visible"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newReporterForTest(
				&tt.cfg,
				&model.TestSpec{},
			)

			req := &model.GRPCRequest{
				Target:  "127.0.0.1:8080",
				Service: "example.Service",
				Method:  "Create",
				Metadata: map[string][]string{
					"authorization": {"Bearer secret"},
					"Authorization": {"Bearer another-secret"},
					"cookie":        {"sid=secret"},
					"x-api-key":     {"api-secret"},
					"x-test":        {"visible"},
				},
			}

			original := make(
				map[string][]string,
				len(req.Metadata),
			)

			for key, values := range req.Metadata {
				original[key] = append(
					[]string(nil),
					values...,
				)
			}

			r.AddGRPCRequest(req)

			actual := r.Result().Summary.Request.GRPC
			if actual == nil {
				t.Fatal("expected gRPC request summary")
			}

			if !reflect.DeepEqual(actual.Metadata, tt.want) {
				t.Errorf(
					"expected metadata %+v, got %+v",
					tt.want,
					actual.Metadata,
				)
			}

			// Reporting must not modify the original request.
			if !reflect.DeepEqual(req.Metadata, original) {
				t.Error("request metadata modified by reporting")
			}

			// The report must own its copy of the metadata values.
			req.Metadata["x-test"][0] = "changed"

			if actual.Metadata["x-test"][0] != tt.want["x-test"][0] {
				t.Error("reported metadata changed with source")
			}
		})
	}
}

func TestReporterAddGRPCRequestWithoutMessage(t *testing.T) {
	r := newReporterForTest(
		&Config{},
		&model.TestSpec{},
	)

	r.AddGRPCRequest(&model.GRPCRequest{
		Service: "example.Service",
		Method:  "Create",
	})

	actual := r.Result().Summary.Request.GRPC

	if actual.Message != nil {
		t.Errorf(
			"expected nil message, got %+v",
			actual.Message,
		)
	}
}

func TestReporterAddGRPCResponseStatus(t *testing.T) {
	r := newReporterForTest(
		&Config{},
		&model.TestSpec{},
	)

	r.AddGRPCResponseStatus("OK")

	response := r.Result().Summary.Response

	if response == nil || response.GRPC == nil {
		t.Fatal("expected gRPC response summary")
	}

	if response.GRPC.Status != "OK" {
		t.Errorf(
			"expected status %q, got %q",
			"OK",
			response.GRPC.Status,
		)
	}
}

func TestReporterAddGRPCResponseMetadata(t *testing.T) {
	spec := &model.TestSpec{
		Expect: model.ExpectSpec{
			GRPC: &model.GRPCResponsePattern{
				Metadata: map[string][]matcher.String{
					"x-included": {
						{
							Operator: matcher.StringMatchOperatorEquals,
							Value:    "expected",
						},
					},
					"x-missing": {
						{
							Operator: matcher.StringMatchOperatorExists,
						},
					},
				},
			},
		},
	}

	r := newReporterForTest(
		&Config{},
		spec,
	)

	metadata := map[string][]string{
		"x-included":  {"actual-1", "actual-2"},
		"x-unrelated": {"secret"},
	}

	r.AddGRPCResponseMetadata(metadata)

	actual := r.Result().
		Summary.
		Response.
		GRPC.
		Metadata

	expected := map[string][]string{
		"x-included": {"actual-1", "actual-2"},
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf(
			"expected metadata %+v, got %+v",
			expected,
			actual,
		)
	}

	metadata["x-included"][0] = "changed"

	if actual["x-included"][0] != "actual-1" {
		t.Error("stored metadata changed with source")
	}
}

func TestReporterAddGRPCResponseMetadataRedaction(t *testing.T) {
	exists := matcher.String{
		Operator: matcher.StringMatchOperatorExists,
	}

	tests := []struct {
		name string
		cfg  Config
		want map[string][]string
	}{
		{
			name: "default and custom redaction",
			cfg: Config{
				RedactMetadata: []string{" X-Private "},
			},
			want: map[string][]string{
				"set-cookie": {redacted},
				"x-private":  {redacted},
				"x-public":   {"visible"},
			},
		},
		{
			name: "redaction explicitly disabled",
			cfg: Config{
				RedactMetadata:               []string{"x-private"},
				SkipRedactingHeadersMetadata: true,
			},
			want: map[string][]string{
				"set-cookie": {"sid=secret"},
				"x-private":  {"private-secret"},
				"x-public":   {"visible"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := &model.TestSpec{
				Expect: model.ExpectSpec{
					GRPC: &model.GRPCResponsePattern{
						Metadata: map[string][]matcher.String{
							"set-cookie": {exists},
							"x-private":  {exists},
							"x-public":   {exists},
						},
					},
				},
			}

			r := newReporterForTest(
				&tt.cfg,
				spec,
			)

			metadata := map[string][]string{
				"set-cookie":  {"sid=secret"},
				"x-private":   {"private-secret"},
				"x-public":    {"visible"},
				"x-unrelated": {"unselected-secret"},
			}

			original := make(
				map[string][]string,
				len(metadata),
			)

			for key, values := range metadata {
				original[key] = append(
					[]string(nil),
					values...,
				)
			}

			r.AddGRPCResponseMetadata(metadata)

			response := r.Result().Summary.Response
			if response == nil || response.GRPC == nil {
				t.Fatal("expected gRPC response summary")
			}

			actual := response.GRPC.Metadata

			if !reflect.DeepEqual(actual, tt.want) {
				t.Errorf(
					"expected metadata %+v, got %+v",
					tt.want,
					actual,
				)
			}

			// Response reporting must not mutate source metadata.
			if !reflect.DeepEqual(metadata, original) {
				t.Error("response metadata modified by reporting")
			}

			// Unselected metadata must not appear in the report.
			if _, ok := actual["x-unrelated"]; ok {
				t.Error("unselected metadata included in report")
			}

			// Selected metadata must be copied.
			metadata["x-public"][0] = "changed"

			if actual["x-public"][0] != "visible" {
				t.Error("reported metadata changed with source")
			}
		})
	}
}

func TestSelectMetadata(t *testing.T) {
	equals := func(value string) matcher.String {
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
		name string

		actual   map[string][]string
		expected map[string][]matcher.String

		want map[string][]string
	}{
		{
			name: "no expected metadata",
			actual: map[string][]string{
				"x-test": {"value"},
			},
		},
		{
			name: "expected metadata absent",
			actual: map[string][]string{
				"x-test": {"value"},
			},
			expected: map[string][]matcher.String{
				"x-missing": {equals("expected")},
			},
		},
		{
			name: "selected metadata",
			actual: map[string][]string{
				"x-one":   {"1"},
				"x-two":   {"2"},
				"x-other": {"other"},
			},
			expected: map[string][]matcher.String{
				"x-one":     {equals("expected")},
				"x-two":     {equals("expected")},
				"x-missing": {equals("expected")},
			},
			want: map[string][]string{
				"x-one": {"1"},
				"x-two": {"2"},
			},
		},
		{
			name: "exists matcher",
			actual: map[string][]string{
				"x-test": {""},
			},
			expected: map[string][]matcher.String{
				"x-test": {exists},
			},
			want: map[string][]string{
				"x-test": {""},
			},
		},
		{
			name: "nonempty matcher",
			actual: map[string][]string{
				"x-test": {"value"},
			},
			expected: map[string][]matcher.String{
				"x-test": {nonempty},
			},
			want: map[string][]string{
				"x-test": {"value"},
			},
		},
		{
			name: "expected key casing preserved",
			actual: map[string][]string{
				"x-request-id": {"123"},
			},
			expected: map[string][]matcher.String{
				"X-Request-Id": {exists},
			},
			want: map[string][]string{
				"X-Request-Id": {"123"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := selectMetadata(
				tt.actual,
				tt.expected,
			)

			if !reflect.DeepEqual(actual, tt.want) {
				t.Errorf(
					"expected %+v, got %+v",
					tt.want,
					actual,
				)
			}
		})
	}
}

func TestReporterAddGRPCResponseMessage(t *testing.T) {
	tests := []struct {
		name string

		dumpPayload bool
		message     map[string]any
		want        map[string]any
	}{
		{
			name: "payload disabled",
			message: map[string]any{
				"name": "test",
			},
			want: map[string]any{
				"<dump disabled>": nil,
			},
		},
		{
			name:        "payload enabled",
			dumpPayload: true,
			message: map[string]any{
				"name": "test",
			},
			want: map[string]any{
				"name": "test",
			},
		},
		{
			name:    "empty message with dumping disabled",
			message: map[string]any{},
			want: map[string]any{
				"<none>": nil,
			},
		},
		{
			name:        "empty message with dumping enabled",
			dumpPayload: true,
			message:     map[string]any{},
			want: map[string]any{
				"<none>": nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newReporterForTest(
				&Config{
					DumpPayload: tt.dumpPayload,
				},
				&model.TestSpec{},
			)

			r.AddGRPCResponseMessage(tt.message)

			response := r.Result().Summary.Response

			if response == nil || response.GRPC == nil {
				t.Fatal("expected gRPC response summary")
			}

			if !reflect.DeepEqual(
				response.GRPC.Message,
				tt.want,
			) {
				t.Errorf(
					"expected message %+v, got %+v",
					tt.want,
					response.GRPC.Message,
				)
			}

			if len(tt.message) > 0 {
				tt.message["name"] = "changed"

				if tt.dumpPayload &&
					response.GRPC.Message["name"] != "test" {
					t.Error("stored message changed with source")
				}
			}
		})
	}
}
