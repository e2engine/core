package tests

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/e2engine/core/execute"
	"github.com/e2engine/core/execute/address"
	"github.com/e2engine/core/execute/call"
	"github.com/e2engine/core/execute/runtime"
	"github.com/e2engine/core/execute/runtime/config"
	"github.com/e2engine/core/execute/runtime/router"
	"github.com/e2engine/core/execute/runtime/service"
	"github.com/e2engine/core/execute/worker"
	grpcworker "github.com/e2engine/core/execute/worker/grpc"
	httpworker "github.com/e2engine/core/execute/worker/http"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/log"
)

func TestCallStoreLimitPropagation(t *testing.T) {
	tests := []struct {
		name              string
		maxCalls          int
		maxRecordedBytes  int64
		wantRecordedCalls int
		wantStatus        model.ExecutionStatus
		wantError         bool
	}{
		{
			name:              "below both limits",
			maxCalls:          3,
			maxRecordedBytes:  1 << 20,
			wantRecordedCalls: 2,
			wantStatus:        model.ExecutionStatusPassed,
		},
		{
			name:              "call count exactly at limit",
			maxCalls:          2,
			maxRecordedBytes:  1 << 20,
			wantRecordedCalls: 2,
			wantStatus:        model.ExecutionStatusPassed,
		},
		{
			name:              "call count exceeded",
			maxCalls:          1,
			maxRecordedBytes:  1 << 20,
			wantRecordedCalls: 1,
			wantStatus:        model.ExecutionStatusError,
			wantError:         true,
		},
		{
			name:              "recorded bytes exceeded",
			maxCalls:          3,
			maxRecordedBytes:  1,
			wantRecordedCalls: 0,
			wantStatus:        model.ExecutionStatusError,
			wantError:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(
				context.Background(),
				10*time.Second,
			)
			defer cancel()

			logger, err := log.NewSilentLogger()
			if err != nil {
				t.Fatalf("NewSilentLogger() error = %v", err)
			}

			resolver, err := address.NewResolver(nil)
			if err != nil {
				t.Fatalf("NewResolver() error = %v", err)
			}

			// The actual service behind E2Engine's HTTP router.
			upstream := httptest.NewServer(
				http.HandlerFunc(
					func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusOK)
					},
				),
			)
			defer upstream.Close()

			// Allocate a port for the environment router.
			routerPort := reserveTCPPort(t)

			environment := model.Environment{
				Kind:    model.ResourceKindEnvironment,
				ID:      "7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3896",
				Name:    "test-environment",
				Version: "1.0.0",
				Spec: model.EnvironmentSpec{
					Services: []model.ServiceSpec{
						{
							ID:           "service-1",
							Kind:         model.ServiceKindHTTP,
							Mode:         model.ServiceModeReal,
							E2EnginePort: routerPort,
							Target:       upstream.URL,
						},
					},
				},
			}

			runtimeCfg := &config.Config{
				E2EngineListenInterface:   "127.0.0.1",
				EnvironmentCleanupTimeout: 5 * time.Second,
			}

			serviceProvider := service.NewProvider(
				&config.ServiceConfig{},
				logger,
				resolver,
			)

			routerProvider := router.NewProvider(
				&config.RouterConfig{
					HTTP: &config.HTTPRouterConfig{
						MaxRecordedBodySize: 1 << 20,
					},
					GRPC: &config.GRPCRouterConfig{
						MaxMessageSize: 4 << 20,
					},
				},
			)

			storeFactory := call.NewStoreFactory(
				&call.Config{
					MaxCalls:         tt.maxCalls,
					MaxRecordedBytes: tt.maxRecordedBytes,
				},
			)

			controller := runtime.NewEnvironmentController(
				runtimeCfg,
				logger,
				serviceProvider,
				routerProvider,
				storeFactory,
			)

			const executionID = "execution-1"

			instance, created, err := controller.Acquire(
				ctx,
				executionID,
				&environment,
			)
			if err != nil {
				t.Fatalf("Acquire() error = %v", err)
			}

			if !created {
				t.Fatal("expected a new environment instance")
			}

			defer func() {
				cleanupCtx, cleanupCancel := context.WithTimeout(
					context.Background(),
					5*time.Second,
				)
				defer cleanupCancel()

				if err := controller.ReleaseAll(cleanupCtx); err != nil {
					t.Errorf("ReleaseAll() error = %v", err)
				}
			}()

			routerURL := "http://" + net.JoinHostPort(
				"127.0.0.1",
				strconv.Itoa(routerPort),
			)

			// The SUT makes two requests through the environment
			// router before responding to the worker.
			sut := httptest.NewServer(
				http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						for range 2 {
							req, err := http.NewRequestWithContext(
								r.Context(),
								http.MethodGet,
								routerURL+"/downstream",
								nil,
							)
							if err != nil {
								t.Errorf("NewRequestWithContext() error = %v", err)
								w.WriteHeader(http.StatusInternalServerError)
								return
							}

							resp, err := http.DefaultClient.Do(req)
							if err != nil {
								t.Errorf("downstream request error = %v", err)
								w.WriteHeader(http.StatusBadGateway)
								return
							}

							_ = resp.Body.Close()

							if resp.StatusCode != http.StatusOK {
								t.Errorf(
									"downstream status = %d, want %d",
									resp.StatusCode,
									http.StatusOK,
								)

								w.WriteHeader(http.StatusBadGateway)
								return
							}
						}

						w.WriteHeader(http.StatusOK)
					},
				),
			)
			defer sut.Close()

			test := model.Test{
				Kind:    model.ResourceKindTest,
				ID:      "ae619f9418f89041a5f6e0e952eae9fb9495e2c271b66739ffd6fae46f9ff30a",
				Name:    "test-1",
				Version: "1.0.0",
				Spec: model.TestSpec{
					Request: model.RequestSpec{
						HTTP: &model.HTTPRequest{
							Method: http.MethodGet,
							URL:    sut.URL + "/execute",
						},
					},
					Expect: model.ExpectSpec{
						HTTP: &model.HTTPResponsePattern{
							StatusCode: http.StatusOK,
						},
					},
				},
			}

			testWorker := worker.NewWorker(
				&worker.Config{
					TestTimeout: 5 * time.Second,
					HTTPEvaluator: &httpworker.EvaluatorConfig{
						MaxResponseBodySize: 1 << 20,
					},
					GRPCExecutor: &grpcworker.ExecutorConfig{
						MaxMessageSize: 4 << 20,
					},
				},
				logger,
				serviceProvider.GetGRPCMethodResolver(),
				resolver,
			)

			job := execute.TestJob{
				ExecutionID: executionID,
				Environment: environment,
				Test:        test,
				Runtime: execute.TestExecutionRuntime{
					Calls: instance.Calls,
				},
			}

			result, err := testWorker.Execute(ctx, job)
			if err != nil {
				t.Fatalf("Worker.Execute() error = %v", err)
			}

			if result == nil {
				t.Fatal("expected execution result")
			}

			if result.Status != tt.wantStatus {
				t.Errorf(
					"execution status = %q, want %q",
					result.Status,
					tt.wantStatus,
				)
			}

			if result.Summary == nil {
				t.Fatal("expected execution summary")
			}

			if tt.wantError {
				if !strings.Contains(
					result.Summary.Error,
					errors.ErrCallStoreLimitExceeded.Error(),
				) {
					t.Errorf(
						"execution error = %q, want call store limit error",
						result.Summary.Error,
					)
				}
			} else {
				if result.Summary.Error != "" {
					t.Errorf(
						"unexpected execution error = %q",
						result.Summary.Error,
					)
				}
			}

			if got := len(instance.Calls.Get(call.Filter{})); got != tt.wantRecordedCalls {
				t.Errorf(
					"recorded calls = %d, want %d",
					got,
					tt.wantRecordedCalls,
				)
			}

			if tt.wantError {
				if err := instance.Calls.Err(); err != errors.ErrCallStoreLimitExceeded {
					t.Errorf(
						"call store error = %v, want %v",
						err,
						errors.ErrCallStoreLimitExceeded,
					)
				}
			} else {
				if err := instance.Calls.Err(); err != nil {
					t.Errorf(
						"unexpected call store error = %v",
						err,
					)
				}
			}
		})
	}
}

func reserveTCPPort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port

	if err := listener.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	return port
}
