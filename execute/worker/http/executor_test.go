package http

import (
	"context"
	nativeerrors "errors"
	"io"
	nethttp "net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	e2enginehttp "github.com/e2engine/instrumentation-go/http"

	"github.com/e2engine/core/execute"
	"github.com/e2engine/core/model"
)

func TestBuildHTTPRequest(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		spec *model.HTTPRequestSpec

		expectedMethod      string
		expectedURL         string
		expectedBody        string
		expectedContentType string
	}{
		{
			name: "basic request",
			spec: &model.HTTPRequestSpec{
				Method: nethttp.MethodGet,
				URL:    "https://example.com/users",
			},
			expectedMethod: nethttp.MethodGet,
			expectedURL:    "https://example.com/users",
		},
		{
			name: "body gets default content type",
			spec: &model.HTTPRequestSpec{
				Method: nethttp.MethodPost,
				URL:    "https://example.com/users",
				Body:   `{"id":42}`,
			},
			expectedMethod:      nethttp.MethodPost,
			expectedURL:         "https://example.com/users",
			expectedBody:        `{"id":42}`,
			expectedContentType: "application/json",
		},
		{
			name: "explicit content type is preserved",
			spec: &model.HTTPRequestSpec{
				Method: nethttp.MethodPost,
				URL:    "https://example.com/users",
				Headers: nethttp.Header{
					"Content-Type": {"application/problem+json"},
				},
				Body: `{"id":42}`,
			},
			expectedMethod:      nethttp.MethodPost,
			expectedURL:         "https://example.com/users",
			expectedBody:        `{"id":42}`,
			expectedContentType: "application/problem+json",
		},
		{
			name: "empty body does not get content type",
			spec: &model.HTTPRequestSpec{
				Method: nethttp.MethodPost,
				URL:    "https://example.com/users",
			},
			expectedMethod: nethttp.MethodPost,
			expectedURL:    "https://example.com/users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := buildHTTPRequest(
				ctx,
				tt.spec,
				"execution-123",
			)
			if err != nil {
				t.Fatalf(
					"buildHTTPRequest() error = %v",
					err,
				)
			}

			if req.Method != tt.expectedMethod {
				t.Errorf(
					"expected method %q, got %q",
					tt.expectedMethod,
					req.Method,
				)
			}

			if req.URL.String() != tt.expectedURL {
				t.Errorf(
					"expected URL %q, got %q",
					tt.expectedURL,
					req.URL.String(),
				)
			}

			if req.Header.Get("Content-Type") !=
				tt.expectedContentType {
				t.Errorf(
					"expected Content-Type %q, got %q",
					tt.expectedContentType,
					req.Header.Get("Content-Type"),
				)
			}

			if id := e2enginehttp.TestExecutionID(req.Header); id != "execution-123" {
				t.Errorf(
					"expected execution ID header %q, got %q",
					"execution-123",
					id,
				)
			}

			if req.Context() != ctx {
				t.Error("expected request to use provided context")
			}

			if tt.expectedBody == "" {
				if req.Body != nil {
					body, err := io.ReadAll(req.Body)
					if err != nil {
						t.Fatalf(
							"cannot read body: %v",
							err,
						)
					}

					if len(body) != 0 {
						t.Errorf(
							"expected empty body, got %q",
							body,
						)
					}
				}

				return
			}

			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf(
					"cannot read body: %v",
					err,
				)
			}

			if string(body) != tt.expectedBody {
				t.Errorf(
					"expected body %q, got %q",
					tt.expectedBody,
					body,
				)
			}
		})
	}
}

func TestBuildHTTPRequestQuery(t *testing.T) {
	spec := &model.HTTPRequestSpec{
		Method: nethttp.MethodGet,
		URL: "https://example.com/users?" +
			"existing=value&override=old",
		Query: map[string]string{
			"added":    "new",
			"override": "new",
		},
	}

	req, err := buildHTTPRequest(
		context.Background(),
		spec,
		"execution-123",
	)
	if err != nil {
		t.Fatalf(
			"buildHTTPRequest() error = %v",
			err,
		)
	}

	query := req.URL.Query()

	if query.Get("existing") != "value" {
		t.Errorf(
			"expected existing query parameter %q, got %q",
			"value",
			query.Get("existing"),
		)
	}

	if query.Get("added") != "new" {
		t.Errorf(
			"expected added query parameter %q, got %q",
			"new",
			query.Get("added"),
		)
	}

	if query.Get("override") != "new" {
		t.Errorf(
			"expected overridden query parameter %q, got %q",
			"new",
			query.Get("override"),
		)
	}
}

func TestBuildHTTPRequestHeaders(t *testing.T) {
	spec := &model.HTTPRequestSpec{
		Method: nethttp.MethodGet,
		URL:    "https://example.com/users",
		Headers: e2enginehttp.WithTestExecutionID(
			nethttp.Header{
				"X-Test": {
					"value-1",
					"value-2",
				},
			},
			"user-value",
		),
	}

	req, err := buildHTTPRequest(
		context.Background(),
		spec,
		"execution-123",
	)
	if err != nil {
		t.Fatalf(
			"buildHTTPRequest() error = %v",
			err,
		)
	}

	if !reflect.DeepEqual(
		req.Header.Values("X-Test"),
		[]string{"value-1", "value-2"},
	) {
		t.Errorf(
			"unexpected X-Test headers: %#v",
			req.Header.Values("X-Test"),
		)
	}

	if id := e2enginehttp.TestExecutionID(req.Header); id != "execution-123" {
		t.Errorf(
			"expected execution header to be overridden, got %s",
			id,
		)
	}
}

func TestBuildHTTPRequestCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	req, err := buildHTTPRequest(
		ctx,
		&model.HTTPRequestSpec{
			Method: nethttp.MethodGet,
			URL:    "https://example.com",
		},
		"execution-123",
	)
	if err != nil {
		t.Fatalf(
			"buildHTTPRequest() error = %v",
			err,
		)
	}

	if req.Context().Err() != context.Canceled {
		t.Errorf(
			"expected context.Canceled, got %v",
			req.Context().Err(),
		)
	}
}

func TestDoHTTPRequest(t *testing.T) {
	server := httptest.NewServer(
		nethttp.HandlerFunc(
			func(
				w nethttp.ResponseWriter,
				r *nethttp.Request,
			) {
				w.WriteHeader(nethttp.StatusCreated)

				_, _ = w.Write(
					[]byte("response"),
				)
			},
		),
	)
	defer server.Close()

	req, err := nethttp.NewRequestWithContext(
		t.Context(),
		nethttp.MethodGet,
		server.URL,
		nil,
	)
	if err != nil {
		t.Fatalf(
			"cannot create request: %v",
			err,
		)
	}

	resp, err := doHTTPRequest(
		server.Client(),
		req,
	)
	if err != nil {
		t.Fatalf(
			"doHTTPRequest() error = %v",
			err,
		)
	}
	defer resp.Body.Close()

	if resp.StatusCode != nethttp.StatusCreated {
		t.Errorf(
			"expected status %d, got %d",
			nethttp.StatusCreated,
			resp.StatusCode,
		)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf(
			"cannot read response body: %v",
			err,
		)
	}

	if string(body) != "response" {
		t.Errorf(
			"expected body %q, got %q",
			"response",
			body,
		)
	}
}

func TestDoHTTPRequestCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	req, err := nethttp.NewRequestWithContext(
		ctx,
		nethttp.MethodGet,
		"http://example.com",
		nil,
	)
	if err != nil {
		t.Fatalf(
			"cannot create request: %v",
			err,
		)
	}

	resp, err := doHTTPRequest(
		nethttp.DefaultClient,
		req,
	)

	if resp != nil {
		defer resp.Body.Close()
	}

	if err == nil {
		t.Fatal("expected error")
	}

	if !nativeerrors.Is(err, context.Canceled) {
		t.Errorf(
			"expected context.Canceled, got %v",
			err,
		)
	}
}

func TestExecutorExecute(t *testing.T) {
	server := httptest.NewServer(
		nethttp.HandlerFunc(
			func(
				w nethttp.ResponseWriter,
				r *nethttp.Request,
			) {
				if id := e2enginehttp.TestExecutionID(r.Header); id != "execution-123" {
					t.Errorf(
						"expected execution ID %q, got %q",
						"execution-123",
						id,
					)
				}

				w.Header().Set(
					"Content-Type",
					"application/json",
				)
				w.WriteHeader(nethttp.StatusOK)

				_, _ = w.Write(
					[]byte(`{"id":42,"name":"test"}`),
				)
			},
		),
	)
	defer server.Close()

	executor := NewTestExecutor(
		server.Client(),
		NewEvaluator(),
	)

	job := &execute.TestJob{
		ExecutionID: "execution-123",
		Test: model.Test{
			Spec: model.TestSpec{
				Request: model.RequestSpec{
					HTTP: &model.HTTPRequestSpec{
						Method: nethttp.MethodGet,
						URL:    server.URL,
					},
				},
				Expect: model.ExpectSpec{
					HTTP: &model.HTTPExpectSpec{
						Status: nethttp.StatusOK,
						Body: `{
							"name": "test",
							"id": 42
						}`,
					},
				},
			},
		},
	}

	result := &execute.TestExecutionResult{
		Summary: &model.TestExecutionSummary{},
	}

	err := executor.Execute(
		context.Background(),
		job,
		result,
	)
	if err != nil {
		t.Fatalf(
			"Execute() error = %v",
			err,
		)
	}

	if result.Summary.Response.HTTP == nil {
		t.Fatal("expected HTTP response summary")
	}

	response := result.Summary.Response.HTTP

	if response.StatusCode != nethttp.StatusOK {
		t.Errorf(
			"expected status %d, got %d",
			nethttp.StatusOK,
			response.StatusCode,
		)
	}

	if response.BodyJSON !=
		`{"id":42,"name":"test"}` {
		t.Errorf(
			"unexpected response body %q",
			response.BodyJSON,
		)
	}

	if len(result.Summary.Deviations) != 0 {
		t.Errorf(
			"expected no deviations, got %+v",
			result.Summary.Deviations,
		)
	}
}
