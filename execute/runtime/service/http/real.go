package http

import (
	"context"
	nativeerrors "errors"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/keys"
	"github.com/e2engine/core/pkg/log"
)

// RealHTTPService exposes a real HTTP service through an execution-local
// runtime address.
//
// ServiceSpec.Address is the logical address owned by EnvironmentRouter.
// ServiceSpec.Target is the actual address of the real service.
type RealHTTPService struct {
	logger log.Logger
	spec   *model.ServiceSpec

	server         *http.Server
	runtimeAddress string
}

func NewRealHTTPService(
	logger log.Logger,
	spec *model.ServiceSpec,
) (*RealHTTPService, error) {
	return &RealHTTPService{
		logger: logger.With(
			log.String(keys.EnvironmentServiceID, spec.ID),
			log.String(keys.EnvironmentServiceKind, string(spec.Kind)),
			log.String(keys.EnvironmentServiceMode, string(spec.Mode)),
		),
		spec: spec,
	}, nil
}

func (s *RealHTTPService) Mount(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	target, err := url.Parse(s.spec.HTTPTarget)
	if err != nil {
		return err
	}

	proxy := &httputil.ReverseProxy{
		Rewrite: func(preq *httputil.ProxyRequest) {
			preq.SetURL(target)
		},
	}

	proxy.ErrorHandler = func(
		w http.ResponseWriter,
		req *http.Request,
		proxyErr error,
	) {
		s.logger.Error(
			"Failed to proxy request to real service",
			log.String(keys.RequestMethod, req.Method),
			log.String(keys.RequestURLPath, req.URL.Path),
			log.Err(keys.Cause, proxyErr),
		)

		http.Error(
			w,
			http.StatusText(http.StatusBadGateway),
			http.StatusBadGateway,
		)
	}

	listenerConfig := net.ListenConfig{}
	listener, err := listenerConfig.Listen(
		ctx,
		"tcp",
		"127.0.0.1:0",
	)
	if err != nil {
		return err
	}

	s.runtimeAddress = listener.Addr().String()

	s.server = &http.Server{
		Handler: proxy,
	}

	go func() {
		err := s.server.Serve(listener)
		if err != nil &&
			!nativeerrors.Is(err, http.ErrServerClosed) {
			s.logger.Error(
				"HTTP real service proxy stopped unexpectedly",
				log.Err(keys.Cause, err),
			)
		}
	}()

	s.logger.Debug(
		"Mounted service",
		log.String(
			keys.EnvironmentServiceRuntimeAddress,
			s.runtimeAddress,
		),
	)

	return nil
}

func (s *RealHTTPService) GetRuntimeAddress() string {
	return s.runtimeAddress
}

func (s *RealHTTPService) Unmount(ctx context.Context) error {
	if s.server == nil {
		return nil
	}

	return s.server.Shutdown(ctx)
}
