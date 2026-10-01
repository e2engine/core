package router

import (
	"context"
	"errors"

	"github.com/e2engine/core/execute/call"
	"github.com/e2engine/core/execute/runtime"
	grpcrouter "github.com/e2engine/core/execute/runtime/router/grpc"
	httprouter "github.com/e2engine/core/execute/runtime/router/http"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/keys"
	"github.com/e2engine/core/pkg/log"
)

// Router is a composite environment router.
type Router struct {
	http *httprouter.Router
	grpc *grpcrouter.Router
}

func (r *Router) Mount(ctx context.Context) error {
	if err := r.http.Mount(ctx); err != nil {
		return err
	}

	if err := r.grpc.Mount(ctx); err != nil {
		return err
	}

	return nil
}

func (r *Router) Unmount(ctx context.Context) error {
	httpErr := r.http.Unmount(ctx)
	grpcErr := r.grpc.Unmount(ctx)

	return errors.Join(httpErr, grpcErr)
}

func (r *Router) Register(route *runtime.Route) {
	switch route.Kind {
	case model.ServiceKindHTTP:
		r.http.Register(route)

	case model.ServiceKindGRPC:
		r.grpc.Register(route)
	}
}

type Provider struct{}

func NewProvider() *Provider {
	return &Provider{}
}

func (p *Provider) Get(logger log.Logger, calls call.Recorder) runtime.Router {
	return &Router{
		http: httprouter.NewRouter(
			logger.With(log.String(keys.Component, "environment_http_router")),
			calls,
		),
		grpc: grpcrouter.NewRouter(
			logger.With(log.String(keys.Component, "environment_grpc_router")),
			calls,
		),
	}
}
