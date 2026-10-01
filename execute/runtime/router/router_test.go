package router

import (
	"context"
	"errors"
	"testing"

	"github.com/e2engine/core/execute/call"
	"github.com/e2engine/core/execute/runtime"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/log"
)

func TestRouterMountUnmount(t *testing.T) {
	r := newTestRouter(t)

	ctx := context.Background()

	if err := r.Mount(ctx); err != nil {
		t.Fatalf("Mount() error = %v", err)
	}

	if err := r.Unmount(ctx); err != nil {
		t.Fatalf("Unmount() error = %v", err)
	}
}

func TestRouterMountCancelledContext(t *testing.T) {
	r := newTestRouter(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := r.Mount(ctx)

	if err != context.Canceled {
		t.Fatalf(
			"expected context.Canceled, got %v",
			err,
		)
	}
}

func TestRouterUnmountCancelledContext(t *testing.T) {
	r := newTestRouter(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := r.Unmount(ctx)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"expected context.Canceled, got %v",
			err,
		)
	}
}

func TestRouterRegisterHTTP(t *testing.T) {
	r := newTestRouter(t)

	r.Register(&runtime.Route{
		ServiceID:      "http-service",
		Kind:           model.ServiceKindHTTP,
		ListenAddress:  "127.0.0.1:0",
		RuntimeAddress: "127.0.0.1:10001",
	})

	if err := r.Mount(context.Background()); err != nil {
		t.Fatalf("Mount() error = %v", err)
	}

	if err := r.Unmount(context.Background()); err != nil {
		t.Fatalf("Unmount() error = %v", err)
	}
}

func TestRouterRegisterGRPC(t *testing.T) {
	r := newTestRouter(t)

	r.Register(&runtime.Route{
		ServiceID:      "grpc-service",
		Kind:           model.ServiceKindGRPC,
		ListenAddress:  "127.0.0.1:0",
		RuntimeAddress: "127.0.0.1:10001",
	})

	if err := r.Mount(context.Background()); err != nil {
		t.Fatalf("Mount() error = %v", err)
	}

	if err := r.Unmount(context.Background()); err != nil {
		t.Fatalf("Unmount() error = %v", err)
	}
}

func TestRouterRegisterHTTPAndGRPC(t *testing.T) {
	r := newTestRouter(t)

	r.Register(&runtime.Route{
		ServiceID:      "http-service",
		Kind:           model.ServiceKindHTTP,
		ListenAddress:  "127.0.0.1:0",
		RuntimeAddress: "127.0.0.1:10001",
	})

	r.Register(&runtime.Route{
		ServiceID:      "grpc-service",
		Kind:           model.ServiceKindGRPC,
		ListenAddress:  "127.0.0.1:0",
		RuntimeAddress: "127.0.0.1:10002",
	})

	if err := r.Mount(context.Background()); err != nil {
		t.Fatalf("Mount() error = %v", err)
	}

	if err := r.Unmount(context.Background()); err != nil {
		t.Fatalf("Unmount() error = %v", err)
	}
}

func newTestRouter(t *testing.T) runtime.Router {
	t.Helper()

	logger, err := log.NewSilentLogger()
	if err != nil {
		t.Fatalf(
			"cannot create silent logger: %v",
			err,
		)
	}

	return NewProvider().Get(
		logger,
		call.NewStoreFactory().New(),
	)
}
