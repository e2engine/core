package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/e2engine/core/execute/runtime/config"
	grpcservice "github.com/e2engine/core/execute/runtime/service/grpc"
	httpservice "github.com/e2engine/core/execute/runtime/service/http"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/log"
)

func TestNewProvider(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.ProviderConfig
	}{
		{
			name: "nil config",
		},
		{
			name: "config",
			cfg: &config.ProviderConfig{
				BufDescriptorLoaderToken: "token",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewProvider(tt.cfg, nil, nil)

			if provider == nil {
				t.Fatal("expected provider")
			}

			if provider.GetGRPCMethodResolver() == nil {
				t.Fatal("expected gRPC method resolver")
			}
		})
	}
}

func TestProvider_GetService(t *testing.T) {
	tests := []struct {
		name string
		spec *model.ServiceSpec
		want any
	}{
		{
			name: "mocked HTTP service",
			spec: &model.ServiceSpec{
				Kind: model.ServiceKindHTTP,
				Mode: model.ServiceModeMocked,
			},
			want: (*httpservice.MockHTTPService)(nil),
		},
		{
			name: "real HTTP service",
			spec: &model.ServiceSpec{
				Kind: model.ServiceKindHTTP,
				Mode: model.ServiceModeReal,
			},
			want: (*httpservice.RealHTTPService)(nil),
		},
		{
			name: "mocked gRPC service",
			spec: &model.ServiceSpec{
				Kind: model.ServiceKindGRPC,
				Mode: model.ServiceModeMocked,
			},
			want: (*grpcservice.MockGRPCService)(nil),
		},
		{
			name: "real gRPC service",
			spec: &model.ServiceSpec{
				Kind: model.ServiceKindGRPC,
				Mode: model.ServiceModeReal,
			},
			want: (*grpcservice.RealGRPCService)(nil),
		},
	}

	logger, err := log.NewSilentLogger()
	if err != nil {
		t.Fatalf("Unexpected NewSilentLogger error: %q", err)
	}
	provider := NewProvider(nil, logger, nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := provider.GetService(tt.spec)
			if err != nil {
				t.Fatalf("GetService() error = %v", err)
			}

			if got == nil {
				t.Fatal("expected service")
			}

			if reflect.TypeOf(got) != reflect.TypeOf(tt.want) {
				t.Fatalf(
					"expected service type %T, got %T",
					tt.want,
					got,
				)
			}
		})
	}
}

func TestProvider_GetService_InvalidConfiguration(t *testing.T) {
	tests := []struct {
		name string
		spec *model.ServiceSpec
	}{
		{
			name: "nil spec",
		},
		{
			name: "unsupported kind",
			spec: &model.ServiceSpec{
				Kind: "unsupported",
				Mode: model.ServiceModeReal,
			},
		},
		{
			name: "unsupported mode",
			spec: &model.ServiceSpec{
				Kind: model.ServiceKindHTTP,
				Mode: "unsupported",
			},
		},
	}

	provider := NewProvider(nil, nil, nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := provider.GetService(tt.spec)

			if got != nil {
				t.Fatalf("expected nil service, got %T", got)
			}

			if !errors.Is(
				err,
				errors.ErrUnsupportedEnvironmentServiceConfiguration,
			) {
				t.Fatalf(
					"expected ErrUnsupportedEnvironmentServiceConfiguration, got %v",
					err,
				)
			}
		})
	}
}

func TestProvider_GetService_AddressResolver(t *testing.T) {
	upstream := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/health" {
				t.Errorf(
					"expected path /health, got %q",
					r.URL.Path,
				)
			}

			w.WriteHeader(http.StatusNoContent)
		}),
	)
	defer upstream.Close()

	logger, err := log.NewSilentLogger()
	if err != nil {
		t.Fatalf("NewSilentLogger() error = %v", err)
	}
	defer logger.Close()

	ctrl := gomock.NewController(t)

	addressResolver := NewMockaddressResolver(ctrl)

	addressResolver.EXPECT().
		ResolveHTTPURL("http://gateway-host:9000").
		Return(upstream.URL, nil)

	provider := NewProvider(
		nil,
		logger,
		addressResolver,
	)

	created, err := provider.GetService(&model.ServiceSpec{
		ID:           "gateway",
		Kind:         model.ServiceKindHTTP,
		Mode:         model.ServiceModeReal,
		E2EnginePort: 8083,
		Target:       "http://gateway-host:9000",
	})
	if err != nil {
		t.Fatalf("GetService() error = %v", err)
	}

	ctx := t.Context()

	if err := created.Mount(ctx); err != nil {
		t.Fatalf("Mount() error = %v", err)
	}

	defer func() {
		if err := created.Unmount(context.Background()); err != nil {
			t.Errorf("Unmount() error = %v", err)
		}
	}()

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		"http://"+created.GetRuntimeAddress()+"/health",
		nil,
	)
	if err != nil {
		t.Fatalf("NewRequestWithContext() error = %v", err)
	}

	resp, err := upstream.Client().Do(req)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
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
