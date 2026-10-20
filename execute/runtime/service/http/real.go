package http

import (
	"bytes"
	"context"
	nativeerrors "errors"
	"io"
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
// ServiceSpec.Target is the actual address of the real service.
type RealHTTPService struct {
	cfg    *RealHTTPServiceConfig
	logger log.Logger
	spec   *model.ServiceSpec

	addressResolver addressResolver

	server         *http.Server
	transport      *http.Transport
	runtimeAddress string
}

func NewRealHTTPService(
	cfg *RealHTTPServiceConfig,
	logger log.Logger,
	spec *model.ServiceSpec,
	addressResolver addressResolver,
) (*RealHTTPService, error) {
	return &RealHTTPService{
		cfg: cfg,
		logger: logger.With(
			log.String(keys.EnvironmentServiceID, spec.ID),
			log.String(keys.EnvironmentServiceKind, string(spec.Kind)),
			log.String(keys.EnvironmentServiceMode, string(spec.Mode)),
		),
		spec:            spec,
		addressResolver: addressResolver,
	}, nil
}

func (s *RealHTTPService) Mount(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	resolvedTarget, err := s.addressResolver.ResolveHTTPURL(s.spec.Target)
	if err != nil {
		return err
	}

	target, err := url.Parse(resolvedTarget)
	if err != nil {
		return err
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()

	transport.ResponseHeaderTimeout = s.cfg.ResponseHeaderTimeout
	transport.MaxResponseHeaderBytes = s.cfg.MaxResponseHeaderBytes

	proxy := &httputil.ReverseProxy{
		Transport: transport,
		Rewrite: func(preq *httputil.ProxyRequest) {
			// SetURL uses the resolved target hostname for the upstream Host header.
			// Services using virtual-host routing may require the original logical
			// hostname instead. Revisit Host/SNI handling if such services are supported.
			preq.SetURL(target)
		},
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, _ error) {
		s.logger.Error(
			"Failed to proxy request to real service",
			log.String(keys.RequestMethod, req.Method),
		)

		http.Error(
			w,
			http.StatusText(http.StatusBadGateway),
			http.StatusBadGateway,
		)
	}

	proxyHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		limit := s.cfg.MaxRequestBodySize

		// Reject requests whose declared body size exceeds the limit.
		if req.ContentLength > limit {
			http.Error(
				w,
				http.StatusText(http.StatusRequestEntityTooLarge),
				http.StatusRequestEntityTooLarge,
			)
			return
		}

		// Read at most limit+1 bytes so requests without a known
		// Content-Length are also subject to the size limit.
		body, errRead := io.ReadAll(io.LimitReader(req.Body, limit+1))
		if errRead != nil {
			http.Error(
				w,
				http.StatusText(http.StatusBadRequest),
				http.StatusBadRequest,
			)
			return
		}

		if int64(len(body)) > limit {
			http.Error(
				w,
				http.StatusText(http.StatusRequestEntityTooLarge),
				http.StatusRequestEntityTooLarge,
			)
			return
		}

		// Restore the body for the reverse proxy.
		req.Body = io.NopCloser(bytes.NewReader(body))
		req.ContentLength = int64(len(body))

		proxy.ServeHTTP(w, req)
	})

	listenerConfig := net.ListenConfig{}
	listener, err := listenerConfig.Listen(
		ctx,
		"tcp",
		"127.0.0.1:0",
	)
	if err != nil {
		transport.CloseIdleConnections()
		return err
	}

	s.runtimeAddress = listener.Addr().String()
	s.transport = transport

	s.server = &http.Server{
		Handler:           proxyHandler,
		ReadHeaderTimeout: s.cfg.ReadHeaderTimeout,
		IdleTimeout:       s.cfg.IdleTimeout,
		MaxHeaderBytes:    s.cfg.MaxRequestHeaderBytes,
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

	err := s.server.Shutdown(ctx)
	if err != nil {
		err = nativeerrors.Join(
			err,
			s.server.Close(),
		)
	}

	if s.transport != nil {
		s.transport.CloseIdleConnections()
	}

	return err
}
