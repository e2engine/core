package http

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	e2enginehttp "github.com/e2engine/instrumentation-go/http"

	"github.com/e2engine/core/execute/call"
	"github.com/e2engine/core/execute/runtime"
	"github.com/e2engine/core/execute/runtime/config"
	"github.com/e2engine/core/pkg/log"
)

const testExecutionID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestValidTestExecutionID(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   string
	}{
		{name: "missing"},
		{name: "valid", values: []string{testExecutionID}, want: testExecutionID},
		{name: "empty", values: []string{""}},
		{name: "too short", values: []string{"execution-1"}},
		{name: "uppercase", values: []string{strings.ToUpper(testExecutionID)}},
		{name: "multiple", values: []string{testExecutionID, testExecutionID}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := make(http.Header)
			for _, value := range tt.values {
				header.Add("E2Engine-Test-Execution-ID", value)
			}

			if got := validTestExecutionID(header); got != tt.want {
				t.Errorf("validTestExecutionID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRouterNormalizesExecutionIDUpstream(t *testing.T) {
	upstreamHeaders := make(chan http.Header, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		upstreamHeaders <- req.Header.Clone()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	store := call.NewStoreFactory(&call.Config{
		MaxCalls:         10,
		MaxRecordedBytes: 1024 * 1024,
	}).New()
	router := newTestHTTPRouter(t, store)
	router.Register(&runtime.Route{
		ServiceID:      "service-1",
		ListenAddress:  "127.0.0.1:0",
		RuntimeAddress: upstream.Listener.Addr().String(),
	})
	if err := router.Mount(t.Context()); err != nil {
		t.Fatalf("Mount() error = %v", err)
	}
	defer func() {
		if err := router.Unmount(context.Background()); err != nil {
			t.Errorf("Unmount() error = %v", err)
		}
	}()

	tests := []struct {
		name   string
		values []string
		want   []string
	}{
		{name: "valid", values: []string{testExecutionID}, want: []string{testExecutionID}},
		{name: "invalid", values: []string{"execution-1"}},
		{name: "multiple", values: []string{testExecutionID, testExecutionID}},
		{name: "missing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(
				t.Context(),
				http.MethodGet,
				"http://"+router.mounted[0].listener.Addr().String(),
				nil,
			)
			if err != nil {
				t.Fatalf("NewRequestWithContext() error = %v", err)
			}
			req.Header.Set("X-Test", "preserved")
			for _, value := range tt.values {
				req.Header.Add("E2Engine-Test-Execution-ID", value)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Do() error = %v", err)
			}
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusNoContent {
				t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
			}

			got := <-upstreamHeaders
			if values := got.Values("E2Engine-Test-Execution-ID"); !slices.Equal(values, tt.want) {
				t.Errorf("upstream execution IDs = %q, want %q", values, tt.want)
			}
			if got.Get("X-Test") != "preserved" {
				t.Errorf("upstream X-Test = %q, want preserved", got.Get("X-Test"))
			}
		})
	}
}

func TestRouter(t *testing.T) {
	var receivedBody string

	upstream := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, req *http.Request) {
				body, err := io.ReadAll(req.Body)
				if err != nil {
					t.Errorf(
						"ReadAll(upstream request) error = %v",
						err,
					)
					return
				}

				receivedBody = string(body)

				w.Header().Set("X-Upstream", "value")
				w.WriteHeader(http.StatusCreated)

				if _, err := w.Write([]byte("response")); err != nil {
					t.Errorf("Write() error = %v", err)
				}
			},
		),
	)
	defer upstream.Close()

	store := call.NewStoreFactory(
		&call.Config{
			MaxCalls:         1000,
			MaxRecordedBytes: 1024 * 1024,
		}).New()
	router := newTestHTTPRouter(t, store)

	router.Register(&runtime.Route{
		ServiceID:      "service-1",
		ListenAddress:  "127.0.0.1:0",
		RuntimeAddress: upstream.Listener.Addr().String(),
	})

	if err := router.Mount(context.Background()); err != nil {
		t.Fatalf("Mount() error = %v", err)
	}

	defer func() {
		if err := router.Unmount(context.Background()); err != nil {
			t.Errorf("Unmount() error = %v", err)
		}
	}()

	if len(router.mounted) != 1 {
		t.Fatalf(
			"expected one mounted route, got %d",
			len(router.mounted),
		)
	}

	address := router.mounted[0].listener.Addr().String()

	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"http://"+address+"/path?foo=bar",
		strings.NewReader("request"),
	)
	if err != nil {
		t.Fatalf("cannot create request: %v", err)
	}

	req.Header.Set("X-Test", "value")

	req.Header = e2enginehttp.WithTestExecutionID(req.Header, testExecutionID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request error = %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll(response) error = %v", err)
	}

	if resp.StatusCode != http.StatusCreated {
		t.Errorf(
			"expected status %d, got %d",
			http.StatusCreated,
			resp.StatusCode,
		)
	}

	if resp.Header.Get("X-Upstream") != "value" {
		t.Errorf(
			"expected X-Upstream header, got %q",
			resp.Header.Get("X-Upstream"),
		)
	}

	if string(body) != "response" {
		t.Errorf(
			"expected response body response, got %q",
			string(body),
		)
	}

	if receivedBody != "request" {
		t.Errorf(
			"expected upstream request body request, got %q",
			receivedBody,
		)
	}

	calls := store.Get(call.Filter{
		TestExecutionID: testExecutionID,
		ServiceID:       "service-1",
	})

	if len(calls) != 1 {
		t.Fatalf(
			"expected one recorded call, got %d",
			len(calls),
		)
	}

	got := calls[0]

	if got.TestExecutionID != testExecutionID {
		t.Errorf(
			"expected execution ID %s, got %s",
			testExecutionID,
			got.TestExecutionID,
		)
	}

	if got.ServiceID != "service-1" {
		t.Errorf(
			"expected service ID service-1, got %s",
			got.ServiceID,
		)
	}

	if got.HTTP == nil {
		t.Fatal("expected HTTP call")
	}

	if got.HTTP.Request.Method != http.MethodPost {
		t.Errorf(
			"expected method POST, got %s",
			got.HTTP.Request.Method,
		)
	}

	if got.HTTP.Request.Path != "/path" {
		t.Errorf(
			"expected path /path, got %s",
			got.HTTP.Request.Path,
		)
	}

	if got.HTTP.Request.Query["foo"][0] != "bar" {
		t.Errorf(
			"expected query foo=bar, got %#v",
			got.HTTP.Request.Query,
		)
	}

	if got.HTTP.Request.Headers.Get("X-Test") != "value" {
		t.Errorf(
			"expected X-Test header, got %q",
			got.HTTP.Request.Headers.Get("X-Test"),
		)
	}

	if id := e2enginehttp.TestExecutionID(got.HTTP.Request.Headers); id == "" {
		t.Errorf(
			"expected execution ID header to be present, got %q",
			id,
		)
	}

	if string(got.HTTP.Request.Body) != "request" {
		t.Errorf(
			"expected recorded request body request, got %q",
			string(got.HTTP.Request.Body),
		)
	}

	if got.HTTP.Request.BodyTruncated {
		t.Error("expected recorded request body not to be truncated")
	}

	if got.HTTP.Response.StatusCode != http.StatusCreated {
		t.Errorf(
			"expected recorded status %d, got %d",
			http.StatusCreated,
			got.HTTP.Response.StatusCode,
		)
	}

	if got.HTTP.Response.Headers.Get("X-Upstream") != "value" {
		t.Errorf(
			"expected recorded X-Upstream header, got %q",
			got.HTTP.Response.Headers.Get("X-Upstream"),
		)
	}

	if string(got.HTTP.Response.Body) != "response" {
		t.Errorf(
			"expected recorded response body response, got %q",
			string(got.HTTP.Response.Body),
		)
	}

	if got.HTTP.Response.BodyTruncated {
		t.Error("expected recorded response body not to be truncated")
	}
}

func TestRouterProxyError(t *testing.T) {
	store := call.NewStoreFactory(
		&call.Config{
			MaxCalls:         1000,
			MaxRecordedBytes: 1024 * 1024,
		}).New()
	router := newTestHTTPRouter(t, store)

	// Obtain an address that currently has no listener.
	listenerConfig := net.ListenConfig{}
	listener, err := listenerConfig.Listen(
		t.Context(),
		"tcp",
		"127.0.0.1:0",
	)
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}

	target := listener.Addr().String()

	if err := listener.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	router.Register(&runtime.Route{
		ServiceID:      "service-1",
		ListenAddress:  "127.0.0.1:0",
		RuntimeAddress: target,
	})

	if err := router.Mount(context.Background()); err != nil {
		t.Fatalf("Mount() error = %v", err)
	}

	defer func() {
		if err := router.Unmount(context.Background()); err != nil {
			t.Errorf("Unmount() error = %v", err)
		}
	}()

	address := router.mounted[0].listener.Addr().String()

	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"http://"+address+"/",
		nil,
	)
	if err != nil {
		t.Fatalf("cannot create request: %v", err)
	}
	req.Header = e2enginehttp.WithTestExecutionID(req.Header, testExecutionID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf(
			"expected status %d, got %d",
			http.StatusBadGateway,
			resp.StatusCode,
		)
	}

	calls := store.Get(call.Filter{
		TestExecutionID: testExecutionID,
		ServiceID:       "service-1",
	})

	if len(calls) != 1 {
		t.Fatalf(
			"expected one recorded call, got %d",
			len(calls),
		)
	}

	if calls[0].HTTP == nil {
		t.Fatal("expected HTTP call")
	}

	if calls[0].HTTP.Response.StatusCode != http.StatusBadGateway {
		t.Errorf(
			"expected recorded status %d, got %d",
			http.StatusBadGateway,
			calls[0].HTTP.Response.StatusCode,
		)
	}
}

func TestRouterMountCancelledContext(t *testing.T) {
	router := newTestHTTPRouter(t, call.NewStoreFactory(
		&call.Config{
			MaxCalls:         1000,
			MaxRecordedBytes: 1024 * 1024,
		}).New())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := router.Mount(ctx)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"expected context.Canceled, got %v",
			err,
		)
	}
}

func TestRouterUnmountCancelledContext(t *testing.T) {
	store := call.NewStoreFactory(&call.Config{
		MaxCalls:         10,
		MaxRecordedBytes: 1 << 20,
	}).New()

	router := newTestHTTPRouter(t, store)

	router.Register(&runtime.Route{
		ServiceID:      "service-1",
		ListenAddress:  "127.0.0.1:0",
		RuntimeAddress: "127.0.0.1:1",
	})

	if err := router.Mount(t.Context()); err != nil {
		t.Fatalf("Mount() error = %v", err)
	}

	listener := router.mounted[0].listener

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := router.Unmount(ctx)

	if err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"Unmount() unexpected error = %v",
			err,
		)
	}

	if router.mounted != nil {
		t.Fatal("expected mounted routes to be cleared")
	}

	if err := listener.(*net.TCPListener).SetDeadline(
		time.Now().Add(time.Second),
	); err != nil && !errors.Is(err, net.ErrClosed) {
		t.Fatalf("SetDeadline() error = %v", err)
	}

	conn, err := listener.Accept()
	if err == nil {
		_ = conn.Close()
		t.Fatal("listener still accepts connections after unmount")
	}
	if !errors.Is(err, net.ErrClosed) {
		t.Fatalf("Accept() error = %v, want net.ErrClosed", err)
	}
}

func TestRouterMultipleRoutes(t *testing.T) {
	router := newTestHTTPRouter(
		t,
		call.NewStoreFactory(
			&call.Config{
				MaxCalls:         1000,
				MaxRecordedBytes: 1024 * 1024,
			}).New(),
	)

	router.Register(&runtime.Route{
		ServiceID:      "service-1",
		ListenAddress:  "127.0.0.1:0",
		RuntimeAddress: "127.0.0.1:10001",
	})

	router.Register(&runtime.Route{
		ServiceID:      "service-2",
		ListenAddress:  "127.0.0.1:0",
		RuntimeAddress: "127.0.0.1:10002",
	})

	if err := router.Mount(context.Background()); err != nil {
		t.Fatalf("Mount() error = %v", err)
	}

	if len(router.mounted) != 2 {
		t.Fatalf(
			"expected two mounted routes, got %d",
			len(router.mounted),
		)
	}

	if err := router.Unmount(context.Background()); err != nil {
		t.Fatalf("Unmount() error = %v", err)
	}

	if router.mounted != nil {
		t.Fatal("expected mounted routes to be cleared")
	}
}

func TestRouterTruncatedBodyRecording(t *testing.T) {
	const (
		requestBody  = "123456789"
		responseBody = "abcdefghi"
	)

	upstream := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, req *http.Request) {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Errorf("ReadAll(upstream request) error = %v", err)
				return
			}

			if string(body) != requestBody {
				t.Errorf(
					"upstream request body = %q, want %q",
					body,
					requestBody,
				)
			}

			if _, err := io.WriteString(w, responseBody); err != nil {
				t.Errorf("Write(upstream response) error = %v", err)
			}
		},
	))
	defer upstream.Close()

	store := call.NewStoreFactory(
		&call.Config{
			MaxCalls:         1000,
			MaxRecordedBytes: 1024 * 1024,
		}).New()
	router := newTestHTTPRouter(t, store)
	router.cfg.MaxRecordedBodySize = 5

	router.Register(&runtime.Route{
		ServiceID:      "service-1",
		ListenAddress:  "127.0.0.1:0",
		RuntimeAddress: upstream.Listener.Addr().String(),
	})

	if err := router.Mount(t.Context()); err != nil {
		t.Fatalf("Mount() error = %v", err)
	}

	defer func() {
		if err := router.Unmount(context.Background()); err != nil {
			t.Errorf("Unmount() error = %v", err)
		}
	}()

	address := router.mounted[0].listener.Addr().String()

	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"http://"+address+"/test",
		strings.NewReader(requestBody),
	)
	if err != nil {
		t.Fatal(err)
	}

	req.Header = e2enginehttp.WithTestExecutionID(
		req.Header,
		testExecutionID,
	)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	// Verify that the complete response was forwarded.
	if string(body) != responseBody {
		t.Errorf(
			"forwarded response body = %q, want %q",
			body,
			responseBody,
		)
	}

	calls := store.Get(call.Filter{
		TestExecutionID: testExecutionID,
		ServiceID:       "service-1",
	})

	if len(calls) != 1 {
		t.Fatalf(
			"recorded calls = %d, want 1",
			len(calls),
		)
	}

	if calls[0].HTTP == nil {
		t.Fatal("expected recorded HTTP call")
	}

	got := calls[0].HTTP

	// Only the first five request bytes should be recorded.
	if string(got.Request.Body) != requestBody[:5] {
		t.Errorf(
			"recorded request body = %q, want %q",
			got.Request.Body,
			requestBody[:5],
		)
	}

	if !got.Request.BodyTruncated {
		t.Error("expected recorded request body to be truncated")
	}

	// Only the first five response bytes should be recorded.
	if string(got.Response.Body) != responseBody[:5] {
		t.Errorf(
			"recorded response body = %q, want %q",
			got.Response.Body,
			responseBody[:5],
		)
	}

	if !got.Response.BodyTruncated {
		t.Error("expected recorded response body to be truncated")
	}
}

func TestRouterResponseHeaderTimeout(t *testing.T) {
	upstreamEntered := make(chan struct{})

	upstream := httptest.NewServer(
		http.HandlerFunc(func(
			w http.ResponseWriter,
			req *http.Request,
		) {
			close(upstreamEntered)

			// Wait until the reverse proxy cancels the request
			// because of the response-header timeout.
			<-req.Context().Done()
		}),
	)
	defer upstream.Close()

	store := call.NewStoreFactory(
		&call.Config{
			MaxCalls:         10,
			MaxRecordedBytes: 1 << 20,
		},
	).New()

	router := newTestHTTPRouter(t, store)
	router.cfg.ResponseHeaderTimeout = 50 * time.Millisecond

	router.Register(&runtime.Route{
		ServiceID:      "service-1",
		ListenAddress:  "127.0.0.1:0",
		RuntimeAddress: upstream.Listener.Addr().String(),
	})

	if err := router.Mount(t.Context()); err != nil {
		t.Fatalf("Mount() error = %v", err)
	}

	defer func() {
		if err := router.Unmount(context.Background()); err != nil {
			t.Errorf("Unmount() error = %v", err)
		}
	}()

	address := router.mounted[0].listener.Addr().String()

	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"http://"+address+"/timeout",
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf(
			"status = %d, want %d",
			resp.StatusCode,
			http.StatusBadGateway,
		)
	}

	select {
	case <-upstreamEntered:
	default:
		t.Error("upstream handler was not entered")
	}
}

func TestRouterResponseHeaderTimeoutDoesNotLimitBody(t *testing.T) {
	const delay = 150 * time.Millisecond

	upstream := httptest.NewServer(
		http.HandlerFunc(func(
			w http.ResponseWriter,
			_ *http.Request,
		) {
			w.WriteHeader(http.StatusOK)

			// Flush headers immediately so that the timeout
			// applies only to receiving the initial headers.
			w.(http.Flusher).Flush()

			time.Sleep(delay)

			if _, err := io.WriteString(w, "late body"); err != nil {
				t.Errorf("Write() error = %v", err)
			}
		}),
	)
	defer upstream.Close()

	store := call.NewStoreFactory(
		&call.Config{
			MaxCalls:         10,
			MaxRecordedBytes: 1 << 20,
		},
	).New()

	router := newTestHTTPRouter(t, store)
	router.cfg.ResponseHeaderTimeout = 50 * time.Millisecond

	router.Register(&runtime.Route{
		ServiceID:      "service-1",
		ListenAddress:  "127.0.0.1:0",
		RuntimeAddress: upstream.Listener.Addr().String(),
	})

	if err := router.Mount(t.Context()); err != nil {
		t.Fatalf("Mount() error = %v", err)
	}

	defer func() {
		if err := router.Unmount(context.Background()); err != nil {
			t.Errorf("Unmount() error = %v", err)
		}
	}()

	address := router.mounted[0].listener.Addr().String()

	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"http://"+address+"/slow-body",
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf(
			"status = %d, want %d",
			resp.StatusCode,
			http.StatusOK,
		)
	}

	if string(body) != "late body" {
		t.Errorf(
			"body = %q, want %q",
			body,
			"late body",
		)
	}

	calls := store.Get(call.Filter{
		ServiceID: "service-1",
	})

	if len(calls) != 1 {
		t.Fatalf(
			"recorded calls = %d, want 1",
			len(calls),
		)
	}

	if calls[0].HTTP == nil {
		t.Fatal("expected recorded HTTP call")
	}

	recorded := calls[0].HTTP.Response

	if recorded.StatusCode != http.StatusOK {
		t.Errorf(
			"recorded status = %d, want %d",
			recorded.StatusCode,
			http.StatusOK,
		)
	}

	if string(recorded.Body) != "late body" {
		t.Errorf(
			"recorded response body = %q, want %q",
			recorded.Body,
			"late body",
		)
	}

	if recorded.BodyTruncated {
		t.Error("recorded response body must not be truncated")
	}

	if err := store.Err(); err != nil {
		t.Errorf(
			"call store error = %v, want nil",
			err,
		)
	}
}

func TestRouterMaxResponseHeaderBytes(t *testing.T) {
	upstream := httptest.NewServer(
		http.HandlerFunc(func(
			w http.ResponseWriter,
			_ *http.Request,
		) {
			w.Header().Set(
				"X-Large-Header",
				strings.Repeat("a", 4096),
			)
			w.WriteHeader(http.StatusOK)
		}),
	)
	defer upstream.Close()

	store := call.NewStoreFactory(&call.Config{
		MaxCalls:         10,
		MaxRecordedBytes: 1 << 20,
	}).New()

	router := newTestHTTPRouter(t, store)
	router.cfg.MaxResponseHeaderBytes = 1024

	router.Register(&runtime.Route{
		ServiceID:      "service-1",
		ListenAddress:  "127.0.0.1:0",
		RuntimeAddress: upstream.Listener.Addr().String(),
	})

	if err := router.Mount(t.Context()); err != nil {
		t.Fatalf("Mount() error = %v", err)
	}

	defer func() {
		if err := router.Unmount(context.Background()); err != nil {
			t.Errorf("Unmount() error = %v", err)
		}
	}()

	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"http://"+router.mounted[0].listener.Addr().String()+"/large-header",
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	req.Header = e2enginehttp.WithTestExecutionID(
		req.Header,
		testExecutionID,
	)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf(
			"status = %d, want %d",
			resp.StatusCode,
			http.StatusBadGateway,
		)
	}

	calls := store.Get(call.Filter{
		TestExecutionID: testExecutionID,
		ServiceID:       "service-1",
	})

	if len(calls) != 1 {
		t.Fatalf("recorded calls = %d, want 1", len(calls))
	}

	if calls[0].HTTP == nil {
		t.Fatal("expected recorded HTTP call")
	}

	if got := calls[0].HTTP.Response.StatusCode; got != http.StatusBadGateway {
		t.Errorf(
			"recorded status = %d, want %d",
			got,
			http.StatusBadGateway,
		)
	}

	if err := store.Err(); err != nil {
		t.Errorf("store.Err() = %v, want nil", err)
	}
}

func TestRouterUnmountClosesActiveRequest(t *testing.T) {
	upstreamEntered := make(chan struct{})
	upstreamCanceled := make(chan struct{})

	upstream := httptest.NewServer(
		http.HandlerFunc(func(
			w http.ResponseWriter,
			req *http.Request,
		) {
			close(upstreamEntered)

			<-req.Context().Done()

			close(upstreamCanceled)
		}),
	)
	defer upstream.Close()

	store := call.NewStoreFactory(&call.Config{
		MaxCalls:         10,
		MaxRecordedBytes: 1 << 20,
	}).New()

	router := newTestHTTPRouter(t, store)

	router.Register(&runtime.Route{
		ServiceID:      "service-1",
		ListenAddress:  "127.0.0.1:0",
		RuntimeAddress: upstream.Listener.Addr().String(),
	})

	if err := router.Mount(t.Context()); err != nil {
		t.Fatalf("Mount() error = %v", err)
	}

	address := router.mounted[0].listener.Addr().String()

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	requestDone := make(chan error, 1)

	go func() {
		req, err := http.NewRequestWithContext(
			t.Context(),
			http.MethodGet,
			"http://"+address+"/blocked",
			nil,
		)
		if err != nil {
			requestDone <- err
			return
		}

		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			requestDone <- errors.New(
				"request unexpectedly succeeded",
			)
			return
		}

		requestDone <- err
	}()

	select {
	case <-upstreamEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("request did not reach upstream")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := router.Unmount(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf(
			"Unmount() error = %v, want context.Canceled",
			err,
		)
	}

	if router.mounted != nil {
		t.Error("mounted routes not cleared")
	}

	if conn, err := net.DialTimeout(
		"tcp",
		address,
		time.Second,
	); err == nil {
		_ = conn.Close()
		t.Error("listener still accepts connections")
	}

	select {
	case requestDoneErr := <-requestDone:
		_ = requestDoneErr
	case <-time.After(5 * time.Second):
		t.Error("active client request did not terminate")
	}

	select {
	case <-upstreamCanceled:
	case <-time.After(5 * time.Second):
		t.Error("upstream request context was not canceled")
	}
}

func newTestHTTPRouter(
	t *testing.T,
	store *call.Store,
) *Router {
	t.Helper()

	logger, err := log.NewSilentLogger()
	if err != nil {
		t.Fatalf(
			"cannot create silent logger: %v",
			err,
		)
	}

	routerCfg := &config.HTTPRouterConfig{
		MaxRecordedBodySize:    1024,
		ReadHeaderTimeout:      5 * time.Second,
		IdleTimeout:            60 * time.Second,
		ResponseHeaderTimeout:  30 * time.Second,
		MaxResponseHeaderBytes: 1 << 20,
	}

	return NewRouter(routerCfg, logger, store)
}
