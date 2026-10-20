package http_test

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	httpservice "github.com/e2engine/core/execute/runtime/service/http"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/model/matcher"
	"github.com/e2engine/core/pkg/keys"
	"github.com/e2engine/core/pkg/log"
)

func TestMockHTTPServiceMatch(t *testing.T) {
	tests := []struct {
		name           string
		fixture        model.FixtureSpec
		method         string
		path           string
		headers        map[string][]string
		body           string
		expectedStatus int
		expectedMiss   bool
	}{
		{
			name: "JSON object body matches semantically",
			fixture: newHTTPFixture(
				"POST",
				"/users",
				nil,
				nil,
				ptr(`{"name":"Alice","age":42}`),
				http.StatusCreated,
			),
			method: "POST",
			path:   "/users",
			body:   `{"age":42,"name":"Alice"}`,

			expectedStatus: http.StatusCreated,
		},
		{
			name: "JSON array body",
			fixture: newHTTPFixture(
				"POST",
				"/values",
				nil,
				nil,
				ptr(`[1,2,3]`),
				http.StatusOK,
			),
			method: "POST",
			path:   "/values",
			body:   `[1,2,3]`,

			expectedStatus: http.StatusOK,
		},
		{
			name: "JSON scalar body",
			fixture: newHTTPFixture(
				"POST",
				"/value",
				nil,
				nil,
				ptr(`42`),
				http.StatusOK,
			),
			method: "POST",
			path:   "/value",
			body:   `42.0`,

			expectedStatus: http.StatusOK,
		},
		{
			name: "JSON string body",
			fixture: newHTTPFixture(
				"POST",
				"/value",
				nil,
				nil,
				ptr(`"hello"`),
				http.StatusOK,
			),
			method: "POST",
			path:   "/value",
			body:   `"hello"`,

			expectedStatus: http.StatusOK,
		},
		{
			name: "JSON boolean body",
			fixture: newHTTPFixture(
				"POST",
				"/value",
				nil,
				nil,
				ptr(`true`),
				http.StatusOK,
			),
			method: "POST",
			path:   "/value",
			body:   `true`,

			expectedStatus: http.StatusOK,
		},
		{
			name: "JSON null body",
			fixture: newHTTPFixture(
				"POST",
				"/value",
				nil,
				nil,
				ptr(`null`),
				http.StatusOK,
			),
			method: "POST",
			path:   "/value",
			body:   `null`,

			expectedStatus: http.StatusOK,
		},
		{
			name: "exact text body",
			fixture: newHTTPFixture(
				"POST",
				"/text",
				nil,
				ptr("hello world"),
				nil,
				http.StatusOK,
			),
			method: "POST",
			path:   "/text",
			body:   "hello world",

			expectedStatus: http.StatusOK,
		},
		{
			name: "arbitrary non-JSON text body",
			fixture: newHTTPFixture(
				"POST",
				"/text",
				nil,
				ptr("this is not JSON: {"),
				nil,
				http.StatusOK,
			),
			method: "POST",
			path:   "/text",
			body:   "this is not JSON: {",

			expectedStatus: http.StatusOK,
		},
		{
			name: "text body uses exact semantics",
			fixture: newHTTPFixture(
				"POST",
				"/users",
				nil,
				ptr(`{"name":"Alice","age":42}`),
				nil,
				http.StatusOK,
			),
			method: "POST",
			path:   "/users",
			body:   `{"age":42,"name":"Alice"}`,

			expectedStatus: http.StatusNotImplemented,
			expectedMiss:   true,
		},
		{
			name: "empty body",
			fixture: newHTTPFixture(
				"GET",
				"/users",
				nil,
				ptr(""),
				nil,
				http.StatusOK,
			),
			method: "GET",
			path:   "/users",

			expectedStatus: http.StatusOK,
		},
		{
			name: "query parameter order ignored",
			fixture: newHTTPFixture(
				"GET",
				"/users?status=active&page=2",
				nil,
				nil,
				nil,
				http.StatusOK,
			),
			method: "GET",
			path:   "/users?page=2&status=active",

			expectedStatus: http.StatusOK,
		},
		{
			name: "repeated query parameters",
			fixture: newHTTPFixture(
				"GET",
				"/users?role=admin&role=operator",
				nil,
				nil,
				nil,
				http.StatusOK,
			),
			method: "GET",
			path:   "/users?role=admin&role=operator",

			expectedStatus: http.StatusOK,
		},
		{
			name: "specified headers match",
			fixture: newHTTPFixture(
				"GET",
				"/users",
				map[string][]matcher.String{
					"X-Test": {
						{
							Operator: matcher.StringMatchOperatorEquals,
							Value:    "one",
						}, {
							Operator: matcher.StringMatchOperatorEquals,
							Value:    "two",
						},
					},
				},
				nil,
				nil,
				http.StatusOK,
			),
			method: "GET",
			path:   "/users",
			headers: map[string][]string{
				"X-Test": {"two", "one"},
			},

			expectedStatus: http.StatusOK,
		},
		{
			name: "additional request headers ignored",
			fixture: newHTTPFixture(
				"GET",
				"/users",
				map[string][]matcher.String{
					"X-Test": {
						{
							Operator: matcher.StringMatchOperatorEquals,
							Value:    "expected",
						},
					},
				},
				nil,
				nil,
				http.StatusOK,
			),
			method: "GET",
			path:   "/users",
			headers: map[string][]string{
				"X-Test":  {"expected"},
				"X-Other": {"ignored"},
			},

			expectedStatus: http.StatusOK,
		},
		{
			name: "header names are case insensitive",
			fixture: newHTTPFixture(
				"GET",
				"/users",
				map[string][]matcher.String{
					"X-Test": {
						{
							Operator: matcher.StringMatchOperatorEquals,
							Value:    "expected",
						},
					},
				},
				nil,
				nil,
				http.StatusOK,
			),
			method: "GET",
			path:   "/users",
			headers: map[string][]string{
				"x-test": {"expected"},
			},

			expectedStatus: http.StatusOK,
		},
		{
			name: "method miss",
			fixture: newHTTPFixture(
				"GET",
				"/users",
				nil,
				nil,
				nil,
				http.StatusOK,
			),
			method: "POST",
			path:   "/users",

			expectedStatus: http.StatusNotImplemented,
			expectedMiss:   true,
		},
		{
			name: "path miss",
			fixture: newHTTPFixture(
				"GET",
				"/users",
				nil,
				nil,
				nil,
				http.StatusOK,
			),
			method: "GET",
			path:   "/other",

			expectedStatus: http.StatusNotImplemented,
			expectedMiss:   true,
		},
		{
			name: "query miss",
			fixture: newHTTPFixture(
				"GET",
				"/users?page=1",
				nil,
				nil,
				nil,
				http.StatusOK,
			),
			method: "GET",
			path:   "/users?page=2",

			expectedStatus: http.StatusNotImplemented,
			expectedMiss:   true,
		},
		{
			name: "additional query parameters ignored",
			fixture: newHTTPFixture(
				"GET",
				"/users",
				nil,
				nil,
				nil,
				http.StatusOK,
			),
			method: "GET",
			path:   "/users?page=1",

			expectedStatus: http.StatusOK,
		},
		{
			name: "missing header is miss",
			fixture: newHTTPFixture(
				"GET",
				"/users",
				map[string][]matcher.String{
					"X-Test": {
						{
							Operator: matcher.StringMatchOperatorEquals,
							Value:    "expected",
						},
					},
				},
				nil,
				nil,
				http.StatusOK,
			),
			method: "GET",
			path:   "/users",

			expectedStatus: http.StatusNotImplemented,
			expectedMiss:   true,
		},
		{
			name: "different header value is miss",
			fixture: newHTTPFixture(
				"GET",
				"/users",
				map[string][]matcher.String{
					"X-Test": {
						{
							Operator: matcher.StringMatchOperatorEquals,
							Value:    "expected",
						},
					},
				},
				nil,
				nil,
				http.StatusOK,
			),
			method: "GET",
			path:   "/users",
			headers: map[string][]string{
				"X-Test": {"actual"},
			},

			expectedStatus: http.StatusNotImplemented,
			expectedMiss:   true,
		},
		{
			name: "additional value for specified header is miss",
			fixture: newHTTPFixture(
				"GET",
				"/users",
				map[string][]matcher.String{
					"X-Test": {
						{
							Operator: matcher.StringMatchOperatorEquals,
							Value:    "expected",
						},
					},
				},
				nil,
				nil,
				http.StatusOK,
			),
			method: "GET",
			path:   "/users",
			headers: map[string][]string{
				"X-Test": {"expected", "additional"},
			},

			expectedStatus: http.StatusNotImplemented,
			expectedMiss:   true,
		},
		{
			name: "JSON body miss",
			fixture: newHTTPFixture(
				"POST",
				"/users",
				nil,
				nil,
				ptr(`{"name":"Alice"}`),
				http.StatusOK,
			),
			method: "POST",
			path:   "/users",
			body:   `{"name":"Bob"}`,

			expectedStatus: http.StatusNotImplemented,
			expectedMiss:   true,
		},
		{
			name: "text body miss",
			fixture: newHTTPFixture(
				"POST",
				"/text",
				nil,
				ptr("expected"),
				nil,
				http.StatusOK,
			),
			method: "POST",
			path:   "/text",
			body:   "actual",

			expectedStatus: http.StatusNotImplemented,
			expectedMiss:   true,
		},
		{
			name: "expected empty body actual non-empty body is miss",
			fixture: newHTTPFixture(
				"POST",
				"/users",
				nil,
				ptr(""),
				nil,
				http.StatusOK,
			),
			method: "POST",
			path:   "/users",
			body:   `{}`,

			expectedStatus: http.StatusNotImplemented,
			expectedMiss:   true,
		},
		{
			name: "expected JSON body actual empty body is miss",
			fixture: newHTTPFixture(
				"POST",
				"/users",
				nil,
				nil,
				ptr(`{}`),
				http.StatusOK,
			),
			method: "POST",
			path:   "/users",

			expectedStatus: http.StatusNotImplemented,
			expectedMiss:   true,
		},
		{
			name: "expected text body actual empty body is miss",
			fixture: newHTTPFixture(
				"POST",
				"/text",
				nil,
				ptr("expected"),
				nil,
				http.StatusOK,
			),
			method: "POST",
			path:   "/text",

			expectedStatus: http.StatusNotImplemented,
			expectedMiss:   true,
		},
		{
			name: "header exists with empty value",
			fixture: newHTTPFixture(
				"GET",
				"/users",
				map[string][]matcher.String{
					"X-Test": {
						{
							Operator: matcher.StringMatchOperatorExists,
						},
					},
				},
				nil,
				nil,
				http.StatusOK,
			),
			method: "GET",
			path:   "/users",
			headers: map[string][]string{
				"X-Test": {""},
			},

			expectedStatus: http.StatusOK,
		},
		{
			name: "header exists but is missing",
			fixture: newHTTPFixture(
				"GET",
				"/users",
				map[string][]matcher.String{
					"X-Test": {
						{
							Operator: matcher.StringMatchOperatorExists,
						},
					},
				},
				nil,
				nil,
				http.StatusOK,
			),
			method: "GET",
			path:   "/users",

			expectedStatus: http.StatusNotImplemented,
			expectedMiss:   true,
		},
		{
			name: "header nonempty matches",
			fixture: newHTTPFixture(
				"GET",
				"/users",
				map[string][]matcher.String{
					"X-Test": {
						{
							Operator: matcher.StringMatchOperatorNonEmpty,
						},
					},
				},
				nil,
				nil,
				http.StatusOK,
			),
			method: "GET",
			path:   "/users",
			headers: map[string][]string{
				"X-Test": {"value"},
			},

			expectedStatus: http.StatusOK,
		},
		{
			name: "header nonempty rejects empty value",
			fixture: newHTTPFixture(
				"GET",
				"/users",
				map[string][]matcher.String{
					"X-Test": {
						{
							Operator: matcher.StringMatchOperatorNonEmpty,
						},
					},
				},
				nil,
				nil,
				http.StatusOK,
			),
			method: "GET",
			path:   "/users",
			headers: map[string][]string{
				"X-Test": {""},
			},

			expectedStatus: http.StatusNotImplemented,
			expectedMiss:   true,
		},
		{
			name: "body nonempty matches",
			fixture: newHTTPFixture(
				"POST",
				"/users",
				nil,
				&matcher.String{
					Operator: matcher.StringMatchOperatorNonEmpty,
				},
				nil,
				http.StatusOK,
			),
			method: "POST",
			path:   "/users",
			body:   "hello",

			expectedStatus: http.StatusOK,
		},
		{
			name: "body nonempty rejects empty body",
			fixture: newHTTPFixture(
				"POST",
				"/users",
				nil,
				&matcher.String{
					Operator: matcher.StringMatchOperatorNonEmpty,
				},
				nil,
				http.StatusOK,
			),
			method: "POST",
			path:   "/users",

			expectedStatus: http.StatusNotImplemented,
			expectedMiss:   true,
		},
		{
			name: "body exists matches empty body",
			fixture: newHTTPFixture(
				"POST",
				"/users",
				nil,
				&matcher.String{
					Operator: matcher.StringMatchOperatorExists,
				},
				nil,
				http.StatusOK,
			),
			method: "POST",
			path:   "/users",

			expectedStatus: http.StatusOK,
		},
		{
			name: "JSON nonempty matches empty object",
			fixture: newHTTPFixture(
				"POST",
				"/users",
				nil,
				nil,
				&matcher.String{
					Operator: matcher.StringMatchOperatorNonEmpty,
				},
				http.StatusOK,
			),
			method: "POST",
			path:   "/users",
			body:   `{}`,

			expectedStatus: http.StatusOK,
		},
		{
			name: "JSON nonempty rejects empty body",
			fixture: newHTTPFixture(
				"POST",
				"/users",
				nil,
				nil,
				&matcher.String{
					Operator: matcher.StringMatchOperatorNonEmpty,
				},
				http.StatusOK,
			),
			method: "POST",
			path:   "/users",

			expectedStatus: http.StatusNotImplemented,
			expectedMiss:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := newTestMockHTTPService(
				t,
				[]model.FixtureSpec{tt.fixture},
			)

			if err := service.Mount(
				context.Background(),
			); err != nil {
				t.Fatalf(
					"Mount() error = %v",
					err,
				)
			}

			t.Cleanup(func() {
				if err := service.Unmount(
					context.Background(),
				); err != nil {
					t.Errorf(
						"Unmount() error = %v",
						err,
					)
				}
			})

			req, err := http.NewRequestWithContext(
				t.Context(),
				tt.method,
				"http://"+
					service.GetRuntimeAddress()+
					tt.path,
				strings.NewReader(tt.body),
			)
			if err != nil {
				t.Fatalf(
					"cannot create request: %v",
					err,
				)
			}

			for key, values := range tt.headers {
				for _, value := range values {
					req.Header.Add(
						key,
						value,
					)
				}
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf(
					"request error = %v",
					err,
				)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf(
					"expected status %d, got %d",
					tt.expectedStatus,
					resp.StatusCode,
				)
			}

			gotMiss := resp.Header.Get(
				string(
					keys.E2EngineMockMissHeader,
				),
			) == "true"

			if gotMiss != tt.expectedMiss {
				t.Errorf(
					"expected mock miss %v, got %v",
					tt.expectedMiss,
					gotMiss,
				)
			}
		})
	}
}

func TestMockHTTPServiceResponse(t *testing.T) {
	tests := []struct {
		name         string
		response     *model.HTTPResponse
		expectedBody string
	}{
		{
			name: "text body",
			response: &model.HTTPResponse{
				StatusCode: http.StatusCreated,
				Headers: map[string][]string{
					"X-Test": {"one", "two"},
				},
				Body: "plain text response",
			},
			expectedBody: "plain text response",
		},
		{
			name: "JSON body preserves authored representation",
			response: &model.HTTPResponse{
				StatusCode: http.StatusCreated,
				Headers: map[string][]string{
					"X-Test": {"one", "two"},
				},
				BodyJSON: "{\n  \"id\": 42\n}",
			},
			expectedBody: "{\n  \"id\": 42\n}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := newTestMockHTTPService(
				t,
				[]model.FixtureSpec{
					{
						When: model.FixtureWhen{
							HTTP: &model.HTTPRequestPattern{
								Method: "GET",
								Path:   "/users",
							},
						},
						Then: model.FixtureThen{
							HTTP: tt.response,
						},
					},
				},
			)

			if err := service.Mount(
				context.Background(),
			); err != nil {
				t.Fatalf(
					"Mount() error = %v",
					err,
				)
			}

			t.Cleanup(func() {
				if err := service.Unmount(
					context.Background(),
				); err != nil {
					t.Errorf(
						"Unmount() error = %v",
						err,
					)
				}
			})

			req, err := http.NewRequestWithContext(
				t.Context(),
				http.MethodGet,
				"http://"+
					service.GetRuntimeAddress()+
					"/users",
				nil,
			)
			if err != nil {
				t.Fatalf(
					"cannot create request: %v",
					err,
				)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf(
					"request error = %v",
					err,
				)
			}
			defer resp.Body.Close()

			if resp.StatusCode !=
				http.StatusCreated {
				t.Errorf(
					"expected status %d, got %d",
					http.StatusCreated,
					resp.StatusCode,
				)
			}

			values := resp.Header.Values(
				"X-Test",
			)
			if len(values) != 2 ||
				values[0] != "one" ||
				values[1] != "two" {
				t.Errorf(
					"expected X-Test headers [one two], got %v",
					values,
				)
			}

			body, err := io.ReadAll(
				resp.Body,
			)
			if err != nil {
				t.Fatalf(
					"cannot read response body: %v",
					err,
				)
			}

			if string(body) != tt.expectedBody {
				t.Errorf(
					"expected response body %q, got %q",
					tt.expectedBody,
					string(body),
				)
			}
		})
	}
}

func TestMockHTTPServiceRequestBodySizeLimit(t *testing.T) {
	const limit int64 = 5

	tests := []struct {
		name           string
		body           string
		fixtureBody    *matcher.String
		expectedStatus int
	}{
		{
			name:           "below limit",
			body:           "abcd",
			fixtureBody:    ptr("abcd"),
			expectedStatus: http.StatusOK,
		},
		{
			name:           "exactly at limit",
			body:           "abcde",
			fixtureBody:    ptr("abcde"),
			expectedStatus: http.StatusOK,
		},
		{
			name:           "exceeds limit by one byte",
			body:           "abcdef",
			fixtureBody:    ptr("abcdef"),
			expectedStatus: http.StatusRequestEntityTooLarge,
		},
		{
			name:           "exceeds limit without body expectation",
			body:           "abcdef",
			expectedStatus: http.StatusRequestEntityTooLarge,
		},
		{
			name:           "empty body",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newHTTPFixture(
				http.MethodPost,
				"/test",
				nil,
				tt.fixtureBody,
				nil,
				http.StatusOK,
			)

			service := newTestMockHTTPServiceWithConfig(
				t,
				&httpservice.MockHTTPServiceConfig{
					MaxRequestBodySize:    limit,
					ReadHeaderTimeout:     5 * time.Second,
					IdleTimeout:           60 * time.Second,
					MaxRequestHeaderBytes: 1 << 20,
				},
				[]model.FixtureSpec{fixture},
			)

			if err := service.Mount(t.Context()); err != nil {
				t.Fatalf("Mount() error = %v", err)
			}

			t.Cleanup(func() {
				if err := service.Unmount(context.Background()); err != nil {
					t.Errorf("Unmount() error = %v", err)
				}
			})

			req, err := http.NewRequestWithContext(
				t.Context(),
				http.MethodPost,
				"http://"+service.GetRuntimeAddress()+"/test",
				strings.NewReader(tt.body),
			)
			if err != nil {
				t.Fatalf("cannot create request: %v", err)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("request error = %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf(
					"expected status %d, got %d",
					tt.expectedStatus,
					resp.StatusCode,
				)
			}

			// An oversized request must not be classified
			// as an ordinary fixture mismatch.
			if got := resp.Header.Get(
				string(keys.E2EngineMockMissHeader),
			); got == "true" {
				t.Error("unexpected fixture mismatch header")
			}
		})
	}
}

func TestMockHTTPServiceRequestHeaderSizeLimit(t *testing.T) {
	service := newTestMockHTTPServiceWithConfig(
		t,
		&httpservice.MockHTTPServiceConfig{
			MaxRequestBodySize:    1 << 20,
			ReadHeaderTimeout:     5 * time.Second,
			IdleTimeout:           60 * time.Second,
			MaxRequestHeaderBytes: 1024,
		},
		[]model.FixtureSpec{
			newHTTPFixture(
				http.MethodGet,
				"/test",
				nil,
				nil,
				nil,
				http.StatusOK,
			),
		},
	)

	if err := service.Mount(t.Context()); err != nil {
		t.Fatalf("Mount() error = %v", err)
	}

	t.Cleanup(func() {
		if err := service.Unmount(context.Background()); err != nil {
			t.Errorf("Unmount() error = %v", err)
		}
	})

	tests := []struct {
		name       string
		header     string
		wantStatus int
	}{
		{
			name:       "within limit",
			header:     "small",
			wantStatus: http.StatusOK,
		},
		{
			name:       "exceeds limit",
			header:     strings.Repeat("x", 16<<10),
			wantStatus: http.StatusRequestHeaderFieldsTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(
				t.Context(),
				http.MethodGet,
				"http://"+service.GetRuntimeAddress()+"/test",
				nil,
			)
			if err != nil {
				t.Fatal(err)
			}

			req.Header.Set("X-Test", tt.header)

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Do() error = %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf(
					"status = %d, want %d",
					resp.StatusCode,
					tt.wantStatus,
				)
			}
		})
	}
}

func TestMockHTTPServiceUnmountClosesActiveRequest(t *testing.T) {
	service := newTestMockHTTPService(
		t,
		[]model.FixtureSpec{
			newHTTPFixture(
				http.MethodPost,
				"/test",
				nil,
				nil,
				nil,
				http.StatusOK,
			),
		},
	)

	if err := service.Mount(t.Context()); err != nil {
		t.Fatalf("Mount() error = %v", err)
	}

	defer func() {
		_ = service.Unmount(context.Background())
	}()

	conn, err := net.DialTimeout(
		"tcp",
		service.GetRuntimeAddress(),
		5*time.Second,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	if err := conn.SetDeadline(
		time.Now().Add(5 * time.Second),
	); err != nil {
		t.Fatal(err)
	}

	_, err = fmt.Fprintf(
		conn,
		"POST /test HTTP/1.1\r\n"+
			"Host: %s\r\n"+
			"Content-Length: 100\r\n"+
			"Expect: 100-continue\r\n"+
			"\r\n",
		service.GetRuntimeAddress(),
	)
	if err != nil {
		t.Fatal(err)
	}

	reader := bufio.NewReader(conn)

	// The server sends 100 Continue when the mock handler
	// begins reading the request body.
	interim, err := http.ReadResponse(
		reader,
		&http.Request{
			Method: http.MethodPost,
		},
	)
	if err != nil {
		t.Fatalf("ReadResponse() error = %v", err)
	}
	defer interim.Body.Close()

	if interim.StatusCode != http.StatusContinue {
		t.Fatalf(
			"status = %d, want %d",
			interim.StatusCode,
			http.StatusContinue,
		)
	}

	// The request body remains incomplete.
	// Graceful shutdown cannot finish while the handler
	// is waiting for the remaining bytes.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = service.Unmount(ctx)

	if err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"Unmount() unexpected error = %v",
			err,
		)
	}

	// Forced shutdown must terminate the active connection.
	_, err = reader.ReadByte()

	if err == nil {
		t.Error(
			"active connection remained readable after shutdown",
		)
	} else if ne, ok := err.(net.Error); ok && ne.Timeout() {
		t.Errorf(
			"active connection was not closed: %v",
			err,
		)
	}

	// The listener must also be closed.
	newConn, err := net.DialTimeout(
		"tcp",
		service.GetRuntimeAddress(),
		time.Second,
	)
	if err == nil {
		_ = newConn.Close()
		t.Error(
			"listener still accepts connections after unmount",
		)
	}
}

type failingRequestBody struct{}

func (f *failingRequestBody) Read(_ []byte) (int, error) {
	return 0, errors.New("simulated read failure")
}

func (f *failingRequestBody) Close() error {
	return nil
}

func TestMockHTTPServiceRequestBodyReadError(t *testing.T) {
	service := newTestMockHTTPService(
		t,
		[]model.FixtureSpec{
			newHTTPFixture(
				http.MethodPost,
				"/test",
				nil,
				nil,
				nil,
				http.StatusOK,
			),
		},
	)

	req := httptest.NewRequest(
		http.MethodPost,
		"http://example.com/test",
		nil,
	)
	req.Body = &failingRequestBody{}

	recorder := httptest.NewRecorder()

	service.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf(
			"expected status %d, got %d",
			http.StatusBadRequest,
			recorder.Code,
		)
	}

	if recorder.Header().Get(
		string(keys.E2EngineMockMissHeader),
	) == "true" {
		t.Error("unexpected fixture mismatch header")
	}
}

func newTestMockHTTPService(
	t *testing.T,
	fixtures []model.FixtureSpec,
) *httpservice.MockHTTPService {
	t.Helper()

	return newTestMockHTTPServiceWithConfig(
		t,
		&httpservice.MockHTTPServiceConfig{
			MaxRequestBodySize:    1048576,
			ReadHeaderTimeout:     5 * time.Second,
			IdleTimeout:           60 * time.Second,
			MaxRequestHeaderBytes: 1 << 20,
		},
		fixtures,
	)
}

func newTestMockHTTPServiceWithConfig(
	t *testing.T,
	cfg *httpservice.MockHTTPServiceConfig,
	fixtures []model.FixtureSpec,
) *httpservice.MockHTTPService {
	t.Helper()

	logger, err := log.NewSilentLogger()
	if err != nil {
		t.Fatalf("cannot create silent logger: %v", err)
	}

	return httpservice.NewMockHTTPService(
		cfg,
		logger,
		&model.ServiceSpec{
			ID:           "service-1",
			Kind:         model.ServiceKindHTTP,
			Mode:         model.ServiceModeMocked,
			E2EnginePort: 8080,
			Fixtures:     fixtures,
		},
	)
}

func newHTTPFixture(
	method,
	path string,
	headers map[string][]matcher.String,
	body,
	bodyJSON *matcher.String,
	status int,
) model.FixtureSpec {
	return model.FixtureSpec{
		When: model.FixtureWhen{
			HTTP: &model.HTTPRequestPattern{
				Method:   method,
				Path:     path,
				Headers:  headers,
				Body:     body,
				BodyJSON: bodyJSON,
			},
		},
		Then: model.FixtureThen{
			HTTP: &model.HTTPResponse{
				StatusCode: status,
			},
		},
	}
}

func ptr(s string) *matcher.String {
	return &matcher.String{
		Operator: matcher.StringMatchOperatorEquals,
		Value:    s,
	}
}
