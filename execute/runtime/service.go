package runtime

import (
	"context"

	"github.com/e2engine/core/execute/runtime/service/grpc"
	"github.com/e2engine/core/model"
)

type ServiceProvider interface {
	GetService(spec *model.ServiceSpec) (Service, error)
	GetGRPCMethodResolver() grpc.MethodResolver
}

type Service interface {
	Mountable
	GetRuntimeAddress() string
}

type Mountable interface {
	Mount(ctx context.Context) error
	Unmount(ctx context.Context) error
}
