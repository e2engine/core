package runtime

import (
	"context"

	"github.com/e2engine/core/execute/call"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/log"
)

type RouterProvider interface {
	Get(logger log.Logger, recorder call.Recorder) Router
}

type Router interface {
	Register(route *Route)
	Mount(ctx context.Context) error
	Unmount(ctx context.Context) error
}

type Route struct {
	ServiceID      string
	Kind           model.ServiceKind
	ListenAddress  string
	RuntimeAddress string
}
