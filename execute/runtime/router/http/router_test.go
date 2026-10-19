package http

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	e2enginehttp "github.com/e2engine/instrumentation-go/http"

	"github.com/e2engine/core/execute/call"
	"github.com/e2engine/core/execute/runtime"
	"github.com/e2engine/core/execute/runtime/config"
	"github.com/e2engine/core/pkg/log"
)

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

	req.Header = e2enginehttp.WithTestExecutionID(req.Header, "execution-1")

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
		TestExecutionID: "execution-1",
		ServiceID:       "service-1",
	})

	if len(calls) != 1 {
		t.Fatalf(
			"expected one recorded call, got %d",
			len(calls),
		)
	}

	got := calls[0]

	if got.TestExecutionID != "execution-1" {
		t.Errorf(
			"expected execution ID execution-1, got %s",
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
	req.Header = e2enginehttp.WithTestExecutionID(req.Header, "execution-1")

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
		TestExecutionID: "execution-1",
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
	router := newTestHTTPRouter(t, call.NewStoreFactory(
		&call.Config{
			MaxCalls:         1000,
			MaxRecordedBytes: 1024 * 1024,
		}).New())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := router.Unmount(ctx)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"expected context.Canceled, got %v",
			err,
		)
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
		"execution-1",
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
		TestExecutionID: "execution-1",
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
		// Use a small buffer size for testing.
		MaxRecordedBodySize: 1024,
	}

	return NewRouter(routerCfg, logger, store)
}
