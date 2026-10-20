package http

import (
	"context"
	nativeerrors "errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/log"
)

func TestRealHTTPServiceProxy(t *testing.T) {
	tests := []struct {
		name       string
		targetPath string
		request    func(t *testing.T, address string) *http.Request
		handler    func(t *testing.T, w http.ResponseWriter, r *http.Request)
	}{
		{
			name: "request is proxied",
			request: func(t *testing.T, address string) *http.Request {
				t.Helper()

				req, err := http.NewRequestWithContext(
					t.Context(),
					http.MethodPost,
					"http://"+address+"/users?active=true",
					strings.NewReader(`{"name":"Alice"}`),
				)
				if err != nil {
					t.Fatalf("cannot create request: %v", err)
				}

				req.Header.Set("X-Test", "request")

				return req
			},
			handler: func(
				t *testing.T,
				w http.ResponseWriter,
				r *http.Request,
			) {
				t.Helper()

				if r.Method != http.MethodPost {
					t.Errorf(
						"expected method %q, got %q",
						http.MethodPost,
						r.Method,
					)
				}

				if r.URL.Path != "/users" {
					t.Errorf(
						"expected path %q, got %q",
						"/users",
						r.URL.Path,
					)
				}

				if r.URL.Query().Get("active") != "true" {
					t.Errorf(
						"expected active query parameter %q, got %q",
						"true",
						r.URL.Query().Get("active"),
					)
				}

				if r.Header.Get("X-Test") != "request" {
					t.Errorf(
						"expected X-Test header %q, got %q",
						"request",
						r.Header.Get("X-Test"),
					)
				}

				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf(
						"cannot read request body: %v",
						err,
					)
				}

				if string(body) != `{"name":"Alice"}` {
					t.Errorf(
						"expected request body %q, got %q",
						`{"name":"Alice"}`,
						string(body),
					)
				}

				w.Header().Set("X-Response", "response")
				w.WriteHeader(http.StatusCreated)

				if _, err := w.Write([]byte(`{"id":42}`)); err != nil {
					t.Errorf(
						"cannot write response: %v",
						err,
					)
				}
			},
		},
		{
			name:       "target base path is preserved",
			targetPath: "/api/v1",
			request: func(t *testing.T, address string) *http.Request {
				t.Helper()

				req, err := http.NewRequestWithContext(
					t.Context(),
					http.MethodGet,
					"http://"+address+"/users",
					nil,
				)
				if err != nil {
					t.Fatalf("cannot create request: %v", err)
				}

				return req
			},
			handler: func(
				t *testing.T,
				w http.ResponseWriter,
				r *http.Request,
			) {
				t.Helper()

				if r.URL.Path != "/api/v1/users" {
					t.Errorf(
						"expected path %q, got %q",
						"/api/v1/users",
						r.URL.Path,
					)
				}

				w.WriteHeader(http.StatusOK)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := httptest.NewServer(
				http.HandlerFunc(func(
					w http.ResponseWriter,
					r *http.Request,
				) {
					tt.handler(t, w, r)
				}),
			)
			defer upstream.Close()

			service := newTestRealHTTPService(
				t,
				upstream.URL+tt.targetPath,
			)

			if err := service.Mount(context.Background()); err != nil {
				t.Fatalf("Mount() error = %v", err)
			}
			t.Cleanup(func() {
				if err := service.Unmount(context.Background()); err != nil {
					t.Errorf("Unmount() error = %v", err)
				}
			})

			req := tt.request(
				t,
				service.GetRuntimeAddress(),
			)

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("request error = %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusCreated {
				if resp.Header.Get("X-Response") != "response" {
					t.Errorf(
						"expected X-Response header %q, got %q",
						"response",
						resp.Header.Get("X-Response"),
					)
				}

				body, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatalf(
						"cannot read response body: %v",
						err,
					)
				}

				if string(body) != `{"id":42}` {
					t.Errorf(
						"expected response body %q, got %q",
						`{"id":42}`,
						string(body),
					)
				}
			}
		})
	}
}

func TestRealHTTPServiceHostnameResolution(t *testing.T) {
	upstream := httptest.NewServer(
		http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			if r.URL.Path != "/health" {
				t.Errorf(
					"expected path %q, got %q",
					"/health",
					r.URL.Path,
				)
			}

			w.WriteHeader(http.StatusNoContent)
		}),
	)
	defer upstream.Close()

	ctrl := gomock.NewController(t)

	resolver := NewMockaddressResolver(ctrl)

	resolver.EXPECT().
		ResolveHTTPURL("http://gateway-host:9000").
		Return(upstream.URL, nil)

	logger, err := log.NewSilentLogger()
	if err != nil {
		t.Fatalf("cannot create silent logger: %v", err)
	}
	defer logger.Close()

	service, err := NewRealHTTPService(
		&RealHTTPServiceConfig{
			MaxRequestBodySize:     1 << 20,
			ReadHeaderTimeout:      5 * time.Second,
			IdleTimeout:            60 * time.Second,
			MaxRequestHeaderBytes:  1 << 20,
			ResponseHeaderTimeout:  30 * time.Second,
			MaxResponseHeaderBytes: 1 << 20,
		},
		logger,
		&model.ServiceSpec{
			ID:           "gateway",
			Kind:         model.ServiceKindHTTP,
			Mode:         model.ServiceModeReal,
			E2EnginePort: 8083,
			Target:       "http://gateway-host:9000",
		},
		resolver,
	)
	if err != nil {
		t.Fatalf("NewRealHTTPService() error = %v", err)
	}

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
		http.MethodGet,
		"http://"+service.GetRuntimeAddress()+"/health",
		nil,
	)
	if err != nil {
		t.Fatalf("cannot create request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf(
			"expected status %d, got %d",
			http.StatusNoContent,
			resp.StatusCode,
		)
	}
}

func TestRealHTTPServiceMountAddressResolutionError(t *testing.T) {
	ctrl := gomock.NewController(t)

	resolver := NewMockaddressResolver(ctrl)

	resolveErr := nativeerrors.New("cannot resolve HTTP target")

	resolver.EXPECT().
		ResolveHTTPURL("http://gateway-host:9000").
		Return("", resolveErr)

	logger, err := log.NewSilentLogger()
	if err != nil {
		t.Fatalf("cannot create silent logger: %v", err)
	}
	defer logger.Close()

	service, err := NewRealHTTPService(
		&RealHTTPServiceConfig{
			MaxRequestBodySize:     1 << 20,
			ReadHeaderTimeout:      5 * time.Second,
			IdleTimeout:            60 * time.Second,
			MaxRequestHeaderBytes:  1 << 20,
			ResponseHeaderTimeout:  30 * time.Second,
			MaxResponseHeaderBytes: 1 << 20,
		},
		logger,
		&model.ServiceSpec{
			ID:           "gateway",
			Kind:         model.ServiceKindHTTP,
			Mode:         model.ServiceModeReal,
			E2EnginePort: 8083,
			Target:       "http://gateway-host:9000",
		},
		resolver,
	)
	if err != nil {
		t.Fatalf("NewRealHTTPService() error = %v", err)
	}

	err = service.Mount(context.Background())

	if !nativeerrors.Is(err, resolveErr) {
		t.Errorf(
			"expected error %v, got %v",
			resolveErr,
			err,
		)
	}

	if service.server != nil {
		t.Error("expected server not to be created")
	}

	if service.GetRuntimeAddress() != "" {
		t.Errorf(
			"expected empty runtime address, got %q",
			service.GetRuntimeAddress(),
		)
	}
}

func TestRealHTTPServiceTargetHost(t *testing.T) {
	var targetHost string

	upstream := httptest.NewServer(
		http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			targetHost = r.Host
			w.WriteHeader(http.StatusOK)
		}),
	)
	defer upstream.Close()

	service := newTestRealHTTPService(t, upstream.URL)

	if err := service.Mount(context.Background()); err != nil {
		t.Fatalf("Mount() error = %v", err)
	}
	t.Cleanup(func() {
		if err := service.Unmount(context.Background()); err != nil {
			t.Errorf("Unmount() error = %v", err)
		}
	})

	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"http://"+service.GetRuntimeAddress()+"/users",
		nil,
	)
	if err != nil {
		t.Fatalf("cannot create request: %v", err)
	}

	req.Host = "users:8080"

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request error = %v", err)
	}
	defer resp.Body.Close()

	if targetHost != upstream.Listener.Addr().String() {
		t.Errorf(
			"expected target host %q, got %q",
			upstream.Listener.Addr().String(),
			targetHost,
		)
	}
}

func TestRealHTTPServiceHTTPS(t *testing.T) {
	upstream := httptest.NewTLSServer(
		http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			w.WriteHeader(http.StatusNoContent)
		}),
	)
	defer upstream.Close()

	service := newTestRealHTTPService(t, upstream.URL)

	if err := service.Mount(context.Background()); err != nil {
		t.Fatalf("Mount() error = %v", err)
	}
	t.Cleanup(func() {
		if err := service.Unmount(context.Background()); err != nil {
			t.Errorf("Unmount() error = %v", err)
		}
	})

	// httptest.NewTLSServer uses a test certificate which is not trusted
	// by the default transport used by ReverseProxy.
	//
	// Therefore, the proxy should return Bad Gateway for this target.

	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"http://"+service.GetRuntimeAddress()+"/users",
		nil,
	)
	if err != nil {
		t.Fatalf("cannot create request: %v", err)
	}

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
}

func TestRealHTTPServiceUpstreamUnavailable(t *testing.T) {
	upstream := httptest.NewServer(
		http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
		}),
	)

	target := upstream.URL
	upstream.Close()

	service := newTestRealHTTPService(t, target)

	if err := service.Mount(context.Background()); err != nil {
		t.Fatalf("Mount() error = %v", err)
	}
	t.Cleanup(func() {
		if err := service.Unmount(context.Background()); err != nil {
			t.Errorf("Unmount() error = %v", err)
		}
	})

	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"http://"+service.GetRuntimeAddress()+"/users",
		nil,
	)
	if err != nil {
		t.Fatalf("cannot create request: %v", err)
	}

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
}

func TestRealHTTPServiceMountCanceledContext(t *testing.T) {
	service := newTestRealHTTPService(
		t,
		"http://127.0.0.1:8080",
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := service.Mount(ctx)
	if err == nil {
		t.Fatal("expected Mount() error")
	}

	if err != context.Canceled {
		t.Errorf(
			"expected error %v, got %v",
			context.Canceled,
			err,
		)
	}

	if service.GetRuntimeAddress() != "" {
		t.Errorf(
			"expected empty runtime address, got %q",
			service.GetRuntimeAddress(),
		)
	}
}

func TestRealHTTPServiceUnmountBeforeMount(t *testing.T) {
	service := newTestRealHTTPService(
		t,
		"http://127.0.0.1:8080",
	)

	if err := service.Unmount(context.Background()); err != nil {
		t.Errorf("Unmount() error = %v", err)
	}
}

func TestRealHTTPServiceRequestHeaderSizeLimit(t *testing.T) {
	upstream := httptest.NewServer(
		http.HandlerFunc(func(
			w http.ResponseWriter,
			_ *http.Request,
		) {
			w.WriteHeader(http.StatusOK)
		}),
	)
	defer upstream.Close()

	service := newTestRealHTTPService(t, upstream.URL)
	service.cfg.MaxRequestHeaderBytes = 1024

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
			header:     strings.Repeat("a", 16<<10),
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

func TestRealHTTPServiceResponseHeaderSizeLimit(t *testing.T) {
	upstream := httptest.NewServer(
		http.HandlerFunc(func(
			w http.ResponseWriter,
			req *http.Request,
		) {
			if req.URL.Path == "/large" {
				w.Header().Set(
					"X-Large",
					strings.Repeat("a", 4096),
				)
			} else {
				w.Header().Set("X-Small", "ok")
			}

			w.WriteHeader(http.StatusOK)
		}),
	)
	defer upstream.Close()

	service := newTestRealHTTPService(t, upstream.URL)
	service.cfg.MaxResponseHeaderBytes = 1024

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
		path       string
		wantStatus int
	}{
		{
			name:       "within limit",
			path:       "/small",
			wantStatus: http.StatusOK,
		},
		{
			name:       "exceeds limit",
			path:       "/large",
			wantStatus: http.StatusBadGateway,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(
				t.Context(),
				http.MethodGet,
				"http://"+service.GetRuntimeAddress()+tt.path,
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

func TestRealHTTPServiceResponseHeaderTimeout(t *testing.T) {
	const (
		headerTimeout = 100 * time.Millisecond
		delay         = 300 * time.Millisecond
	)

	tests := []struct {
		name       string
		delayBody  bool
		wantStatus int
		wantBody   string
	}{
		{
			name:       "response headers timeout",
			wantStatus: http.StatusBadGateway,
		},
		{
			name:       "response body may arrive after timeout",
			delayBody:  true,
			wantStatus: http.StatusOK,
			wantBody:   "delayed response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := httptest.NewServer(
				http.HandlerFunc(func(
					w http.ResponseWriter,
					req *http.Request,
				) {
					if tt.delayBody {
						w.WriteHeader(http.StatusOK)

						// Send response headers immediately.
						w.(http.Flusher).Flush()

						time.Sleep(delay)

						_, _ = io.WriteString(
							w,
							"delayed response",
						)

						return
					}

					// Wait for cancellation from the proxy's
					// response-header timeout.
					<-req.Context().Done()
				}),
			)
			defer upstream.Close()

			service := newTestRealHTTPService(
				t,
				upstream.URL,
			)
			service.cfg.ResponseHeaderTimeout = headerTimeout

			if err := service.Mount(t.Context()); err != nil {
				t.Fatalf("Mount() error = %v", err)
			}

			defer func() {
				if err := service.Unmount(
					context.Background(),
				); err != nil {
					t.Errorf("Unmount() error = %v", err)
				}
			}()

			req, err := http.NewRequestWithContext(
				t.Context(),
				http.MethodGet,
				"http://"+service.GetRuntimeAddress()+"/test",
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

			if resp.StatusCode != tt.wantStatus {
				t.Errorf(
					"status = %d, want %d",
					resp.StatusCode,
					tt.wantStatus,
				)
			}

			if tt.delayBody {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatalf("ReadAll() error = %v", err)
				}

				if string(body) != tt.wantBody {
					t.Errorf(
						"body = %q, want %q",
						body,
						tt.wantBody,
					)
				}
			}
		})
	}
}

func TestRealHTTPServiceUnmountClosesActiveRequest(t *testing.T) {
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

	service := newTestRealHTTPService(t, upstream.URL)

	if err := service.Mount(t.Context()); err != nil {
		t.Fatalf("Mount() error = %v", err)
	}

	address := service.GetRuntimeAddress()

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

			requestDone <- nativeerrors.New(
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

	err := service.Unmount(ctx)

	if err != nil && !nativeerrors.Is(err, context.Canceled) {
		t.Errorf(
			"Unmount() error = %v, want nil or context.Canceled",
			err,
		)
	}

	select {
	case <-requestDone:
	case <-time.After(5 * time.Second):
		t.Error("active client request did not terminate")
	}

	select {
	case <-upstreamCanceled:
	case <-time.After(5 * time.Second):
		t.Error("upstream request was not canceled")
	}

	conn, err := net.DialTimeout(
		"tcp",
		address,
		time.Second,
	)
	if err == nil {
		_ = conn.Close()

		t.Error(
			"listener still accepts connections after unmount",
		)
	}
}

func TestRealHTTPServiceRequestBodySizeLimit(t *testing.T) {
	const limit = int64(16)

	var upstreamCalls atomic.Int32

	upstream := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			upstreamCalls.Add(1)

			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("upstream ReadAll() error = %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(body)
		}),
	)
	defer upstream.Close()

	service := newTestRealHTTPService(t, upstream.URL)
	service.cfg.MaxRequestBodySize = limit

	if err := service.Mount(t.Context()); err != nil {
		t.Fatalf("Mount() error = %v", err)
	}

	t.Cleanup(func() {
		if err := service.Unmount(context.Background()); err != nil {
			t.Errorf("Unmount() error = %v", err)
		}
	})

	tests := []struct {
		name          string
		body          string
		chunked       bool
		wantStatus    int
		wantForwarded bool
	}{
		{
			name:          "below limit",
			body:          strings.Repeat("a", int(limit)-1),
			wantStatus:    http.StatusOK,
			wantForwarded: true,
		},
		{
			name:          "exactly at limit",
			body:          strings.Repeat("a", int(limit)),
			wantStatus:    http.StatusOK,
			wantForwarded: true,
		},
		{
			name:       "known length exceeds limit",
			body:       strings.Repeat("a", int(limit)+1),
			wantStatus: http.StatusRequestEntityTooLarge,
		},
		{
			name:          "chunked within limit",
			body:          strings.Repeat("a", int(limit)),
			chunked:       true,
			wantStatus:    http.StatusOK,
			wantForwarded: true,
		},
		{
			name:       "chunked exceeds limit",
			body:       strings.Repeat("a", int(limit)+1),
			chunked:    true,
			wantStatus: http.StatusRequestEntityTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := upstreamCalls.Load()

			req, err := http.NewRequestWithContext(
				t.Context(),
				http.MethodPost,
				"http://"+service.GetRuntimeAddress()+"/upload",
				strings.NewReader(tt.body),
			)
			if err != nil {
				t.Fatal(err)
			}

			if tt.chunked {
				req.ContentLength = -1
			}

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

			responseBody, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("ReadAll() error = %v", err)
			}

			if tt.wantForwarded && string(responseBody) != tt.body {
				t.Errorf(
					"body = %q, want %q",
					responseBody,
					tt.body,
				)
			}

			wantCalls := before
			if tt.wantForwarded {
				wantCalls++
			}

			if got := upstreamCalls.Load(); got != wantCalls {
				t.Errorf(
					"upstream calls = %d, want %d",
					got,
					wantCalls,
				)
			}
		})
	}
}

func newTestRealHTTPService(
	t *testing.T,
	target string,
) *RealHTTPService {
	t.Helper()

	logger, err := log.NewSilentLogger()
	if err != nil {
		t.Fatalf("cannot create silent logger: %v", err)
	}
	t.Cleanup(func() {
		if err := logger.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	ctrl := gomock.NewController(t)

	resolver := NewMockaddressResolver(ctrl)

	resolver.EXPECT().
		ResolveHTTPURL(target).
		Return(target, nil).
		AnyTimes()

	service, err := NewRealHTTPService(
		&RealHTTPServiceConfig{
			MaxRequestBodySize:     1 << 20,
			ReadHeaderTimeout:      5 * time.Second,
			IdleTimeout:            60 * time.Second,
			MaxRequestHeaderBytes:  1 << 20,
			ResponseHeaderTimeout:  30 * time.Second,
			MaxResponseHeaderBytes: 1 << 20,
		},
		logger,
		&model.ServiceSpec{
			ID:           "service-1",
			Kind:         model.ServiceKindHTTP,
			Mode:         model.ServiceModeReal,
			E2EnginePort: 8080,
			Target:       target,
		},
		resolver,
	)
	if err != nil {
		t.Fatalf(
			"NewRealHTTPService() error = %v",
			err,
		)
	}

	return service
}
