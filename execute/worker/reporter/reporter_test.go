package reporter

import (
	"errors"
	"net/http"
	"net/url"
	"reflect"
	"testing"

	"github.com/e2engine/core/model"
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
		r.AddDeviation(
			&model.TestExecutionDeviation{},
		)
	})
}

func TestReporterFinalize(t *testing.T) {
	tests := []struct {
		name string

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
	deviation := &model.TestExecutionDeviation{
		Field:    "body",
		Expected: "expected payload",
		Actual:   "actual payload",
		Message:  "payload differs",
	}

	tests := []struct {
		name string

		dumpPayload bool

		expectedExpected string
		expectedActual   string
	}{
		{
			name: "payload disabled",
		},
		{
			name:             "payload enabled",
			dumpPayload:      true,
			expectedExpected: deviation.Expected,
			expectedActual:   deviation.Actual,
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

			r.AddPayloadDeviation(deviation)

			if len(r.Result().Summary.Deviations) != 1 {
				t.Fatalf(
					"expected 1 deviation, got %d",
					len(r.Result().Summary.Deviations),
				)
			}

			actual := r.Result().Summary.Deviations[0]

			if actual.Field != deviation.Field {
				t.Errorf(
					"expected field %q, got %q",
					deviation.Field,
					actual.Field,
				)
			}

			if actual.Message != deviation.Message {
				t.Errorf(
					"expected message %q, got %q",
					deviation.Message,
					actual.Message,
				)
			}

			if actual.Expected != tt.expectedExpected {
				t.Errorf(
					"expected Expected %q, got %q",
					tt.expectedExpected,
					actual.Expected,
				)
			}

			if actual.Actual != tt.expectedActual {
				t.Errorf(
					"expected Actual %q, got %q",
					tt.expectedActual,
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
			name: "payload disabled",
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
					"expected headers %#+v, got %#+v",
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
				t.Error(
					"stored headers changed with source",
				)
			}
		})
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
			HTTP: &model.HTTPResponse{
				Headers: map[string][]string{
					"X-Included": {"expected"},
					"X-Missing":  {"expected"},
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
		t.Error(
			"stored headers changed with source",
		)
	}
}

func TestSelectHeaders(t *testing.T) {
	tests := []struct {
		name string

		actual   http.Header
		expected map[string][]string

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
			expected: map[string][]string{
				"X-Missing": {"expected"},
			},
		},
		{
			name: "selected headers",
			actual: http.Header{
				"X-One":   {"1"},
				"X-Two":   {"2"},
				"X-Other": {"other"},
			},
			expected: map[string][]string{
				"X-One":     {"expected"},
				"X-Two":     {"expected"},
				"X-Missing": {"expected"},
			},
			want: map[string][]string{
				"X-One": {"1"},
				"X-Two": {"2"},
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
	t.Run("payload disabled", func(t *testing.T) {
		r := newReporterForTest(
			&Config{},
			&model.TestSpec{},
		)

		r.AddHTTPResponseBody("body")

		if r.Result().Summary.Response != nil {
			t.Fatal(
				"expected response summary not to be created",
			)
		}
	})

	t.Run("payload enabled", func(t *testing.T) {
		r := newReporterForTest(
			&Config{
				DumpPayload: true,
			},
			&model.TestSpec{},
		)

		r.AddHTTPResponseBody("")

		response := r.Result().Summary.Response

		if response == nil || response.HTTP == nil {
			t.Fatal("expected HTTP response summary")
		}

		if response.HTTP.Body == nil {
			t.Fatal(
				"expected non-nil body pointer",
			)
		}

		if *response.HTTP.Body != "" {
			t.Errorf(
				"expected empty body, got %q",
				*response.HTTP.Body,
			)
		}
	})
}

func TestReporterAddHTTPResponseBodyJSON(t *testing.T) {
	t.Run("payload disabled", func(t *testing.T) {
		r := newReporterForTest(
			&Config{},
			&model.TestSpec{},
		)

		r.AddHTTPResponseBodyJSON("{}")

		if r.Result().Summary.Response != nil {
			t.Fatal(
				"expected response summary not to be created",
			)
		}
	})

	t.Run("payload enabled", func(t *testing.T) {
		r := newReporterForTest(
			&Config{
				DumpPayload: true,
			},
			&model.TestSpec{},
		)

		r.AddHTTPResponseBodyJSON("")

		response := r.Result().Summary.Response

		if response == nil || response.HTTP == nil {
			t.Fatal("expected HTTP response summary")
		}

		if response.HTTP.BodyJSON == nil {
			t.Fatal(
				"expected non-nil JSON body pointer",
			)
		}

		if *response.HTTP.BodyJSON != "" {
			t.Errorf(
				"expected empty JSON body, got %q",
				*response.HTTP.BodyJSON,
			)
		}
	})
}

func TestReporterAddGRPCRequest(t *testing.T) {
	tests := []struct {
		name string

		dumpPayload   bool
		expectMessage bool
	}{
		{
			name: "payload disabled",
		},
		{
			name:          "payload enabled",
			dumpPayload:   true,
			expectMessage: true,
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

			if tt.expectMessage {
				if !reflect.DeepEqual(
					actual.Message,
					req.Message,
				) {
					t.Errorf(
						"expected message %+v, got %+v",
						req.Message,
						actual.Message,
					)
				}
			} else if actual.Message != nil {
				t.Errorf(
					"expected nil message, got %+v",
					actual.Message,
				)
			}

			req.Metadata["x-test"][0] = "changed"
			req.Message["name"] = "changed"

			if actual.Metadata["x-test"][0] != "one" {
				t.Error(
					"stored metadata changed with source",
				)
			}

			if tt.expectMessage &&
				actual.Message["name"] != "test" {
				t.Error(
					"stored message changed with source",
				)
			}
		})
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
			GRPC: &model.GRPCResponse{
				Metadata: map[string][]string{
					"x-included": {"expected"},
					"x-missing":  {"expected"},
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
		t.Error(
			"stored metadata changed with source",
		)
	}
}

func TestSelectMetadata(t *testing.T) {
	tests := []struct {
		name string

		actual   map[string][]string
		expected map[string][]string

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
			expected: map[string][]string{
				"x-missing": {"expected"},
			},
		},
		{
			name: "selected metadata",
			actual: map[string][]string{
				"x-one":   {"1"},
				"x-two":   {"2"},
				"x-other": {"other"},
			},
			expected: map[string][]string{
				"x-one":     {"expected"},
				"x-two":     {"expected"},
				"x-missing": {"expected"},
			},
			want: map[string][]string{
				"x-one": {"1"},
				"x-two": {"2"},
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
	t.Run("payload disabled", func(t *testing.T) {
		r := newReporterForTest(
			&Config{},
			&model.TestSpec{},
		)

		r.AddGRPCResponseMessage(
			map[string]any{
				"name": "test",
			},
		)

		if r.Result().Summary.Response != nil {
			t.Fatal(
				"expected response summary not to be created",
			)
		}
	})

	t.Run("payload enabled", func(t *testing.T) {
		r := newReporterForTest(
			&Config{
				DumpPayload: true,
			},
			&model.TestSpec{},
		)

		message := map[string]any{
			"name": "test",
		}

		r.AddGRPCResponseMessage(message)

		response := r.Result().Summary.Response

		if response == nil || response.GRPC == nil {
			t.Fatal("expected gRPC response summary")
		}

		if response.GRPC.Message == nil {
			t.Fatal(
				"expected non-nil message pointer",
			)
		}

		if !reflect.DeepEqual(
			*response.GRPC.Message,
			message,
		) {
			t.Errorf(
				"expected message %+v, got %+v",
				message,
				*response.GRPC.Message,
			)
		}

		message["name"] = "changed"

		if (*response.GRPC.Message)["name"] != "test" {
			t.Error(
				"stored message changed with source",
			)
		}
	})

	t.Run("explicit empty message", func(t *testing.T) {
		r := newReporterForTest(
			&Config{
				DumpPayload: true,
			},
			&model.TestSpec{},
		)

		r.AddGRPCResponseMessage(
			map[string]any{},
		)

		response := r.Result().Summary.Response

		if response == nil || response.GRPC == nil {
			t.Fatal("expected gRPC response summary")
		}

		if response.GRPC.Message == nil {
			t.Fatal(
				"expected non-nil message pointer",
			)
		}

		if len(*response.GRPC.Message) != 0 {
			t.Errorf(
				"expected empty message, got %+v",
				*response.GRPC.Message,
			)
		}
	})
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
