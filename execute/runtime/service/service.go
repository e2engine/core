package service

import (
	"net/http"

	"github.com/e2engine/core/execute/runtime"
	"github.com/e2engine/core/execute/runtime/config"
	grpcservice "github.com/e2engine/core/execute/runtime/service/grpc"
	httpservice "github.com/e2engine/core/execute/runtime/service/http"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/log"
)

type Provider struct {
	cfg                *config.ProviderConfig
	logger             log.Logger
	grpcMethodResolver grpcservice.MethodResolver
}

func NewProvider(
	cfg *config.ProviderConfig,
	logger log.Logger,
) *Provider {
	if cfg == nil {
		cfg = &config.ProviderConfig{}
	}

	client := http.DefaultClient // TODO: make it configurable.

	descriptorLoader := grpcservice.NewBufDescriptorLoader(
		client,
		cfg.BufDescriptorLoaderToken,
	)

	methodResolver := grpcservice.NewProtoMethodResolver(
		descriptorLoader,
	)

	return &Provider{
		cfg:                cfg,
		logger:             logger,
		grpcMethodResolver: methodResolver,
	}
}

func (p *Provider) GetService(
	spec *model.ServiceSpec,
) (runtime.Service, error) {
	if spec == nil {
		return nil, errors.ErrUnsupportedEnvironmentServiceConfiguration
	}

	switch {
	case spec.Kind == model.ServiceKindHTTP &&
		spec.Mode == model.ServiceModeMocked:
		return httpservice.NewMockHTTPService(
			p.logger,
			spec,
		), nil

	case spec.Kind == model.ServiceKindHTTP &&
		spec.Mode == model.ServiceModeReal:
		return httpservice.NewRealHTTPService(
			p.logger,
			spec,
		)

	case spec.Kind == model.ServiceKindGRPC &&
		spec.Mode == model.ServiceModeMocked:
		return grpcservice.NewMockGRPCService(
			p.logger,
			spec,
			p.grpcMethodResolver,
		), nil

	case spec.Kind == model.ServiceKindGRPC &&
		spec.Mode == model.ServiceModeReal:
		return grpcservice.NewRealGRPCService(
			p.logger,
			spec,
		)

	default:
		return nil,
			errors.ErrUnsupportedEnvironmentServiceConfiguration
	}
}

func (p *Provider) GetGRPCMethodResolver() grpcservice.MethodResolver {
	return p.grpcMethodResolver
}
