package http

import (
	"context"
	nativeerrors "errors"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/ygrebnov/errorc"

	"github.com/e2engine/core/execute/call"
	"github.com/e2engine/core/execute/runtime"
	"github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/keys"
	"github.com/e2engine/core/pkg/log"
)

type Router struct {
	logger log.Logger
	calls  call.Recorder

	services map[string]*runtime.Route
	mounted  []mountedRoute
}

func NewRouter(
	logger log.Logger,
	calls call.Recorder,
) *Router {
	return &Router{
		logger:   logger,
		services: make(map[string]*runtime.Route),
		calls:    calls,
	}
}

func (r *Router) Register(route *runtime.Route) {
	r.services[route.ServiceID] = route
	r.logger.Debug(
		"Registered route",
		log.String(keys.EnvironmentServiceID, route.ServiceID),
		log.String(keys.EnvironmentServiceListenAddress, route.ListenAddress),
		log.String(keys.EnvironmentServiceRuntimeAddress, route.RuntimeAddress),
	)
}

type mountedRoute struct {
	route    *runtime.Route
	listener net.Listener
	server   *http.Server
}

func (r *Router) Mount(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	mounted := make([]mountedRoute, 0, len(r.services))

	for _, route := range r.services {
		target, err := url.Parse("http://" + route.RuntimeAddress)
		if err != nil {
			return nativeerrors.Join(
				err,
				r.unmountMountedRoutes(ctx, mounted),
			)
		}

		proxy := httputil.NewSingleHostReverseProxy(target)

		proxy.ErrorHandler = func(
			w http.ResponseWriter,
			req *http.Request,
			proxyErr error,
		) {
			r.logger.Error(
				"Failed to proxy environment request",
				log.String(keys.EnvironmentServiceID, route.ServiceID),
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

		handler := http.HandlerFunc(func(
			w http.ResponseWriter,
			req *http.Request,
		) {
			r.logger.Debug(
				"Routing environment request",
				log.String(keys.EnvironmentServiceID, route.ServiceID),
				log.String(keys.RequestMethod, req.Method),
				log.String(keys.RequestURLPath, req.URL.Path),
			)

			request, errCapture := captureRequest(req)
			if errCapture != nil {
				captureErr := errorc.With(
					errors.ErrFailedToCaptureEnvironmentServiceCall,
					errorc.String(keys.EnvironmentServiceID, route.ServiceID),
					errorc.String(keys.RequestMethod, req.Method),
					errorc.String(keys.RequestURLPath, req.URL.Path),
					errorc.Error(keys.Cause, errCapture),
				)

				r.logger.Error(
					errors.ErrFailedToCaptureEnvironmentServiceCall.Error(),
					log.String(keys.EnvironmentServiceID, route.ServiceID),
					log.String(keys.RequestMethod, req.Method),
					log.String(keys.RequestURLPath, req.URL.Path),
					log.Err(keys.Cause, captureErr),
				)

				http.Error(
					w,
					http.StatusText(http.StatusInternalServerError),
					http.StatusInternalServerError,
				)

				return
			}

			responseRecorder := newResponseRecorder(w)

			proxy.ServeHTTP(responseRecorder, req)

			r.calls.Record(call.Call{
				TestExecutionID: req.Header.Get(string(keys.E2EngineTestExecutionID)),
				ServiceID:       route.ServiceID,
				HTTP: &call.HTTPCall{
					Request:  request,
					Response: responseRecorder.GetResponse(),
				},
			})
		})

		listenerConfig := net.ListenConfig{}
		listener, err := listenerConfig.Listen(
			ctx,
			"tcp",
			route.ListenAddress,
		)
		if err != nil {
			return nativeerrors.Join(
				err,
				r.unmountMountedRoutes(ctx, mounted),
			)
		}

		server := &http.Server{
			Handler: handler,
		}

		mounted = append(
			mounted,
			mountedRoute{
				route:    route,
				listener: listener,
				server:   server,
			},
		)

		go func() {
			err := server.Serve(listener)
			if err != nil &&
				!nativeerrors.Is(err, http.ErrServerClosed) {
				r.logger.Error(
					"Environment router listener stopped unexpectedly",
					log.String(
						keys.EnvironmentServiceID,
						route.ServiceID,
					),
					log.String(
						keys.EnvironmentServiceListenAddress,
						route.ListenAddress,
					),
					log.Err(keys.Cause, err),
				)
			}
		}()
	}

	r.mounted = mounted

	r.logger.Debug("Mounted router", log.Int("routes", len(r.services)))

	return nil
}

func (r *Router) Unmount(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	err := r.unmountMountedRoutes(ctx, r.mounted)
	r.mounted = nil

	return err
}

func (r *Router) unmountMountedRoutes(
	ctx context.Context,
	mounted []mountedRoute,
) error {
	var accErr error

	for i := len(mounted) - 1; i >= 0; i-- {
		route := mounted[i]

		if err := route.server.Shutdown(ctx); err != nil {
			r.logger.Error(
				"Failed to unmount router route",
				log.String(keys.EnvironmentServiceID, route.route.ServiceID),
				log.String(keys.EnvironmentServiceListenAddress, route.route.ListenAddress),
				log.Err(keys.Cause, err),
			)

			accErr = nativeerrors.Join(accErr, err)
		}
	}

	return accErr
}
