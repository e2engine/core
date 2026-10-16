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
	"go.uber.org/mock/gomock"

	"github.com/e2engine/core/execute"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/model/matcher"
)

func TestBuildHTTPRequest(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		spec *model.HTTPRequest

		expectedMethod      string
		expectedURL         string
		expectedBody        string
		expectedContentType string
	}{
		{
			name: "basic request",
			spec: &model.HTTPRequest{
				Method: nethttp.MethodGet,
				URL:    "https://example.com/users",
			},
			expectedMethod: nethttp.MethodGet,
			expectedURL:    "https://example.com/users",
		},
		{
			name: "body does not get default content type",
			spec: &model.HTTPRequest{
				Method: nethttp.MethodPost,
				URL:    "https://example.com/users",
				Body:   `{"id":42}`,
			},
			expectedMethod: nethttp.MethodPost,
			expectedURL:    "https://example.com/users",
			expectedBody:   `{"id":42}`,
		},
		{
			name: "body JSON gets default content type",
			spec: &model.HTTPRequest{
				Method:   nethttp.MethodPost,
				URL:      "https://example.com/users",
				BodyJSON: `{"id":42}`,
			},
			expectedMethod:      nethttp.MethodPost,
			expectedURL:         "https://example.com/users",
			expectedBody:        `{"id":42}`,
			expectedContentType: "application/json",
		},
		{
			name: "body JSON explicit content type is preserved",
			spec: &model.HTTPRequest{
				Method: nethttp.MethodPost,
				URL:    "https://example.com/users",
				Headers: nethttp.Header{
					"Content-Type": {"application/problem+json"},
				},
				BodyJSON: `{"id":42}`,
			},
			expectedMethod:      nethttp.MethodPost,
			expectedURL:         "https://example.com/users",
			expectedBody:        `{"id":42}`,
			expectedContentType: "application/problem+json",
		},
		{
			name: "empty body does not get content type",
			spec: &model.HTTPRequest{
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
				tt.spec.URL,
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

			if id := e2enginehttp.TestExecutionID(
				req.Header,
			); id != "execution-123" {
				t.Errorf(
					"expected execution ID header %q, got %q",
					"execution-123",
					id,
				)
			}

			if req.Context() != ctx {
				t.Error(
					"expected request to use provided context",
				)
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
	spec := &model.HTTPRequest{
		Method: nethttp.MethodGet,
		URL: "https://example.com/users?" +
			"existing=value&override=old",
		Query: map[string][]string{
			"added":    {"new"},
			"override": {"new"},
		},
	}

	req, err := buildHTTPRequest(
		context.Background(),
		spec.URL,
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

	if !reflect.DeepEqual(
		query["added"],
		[]string{"new"},
	) {
		t.Errorf(
			"unexpected added query parameters: %#v",
			query["added"],
		)
	}

	if !reflect.DeepEqual(
		query["override"],
		[]string{"old", "new"},
	) {
		t.Errorf(
			"unexpected override query parameters: %#v",
			query["override"],
		)
	}
}

func TestBuildHTTPRequestHeaders(t *testing.T) {
	spec := &model.HTTPRequest{
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
		spec.URL,
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

	if id := e2enginehttp.TestExecutionID(
		req.Header,
	); id != "execution-123" {
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

	spec := &model.HTTPRequest{
		Method: nethttp.MethodGet,
		URL:    "https://example.com",
	}

	req, err := buildHTTPRequest(
		ctx,
		spec.URL,
		spec,
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

func TestBuildHTTPRequestResolvedURL(t *testing.T) {
	spec := &model.HTTPRequest{
		Method: nethttp.MethodGet,
		URL:    "http://payment-api:8080/payments",
	}

	const resolvedURL = "http://127.0.0.1:8080/payments"

	req, err := buildHTTPRequest(
		t.Context(),
		resolvedURL,
		spec,
		"execution-123",
	)
	if err != nil {
		t.Fatalf("buildHTTPRequest() error = %v", err)
	}

	if actual := req.URL.String(); actual != resolvedURL {
		t.Errorf(
			"expected URL %q, got %q",
			resolvedURL,
			actual,
		)
	}

	if spec.URL != "http://payment-api:8080/payments" {
		t.Errorf(
			"original URL was modified: %q",
			spec.URL,
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
				if id := e2enginehttp.TestExecutionID(
					r.Header,
				); id != "execution-123" {
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

	const logicalURL = "http://payment-api:8080/users"
	resolvedURL := server.URL + "/users"

	ctrl := gomock.NewController(t)

	reporter := NewMockexecutionReporter(ctrl)
	addressResolver := NewMockaddressResolver(ctrl)

	addressResolver.EXPECT().
		ResolveHTTPURL(logicalURL).
		Return(resolvedURL, nil)

	evaluator := NewEvaluator(reporter)

	executor := NewTestExecutor(
		server.Client(),
		evaluator,
		reporter,
		addressResolver,
	)

	job := &execute.TestJob{
		ExecutionID: "execution-123",
		Test: model.Test{
			Spec: model.TestSpec{
				Request: model.RequestSpec{
					HTTP: &model.HTTPRequest{
						Method: nethttp.MethodGet,
						URL:    logicalURL,
					},
				},
				Expect: model.ExpectSpec{
					HTTP: &model.HTTPResponsePattern{
						StatusCode: nethttp.StatusOK,
						BodyJSON: stringPtrExecutor(
							`{"name":"test","id":42}`,
						),
					},
				},
			},
		},
	}

	reporter.EXPECT().
		AddHTTPRequest(gomock.Any()).
		Do(func(req *nethttp.Request) {
			if req.Method != nethttp.MethodGet {
				t.Errorf(
					"expected method %q, got %q",
					nethttp.MethodGet,
					req.Method,
				)
			}

			if req.URL.String() != resolvedURL {
				t.Errorf(
					"expected resolved URL %q, got %q",
					resolvedURL,
					req.URL.String(),
				)
			}

			if id := e2enginehttp.TestExecutionID(
				req.Header,
			); id != "execution-123" {
				t.Errorf(
					"expected execution ID header %q, got %q",
					"execution-123",
					id,
				)
			}
		})

	reporter.EXPECT().
		AddHTTPResponseStatus(nethttp.StatusOK)

	reporter.EXPECT().
		AddHTTPResponseHeaders(gomock.Any()).
		Do(func(headers nethttp.Header) {
			if headers.Get("Content-Type") !=
				"application/json" {
				t.Errorf(
					"expected Content-Type %q, got %q",
					"application/json",
					headers.Get("Content-Type"),
				)
			}
		})

	reporter.EXPECT().
		AddHTTPResponseBodyJSON(
			[]byte(`{"id":42,"name":"test"}`),
		)

	err := executor.Execute(
		context.Background(),
		job,
	)
	if err != nil {
		t.Fatalf(
			"Execute() error = %v",
			err,
		)
	}

	// Resolution must not modify the original Test specification.
	if job.Test.Spec.Request.HTTP.URL != logicalURL {
		t.Errorf(
			"expected original URL %q, got %q",
			logicalURL,
			job.Test.Spec.Request.HTTP.URL,
		)
	}
}

func TestExecutorExecuteAddressResolutionError(t *testing.T) {
	ctrl := gomock.NewController(t)

	reporter := NewMockexecutionReporter(ctrl)
	addressResolver := NewMockaddressResolver(ctrl)

	evaluator := NewEvaluator(reporter)

	executor := NewTestExecutor(
		nethttp.DefaultClient,
		evaluator,
		reporter,
		addressResolver,
	)

	const logicalURL = "http://payment-api:8080/payments"

	resolveErr := nativeerrors.New("cannot resolve HTTP URL")

	addressResolver.EXPECT().
		ResolveHTTPURL(logicalURL).
		Return("", resolveErr)

	job := &execute.TestJob{
		ExecutionID: "execution-123",
		Test: model.Test{
			Spec: model.TestSpec{
				Request: model.RequestSpec{
					HTTP: &model.HTTPRequest{
						Method: nethttp.MethodGet,
						URL:    logicalURL,
					},
				},
			},
		},
	}

	err := executor.Execute(
		context.Background(),
		job,
	)

	if !nativeerrors.Is(err, resolveErr) {
		t.Errorf(
			"expected error %v, got %v",
			resolveErr,
			err,
		)
	}

	// No reporter calls should occur when address resolution fails.
	// gomock verifies this because no expectations were registered.
}

func stringPtrExecutor(
	value string,
) *matcher.String {
	return &matcher.String{
		Operator: matcher.StringMatchOperatorEquals,
		Value:    value,
	}
}
