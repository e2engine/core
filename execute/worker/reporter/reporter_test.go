package reporter

import (
	"errors"
	"net/http"
	"net/url"
	"reflect"
	"testing"

	"github.com/e2engine/core/model"
	"github.com/e2engine/core/model/matcher"
)

func TestNew(t *testing.T) {
	r := newReporterForTest(
		&Config{},
		&model.TestSpec{},
	)

	result := r.Result()

	if result.ExecutionID != "execution-1" {
		t.Errorf(
			"expected execution ID %q, got %q",
			"execution-1",
			result.ExecutionID,
		)
	}

	if result.TestSuiteExecutionID != "suite-execution-1" {
		t.Errorf(
			"expected test suite execution ID %q, got %q",
			"suite-execution-1",
			result.TestSuiteExecutionID,
		)
	}

	if result.Summary == nil {
		t.Fatal("expected summary")
	}

	if !result.FinishedAt.IsZero() {
		t.Errorf(
			"expected zero finished time, got %v",
			result.FinishedAt,
		)
	}
}

func TestReporterSetError(t *testing.T) {
	r := newReporterForTest(
		&Config{},
		&model.TestSpec{},
	)

	expectedErr := errors.New("execution failed")

	r.SetError(expectedErr)

	result := r.Result()

	if result.Status != model.ExecutionStatusError {
		t.Errorf(
			"expected status %q, got %q",
			model.ExecutionStatusError,
			result.Status,
		)
	}

	if result.Summary.Error != expectedErr.Error() {
		t.Errorf(
			"expected error %q, got %q",
			expectedErr.Error(),
			result.Summary.Error,
		)
	}

	if result.FinishedAt.IsZero() {
		t.Error("expected non-zero finished time")
	}

	mustPanic(t, func() {
		r.AddDeviation(&model.TestExecutionDeviation{})
	})

	mustPanic(t, func() {
		r.AddPayloadDeviation(&model.PayloadDeviation{})
	})
}

func TestReporterFinalize(t *testing.T) {
	tests := []struct {
		name         string
		addDeviation bool
		expected     model.ExecutionStatus
	}{
		{
			name:     "passed",
			expected: model.ExecutionStatusPassed,
		},
		{
			name:         "failed",
			addDeviation: true,
			expected:     model.ExecutionStatusFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newReporterForTest(
				&Config{},
				&model.TestSpec{},
			)

			if tt.addDeviation {
				r.AddDeviation(
					&model.TestExecutionDeviation{
						Field: "status",
					},
				)
			}

			r.Finalize()

			result := r.Result()

			if result.Status != tt.expected {
				t.Errorf(
					"expected status %q, got %q",
					tt.expected,
					result.Status,
				)
			}

			if result.FinishedAt.IsZero() {
				t.Error("expected non-zero finished time")
			}

			mustPanic(t, func() {
				r.Finalize()
			})
		})
	}
}

func TestReporterAddDeviation(t *testing.T) {
	r := newReporterForTest(
		&Config{},
		&model.TestSpec{},
	)

	r.AddDeviation(nil)

	if len(r.Result().Summary.Deviations) != 0 {
		t.Fatal("expected nil deviation to be ignored")
	}

	deviation := &model.TestExecutionDeviation{
		Field:    "body",
		Expected: "expected",
		Actual:   "actual",
		Message:  "message",
	}

	r.AddDeviation(deviation)

	if len(r.Result().Summary.Deviations) != 1 {
		t.Fatalf(
			"expected 1 deviation, got %d",
			len(r.Result().Summary.Deviations),
		)
	}

	actual := r.Result().Summary.Deviations[0]

	if actual != *deviation {
		t.Errorf(
			"expected deviation %+v, got %+v",
			*deviation,
			actual,
		)
	}

	// Reporter stores the deviation by value.
	deviation.Field = "changed"

	if r.Result().Summary.Deviations[0].Field != "body" {
		t.Error("stored deviation changed with source")
	}
}

func TestReporterAddPayloadDeviation(t *testing.T) {
	tests := []struct {
		name string

		dumpPayload bool
		expected    *matcher.String
		actual      []byte

		wantExpected string
		wantActual   string
	}{
		{
			name: "payload disabled",
			expected: &matcher.String{
				Operator: matcher.StringMatchOperatorEquals,
				Value:    "expected payload",
			},
			actual:       []byte("actual payload"),
			wantExpected: "<dump disabled>",
			wantActual:   "<dump disabled>",
		},
		{
			name:        "payload enabled",
			dumpPayload: true,
			expected: &matcher.String{
				Operator: matcher.StringMatchOperatorEquals,
				Value:    "expected payload",
			},
			actual:       []byte("actual payload"),
			wantExpected: "expected payload",
			wantActual:   "actual payload",
		},
		{
			name: "nonempty matcher with dumping disabled",
			expected: &matcher.String{
				Operator: matcher.StringMatchOperatorNonEmpty,
			},
			actual:       []byte("actual payload"),
			wantExpected: "<dump disabled>",
			wantActual:   "<dump disabled>",
		},
		{
			name:        "nonempty matcher with dumping enabled",
			dumpPayload: true,
			expected: &matcher.String{
				Operator: matcher.StringMatchOperatorNonEmpty,
			},
			actual:       []byte("actual payload"),
			wantExpected: "nonempty",
			wantActual:   "actual payload",
		},
		{
			name: "empty actual payload",
			expected: &matcher.String{
				Operator: matcher.StringMatchOperatorNonEmpty,
			},
			wantExpected: "<dump disabled>",
		},
		{
			name:        "empty actual payload with dumping enabled",
			dumpPayload: true,
			expected: &matcher.String{
				Operator: matcher.StringMatchOperatorNonEmpty,
			},
			wantExpected: "nonempty",
		},
		{
			name:         "nil expected matcher",
			actual:       []byte("actual payload"),
			wantExpected: "<dump disabled>",
			wantActual:   "<dump disabled>",
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

			r.AddPayloadDeviation(nil)

			r.AddPayloadDeviation(&model.PayloadDeviation{
				Field:    "body",
				Expected: tt.expected,
				Actual:   tt.actual,
				Message:  "payload differs",
			})

			deviations := r.Result().Summary.Deviations

			if len(deviations) != 1 {
				t.Fatalf(
					"expected 1 deviation, got %d",
					len(deviations),
				)
			}

			actual := deviations[0]

			if actual.Field != "body" {
				t.Errorf(
					"expected field %q, got %q",
					"body",
					actual.Field,
				)
			}

			if actual.Message != "payload differs" {
				t.Errorf(
					"expected message %q, got %q",
					"payload differs",
					actual.Message,
				)
			}

			if actual.Expected != tt.wantExpected {
				t.Errorf(
					"expected Expected %q, got %q",
					tt.wantExpected,
					actual.Expected,
				)
			}

			if actual.Actual != tt.wantActual {
				t.Errorf(
					"expected Actual %q, got %q",
					tt.wantActual,
					actual.Actual,
				)
			}
		})
	}
}

func TestReporterAddHTTPRequest(t *testing.T) {
	tests := []struct {
		name string

		dumpPayload bool

		expectedBody     string
		expectedBodyJSON string
	}{
		{
			name:             "payload disabled",
			expectedBody:     "<dump disabled>",
			expectedBodyJSON: "<dump disabled>",
		},
		{
			name:             "payload enabled",
			dumpPayload:      true,
			expectedBody:     "request body",
			expectedBodyJSON: `{"name":"test"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := &model.TestSpec{
				Request: model.RequestSpec{
					HTTP: &model.HTTPRequest{
						Body: "request body",
						BodyJSON: `{
							"name": "test"
						}`,
					},
				},
			}

			r := newReporterForTest(
				&Config{
					DumpPayload: tt.dumpPayload,
				},
				spec,
			)

			u, err := url.Parse(
				"http://example.com/resource?a=1",
			)
			if err != nil {
				t.Fatal(err)
			}

			req := &http.Request{
				Method: http.MethodPost,
				URL:    u,
				Header: http.Header{
					"X-Test": {"one", "two"},
				},
			}

			r.AddHTTPRequest(req)

			if r.Result().Summary.Request == nil {
				t.Fatal("expected request summary")
			}

			actual := r.Result().Summary.Request.HTTP

			if actual == nil {
				t.Fatal("expected HTTP request summary")
			}

			if actual.Method != http.MethodPost {
				t.Errorf(
					"expected method %q, got %q",
					http.MethodPost,
					actual.Method,
				)
			}

			if actual.URL != u.String() {
				t.Errorf(
					"expected URL %q, got %q",
					u.String(),
					actual.URL,
				)
			}

			if !reflect.DeepEqual(
				actual.Headers,
				map[string][]string(req.Header),
			) {
				t.Errorf(
					"expected headers %+v, got %+v",
					req.Header,
					actual.Headers,
				)
			}

			if actual.Body != tt.expectedBody {
				t.Errorf(
					"expected body %q, got %q",
					tt.expectedBody,
					actual.Body,
				)
			}

			if actual.BodyJSON != tt.expectedBodyJSON {
				t.Errorf(
					"expected JSON body %q, got %q",
					tt.expectedBodyJSON,
					actual.BodyJSON,
				)
			}

			req.Header["X-Test"][0] = "changed"

			if actual.Headers["X-Test"][0] != "one" {
				t.Error("stored headers changed with source")
			}
		})
	}
}

func TestReporterAddHTTPRequestWithoutBody(t *testing.T) {
	spec := &model.TestSpec{
		Request: model.RequestSpec{
			HTTP: &model.HTTPRequest{},
		},
	}

	r := newReporterForTest(
		&Config{},
		spec,
	)

	req, err := http.NewRequest(
		http.MethodGet,
		"http://example.com/resource",
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	r.AddHTTPRequest(req)

	actual := r.Result().Summary.Request.HTTP

	if actual.Body != "" || actual.BodyJSON != "" {
		t.Errorf(
			"expected omitted body fields, got body=%q body_json=%q",
			actual.Body,
			actual.BodyJSON,
		)
	}
}

func TestReporterAddHTTPResponseStatus(t *testing.T) {
	r := newReporterForTest(
		&Config{},
		&model.TestSpec{},
	)

	r.AddHTTPResponseStatus(http.StatusCreated)

	response := r.Result().Summary.Response

	if response == nil || response.HTTP == nil {
		t.Fatal("expected HTTP response summary")
	}

	if response.HTTP.StatusCode != http.StatusCreated {
		t.Errorf(
			"expected status %d, got %d",
			http.StatusCreated,
			response.HTTP.StatusCode,
		)
	}
}

func TestReporterAddHTTPResponseHeaders(t *testing.T) {
	spec := &model.TestSpec{
		Expect: model.ExpectSpec{
			HTTP: &model.HTTPResponsePattern{
				Headers: map[string][]matcher.String{
					"X-Included": {
						{
							Operator: matcher.StringMatchOperatorEquals,
							Value:    "expected",
						},
					},
					"X-Missing": {
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

	headers := http.Header{
		"X-Included":  {"actual-1", "actual-2"},
		"X-Unrelated": {"secret"},
	}

	r.AddHTTPResponseHeaders(headers)

	actual := r.Result().Summary.Response.HTTP.Headers

	expected := map[string][]string{
		"X-Included": {"actual-1", "actual-2"},
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf(
			"expected headers %+v, got %+v",
			expected,
			actual,
		)
	}

	headers["X-Included"][0] = "changed"

	if actual["X-Included"][0] != "actual-1" {
		t.Error("stored headers changed with source")
	}
}

func TestSelectHeaders(t *testing.T) {
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

		actual   http.Header
		expected map[string][]matcher.String

		want map[string][]string
	}{
		{
			name: "no expected headers",
			actual: http.Header{
				"X-Test": {"value"},
			},
		},
		{
			name: "expected header absent",
			actual: http.Header{
				"X-Test": {"value"},
			},
			expected: map[string][]matcher.String{
				"X-Missing": {equals("expected")},
			},
		},
		{
			name: "selected headers",
			actual: http.Header{
				"X-One":   {"1"},
				"X-Two":   {"2"},
				"X-Other": {"other"},
			},
			expected: map[string][]matcher.String{
				"X-One":     {equals("expected")},
				"X-Two":     {equals("expected")},
				"X-Missing": {equals("expected")},
			},
			want: map[string][]string{
				"X-One": {"1"},
				"X-Two": {"2"},
			},
		},
		{
			name: "exists matcher",
			actual: http.Header{
				"X-Test": {""},
			},
			expected: map[string][]matcher.String{
				"X-Test": {exists},
			},
			want: map[string][]string{
				"X-Test": {""},
			},
		},
		{
			name: "nonempty matcher",
			actual: http.Header{
				"X-Test": {"value"},
			},
			expected: map[string][]matcher.String{
				"X-Test": {nonempty},
			},
			want: map[string][]string{
				"X-Test": {"value"},
			},
		},
		{
			name: "case insensitive header lookup",
			actual: http.Header{
				"X-Request-Id": {"123"},
			},
			expected: map[string][]matcher.String{
				"x-request-id": {nonempty},
			},
			want: map[string][]string{
				"x-request-id": {"123"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := selectHeaders(
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

func TestReporterAddHTTPResponseBody(t *testing.T) {
	tests := []struct {
		name        string
		dumpPayload bool
		body        []byte
		want        string
	}{
		{
			name: "payload disabled",
			body: []byte("body"),
			want: "<dump disabled>",
		},
		{
			name:        "payload enabled",
			dumpPayload: true,
			body:        []byte("body"),
			want:        "body",
		},
		{
			name: "empty body with dumping disabled",
			body: []byte{},
		},
		{
			name:        "empty body with dumping enabled",
			dumpPayload: true,
			body:        []byte{},
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

			r.AddHTTPResponseBody(tt.body)

			response := r.Result().Summary.Response

			if response == nil || response.HTTP == nil {
				t.Fatal("expected HTTP response summary")
			}

			if response.HTTP.Body != tt.want {
				t.Errorf(
					"expected body %q, got %q",
					tt.want,
					response.HTTP.Body,
				)
			}
		})
	}
}

func TestReporterAddHTTPResponseBodyJSON(t *testing.T) {
	tests := []struct {
		name        string
		dumpPayload bool
		body        []byte
		want        string
	}{
		{
			name: "payload disabled",
			body: []byte(`{"name":"test"}`),
			want: "<dump disabled>",
		},
		{
			name:        "payload enabled",
			dumpPayload: true,
			body:        []byte(`{"name":"test"}`),
			want:        `{"name":"test"}`,
		},
		{
			name: "empty body with dumping disabled",
			body: []byte{},
		},
		{
			name:        "empty body with dumping enabled",
			dumpPayload: true,
			body:        []byte{},
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

			r.AddHTTPResponseBodyJSON(tt.body)

			response := r.Result().Summary.Response

			if response == nil || response.HTTP == nil {
				t.Fatal("expected HTTP response summary")
			}

			if response.HTTP.BodyJSON != tt.want {
				t.Errorf(
					"expected JSON body %q, got %q",
					tt.want,
					response.HTTP.BodyJSON,
				)
			}
		})
	}
}

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

func newReporterForTest(
	cfg *Config,
	spec *model.TestSpec,
) *Reporter {
	return New(
		cfg,
		spec,
		"execution-1",
		"suite-execution-1",
	)
}

func mustPanic(
	t *testing.T,
	fn func(),
) {
	t.Helper()

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()

	fn()
}
