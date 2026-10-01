package service_test

import (
	"reflect"
	"testing"

	"github.com/e2engine/core/execute/runtime/config"
	"github.com/e2engine/core/execute/runtime/service"
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
			provider := service.NewProvider(tt.cfg, nil)

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
	provider := service.NewProvider(nil, logger)

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

	provider := service.NewProvider(nil, nil)

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
