package runtime_test

import (
	"context"
	nativeerrors "errors"
	"fmt"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/e2engine/core/execute/call"
	"github.com/e2engine/core/execute/runtime"
	"github.com/e2engine/core/execute/runtime/config"
	"github.com/e2engine/core/mock"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/log"
)

func TestEnvironmentMount(t *testing.T) {
	ctrl := gomock.NewController(t)

	service1 := mock.NewMockService(ctrl)
	service2 := mock.NewMockService(ctrl)
	router := mock.NewMockRouter(ctrl)

	env := newTestEnvironment(
		t,
		[]runtime.Service{service1, service2},
		router,
	)

	gomock.InOrder(
		service1.EXPECT().
			Mount(gomock.Any()).
			Return(nil),

		service1.EXPECT().
			GetRuntimeAddress().
			Return("127.0.0.1:10001"),

		router.EXPECT().
			Register(&runtime.Route{
				ServiceID:      "service-1",
				Kind:           model.ServiceKindHTTP,
				ListenAddress:  "127.0.0.1:8081",
				RuntimeAddress: "127.0.0.1:10001",
			}),

		service2.EXPECT().
			Mount(gomock.Any()).
			Return(nil),

		service2.EXPECT().
			GetRuntimeAddress().
			Return("127.0.0.1:10002"),

		router.EXPECT().
			Register(&runtime.Route{
				ServiceID:      "service-2",
				Kind:           model.ServiceKindHTTP,
				ListenAddress:  "127.0.0.1:8082",
				RuntimeAddress: "127.0.0.1:10002",
			}),

		router.EXPECT().
			Mount(gomock.Any()).
			Return(nil),
	)

	if err := env.Mount(context.Background()); err != nil {
		t.Fatalf("Mount() error = %v", err)
	}
}

func TestEnvironmentMountCancelledContext(t *testing.T) {
	ctrl := gomock.NewController(t)

	service1 := mock.NewMockService(ctrl)

	env := newTestEnvironment(
		t,
		[]runtime.Service{service1},
		mock.NewMockRouter(ctrl),
	)

	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	err := env.Mount(ctx)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"expected context.Canceled, got %v",
			err,
		)
	}
}

func TestEnvironmentMountServiceError(t *testing.T) {
	ctrl := gomock.NewController(t)

	mountErr := nativeerrors.New("mount service")

	service1 := mock.NewMockService(ctrl)
	service2 := mock.NewMockService(ctrl)
	router := mock.NewMockRouter(ctrl)

	env := newTestEnvironment(
		t,
		[]runtime.Service{service1, service2},
		router,
	)

	gomock.InOrder(
		service1.EXPECT().
			Mount(gomock.Any()).
			Return(nil),

		service1.EXPECT().
			GetRuntimeAddress().
			Return("127.0.0.1:10001"),

		router.EXPECT().
			Register(gomock.Any()),

		service2.EXPECT().
			Mount(gomock.Any()).
			Return(mountErr),

		service1.EXPECT().
			Unmount(gomock.Any()).
			Return(nil),
	)

	err := env.Mount(context.Background())

	if !errors.Is(err, errors.ErrCannotMountEnvironmentService) {
		t.Fatalf(
			"expected ErrCannotMountEnvironmentService, got %v",
			err,
		)
	}
}

func TestEnvironmentMountServiceErrorUsesCleanupContext(
	t *testing.T,
) {
	ctrl := gomock.NewController(t)

	service1 := mock.NewMockService(ctrl)
	service2 := mock.NewMockService(ctrl)
	router := mock.NewMockRouter(ctrl)

	env := newTestEnvironment(
		t,
		[]runtime.Service{service1, service2},
		router,
	)

	ctx, cancel := context.WithCancel(
		context.Background(),
	)

	gomock.InOrder(
		service1.EXPECT().
			Mount(ctx).
			Return(nil),

		service1.EXPECT().
			GetRuntimeAddress().
			Return("127.0.0.1:10001"),

		router.EXPECT().
			Register(gomock.Any()),

		service2.EXPECT().
			Mount(ctx).
			DoAndReturn(
				func(context.Context) error {
					cancel()
					return context.Canceled
				},
			),

		service1.EXPECT().
			Unmount(
				gomock.Cond(
					func(cleanupCtx context.Context) bool {
						if cleanupCtx.Err() != nil {
							return false
						}

						_, hasDeadline :=
							cleanupCtx.Deadline()

						return hasDeadline
					},
				),
			).
			Return(nil),
	)

	err := env.Mount(ctx)

	if !errors.Is(err, errors.ErrCannotMountEnvironmentService) {
		t.Fatalf(
			"expected ErrCannotMountEnvironmentService, got %v",
			err,
		)
	}
}

func TestEnvironmentMountRouterError(t *testing.T) {
	ctrl := gomock.NewController(t)

	routerErr := nativeerrors.New("mount router")

	service1 := mock.NewMockService(ctrl)
	service2 := mock.NewMockService(ctrl)
	router := mock.NewMockRouter(ctrl)

	env := newTestEnvironment(
		t,
		[]runtime.Service{service1, service2},
		router,
	)

	gomock.InOrder(
		service1.EXPECT().
			Mount(gomock.Any()).
			Return(nil),

		service1.EXPECT().
			GetRuntimeAddress().
			Return("127.0.0.1:10001"),

		router.EXPECT().
			Register(gomock.Any()),

		service2.EXPECT().
			Mount(gomock.Any()).
			Return(nil),

		service2.EXPECT().
			GetRuntimeAddress().
			Return("127.0.0.1:10002"),

		router.EXPECT().
			Register(gomock.Any()),

		router.EXPECT().
			Mount(gomock.Any()).
			Return(routerErr),

		router.EXPECT().
			Unmount(gomock.Any()).
			Return(nil),

		service2.EXPECT().
			Unmount(gomock.Any()).
			Return(nil),

		service1.EXPECT().
			Unmount(gomock.Any()).
			Return(nil),
	)

	err := env.Mount(context.Background())

	if !errors.Is(err, errors.ErrFailedToMountRouter) {
		t.Fatalf(
			"expected ErrFailedToMountRouter, got %v",
			err,
		)
	}
}

func TestEnvironmentMountJoinsCleanupError(t *testing.T) {
	ctrl := gomock.NewController(t)

	mountErr := nativeerrors.New("mount service")
	cleanupErr := nativeerrors.New("cleanup service")

	service1 := mock.NewMockService(ctrl)
	service2 := mock.NewMockService(ctrl)
	router := mock.NewMockRouter(ctrl)

	env := newTestEnvironment(
		t,
		[]runtime.Service{service1, service2},
		router,
	)

	gomock.InOrder(
		service1.EXPECT().
			Mount(gomock.Any()).
			Return(nil),

		service1.EXPECT().
			GetRuntimeAddress().
			Return("127.0.0.1:10001"),

		router.EXPECT().
			Register(gomock.Any()),

		service2.EXPECT().
			Mount(gomock.Any()).
			Return(mountErr),

		service1.EXPECT().
			Unmount(gomock.Any()).
			Return(cleanupErr),
	)

	err := env.Mount(context.Background())

	if !errors.Is(err, errors.ErrCannotMountEnvironmentService) {
		t.Fatalf(
			"expected ErrCannotMountEnvironmentService, got %v",
			err,
		)
	}

	if !errors.Is(err, errors.ErrCannotUnmountEnvironmentService) {
		t.Fatalf(
			"expected ErrCannotUnmountEnvironmentService, got %v",
			err,
		)
	}
}

func TestEnvironmentUnmount(t *testing.T) {
	ctrl := gomock.NewController(t)

	service1 := mock.NewMockService(ctrl)
	service2 := mock.NewMockService(ctrl)
	router := mock.NewMockRouter(ctrl)

	env := newTestEnvironment(
		t,
		[]runtime.Service{service1, service2},
		router,
	)

	gomock.InOrder(
		router.EXPECT().
			Unmount(gomock.Any()).
			Return(nil),

		service2.EXPECT().
			Unmount(gomock.Any()).
			Return(nil),

		service1.EXPECT().
			Unmount(gomock.Any()).
			Return(nil),
	)

	if err := env.Unmount(context.Background()); err != nil {
		t.Fatalf("Unmount() error = %v", err)
	}
}

func TestEnvironmentUnmountJoinsErrors(t *testing.T) {
	ctrl := gomock.NewController(t)

	routerErr := nativeerrors.New("unmount router")
	service1Err := nativeerrors.New("unmount service 1")
	service2Err := nativeerrors.New("unmount service 2")

	service1 := mock.NewMockService(ctrl)
	service2 := mock.NewMockService(ctrl)
	router := mock.NewMockRouter(ctrl)

	env := newTestEnvironment(
		t,
		[]runtime.Service{service1, service2},
		router,
	)

	gomock.InOrder(
		router.EXPECT().
			Unmount(gomock.Any()).
			Return(routerErr),

		service2.EXPECT().
			Unmount(gomock.Any()).
			Return(service2Err),

		service1.EXPECT().
			Unmount(gomock.Any()).
			Return(service1Err),
	)

	err := env.Unmount(context.Background())

	if !errors.Is(err, routerErr) {
		t.Fatalf(
			"expected router error to be in error chain, got %v",
			err,
		)
	}

	if !errors.Is(err, errors.ErrCannotUnmountEnvironmentService) {
		t.Fatalf(
			"expected ErrCannotUnmountEnvironmentService to be in error chain, got %v",
			err,
		)
	}
}

func TestEnvironmentControllerAcquire(t *testing.T) {
	ctrl := gomock.NewController(t)

	service := mock.NewMockService(ctrl)
	serviceProvider := mock.NewMockServiceProvider(ctrl)
	router := mock.NewMockRouter(ctrl)
	routerProvider := mock.NewMockRouterProvider(ctrl)

	controller := newTestEnvironmentController(
		t,
		serviceProvider,
		routerProvider,
	)

	env := validTestEnvironment()

	routerProvider.EXPECT().
		Get(gomock.Any(), gomock.Any()).
		Return(router)

	serviceProvider.EXPECT().
		GetService(&env.Spec.Services[0]).
		Return(service, nil)

	gomock.InOrder(
		service.EXPECT().
			Mount(gomock.Any()).
			Return(nil),

		service.EXPECT().
			GetRuntimeAddress().
			Return("127.0.0.1:10001"),

		router.EXPECT().
			Register(gomock.Any()),

		router.EXPECT().
			Mount(gomock.Any()).
			Return(nil),
	)

	instance, created, err := controller.Acquire(
		context.Background(),
		"instance-1",
		env,
	)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}

	if !created {
		t.Fatal("expected created = true")
	}

	if instance == nil {
		t.Fatal("expected instance")
	}

	if instance.ID != "instance-1" {
		t.Errorf(
			"expected instance ID instance-1, got %s",
			instance.ID,
		)
	}

	if instance.Environment == nil {
		t.Fatal("expected environment")
	}

	if instance.Calls == nil {
		t.Fatal("expected call store")
	}
}

func TestEnvironmentControllerAcquireExisting(
	t *testing.T,
) {
	ctrl := gomock.NewController(t)

	service := mock.NewMockService(ctrl)
	serviceProvider := mock.NewMockServiceProvider(ctrl)
	router := mock.NewMockRouter(ctrl)
	routerProvider := mock.NewMockRouterProvider(ctrl)

	controller := newTestEnvironmentController(
		t,
		serviceProvider,
		routerProvider,
	)

	env := validTestEnvironment()

	routerProvider.EXPECT().
		Get(gomock.Any(), gomock.Any()).
		Return(router)

	serviceProvider.EXPECT().
		GetService(&env.Spec.Services[0]).
		Return(service, nil)

	gomock.InOrder(
		service.EXPECT().
			Mount(gomock.Any()).
			Return(nil),

		service.EXPECT().
			GetRuntimeAddress().
			Return("127.0.0.1:10001"),

		router.EXPECT().
			Register(gomock.Any()),

		router.EXPECT().
			Mount(gomock.Any()).
			Return(nil),
	)

	first, created, err := controller.Acquire(
		context.Background(),
		"instance-1",
		env,
	)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}

	if !created {
		t.Fatal("expected first Acquire to create")
	}

	second, created, err := controller.Acquire(
		context.Background(),
		"instance-1",
		env,
	)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}

	if created {
		t.Fatal(
			"expected second Acquire not to create",
		)
	}

	if first != second {
		t.Fatal("expected same instance")
	}
}

func TestEnvironmentControllerAcquireMountError(
	t *testing.T,
) {
	ctrl := gomock.NewController(t)

	mountErr := nativeerrors.New("mount service")

	service := mock.NewMockService(ctrl)
	serviceProvider := mock.NewMockServiceProvider(ctrl)
	router := mock.NewMockRouter(ctrl)
	routerProvider := mock.NewMockRouterProvider(ctrl)

	controller := newTestEnvironmentController(
		t,
		serviceProvider,
		routerProvider,
	)

	env := validTestEnvironment()

	routerProvider.EXPECT().
		Get(gomock.Any(), gomock.Any()).
		Return(router)

	serviceProvider.EXPECT().
		GetService(&env.Spec.Services[0]).
		Return(service, nil)

	service.EXPECT().
		Mount(gomock.Any()).
		Return(mountErr)

	instance, created, err := controller.Acquire(
		context.Background(),
		"instance-1",
		env,
	)

	if !errors.Is(err, errors.ErrCannotMountEnvironmentService) {
		t.Fatalf(
			"expected ErrCannotMountEnvironmentService, got %v",
			err,
		)
	}

	if instance != nil {
		t.Fatal("expected nil instance")
	}

	if created {
		t.Fatal("expected created = false")
	}
}

func TestEnvironmentControllerRelease(t *testing.T) {
	ctrl := gomock.NewController(t)

	service := mock.NewMockService(ctrl)
	serviceProvider := mock.NewMockServiceProvider(ctrl)
	router := mock.NewMockRouter(ctrl)
	routerProvider := mock.NewMockRouterProvider(ctrl)

	controller := newTestEnvironmentController(
		t,
		serviceProvider,
		routerProvider,
	)

	env := validTestEnvironment()

	routerProvider.EXPECT().
		Get(gomock.Any(), gomock.Any()).
		Return(router)

	serviceProvider.EXPECT().
		GetService(&env.Spec.Services[0]).
		Return(service, nil)

	gomock.InOrder(
		service.EXPECT().
			Mount(gomock.Any()).
			Return(nil),

		service.EXPECT().
			GetRuntimeAddress().
			Return("127.0.0.1:10001"),

		router.EXPECT().
			Register(gomock.Any()),

		router.EXPECT().
			Mount(gomock.Any()).
			Return(nil),

		router.EXPECT().
			Unmount(gomock.Any()).
			Return(nil),

		service.EXPECT().
			Unmount(gomock.Any()).
			Return(nil),
	)

	_, _, err := controller.Acquire(
		context.Background(),
		"instance-1",
		env,
	)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}

	if err := controller.Release(
		context.Background(),
		"instance-1",
	); err != nil {
		t.Fatalf("Release() error = %v", err)
	}

	// Release is intentionally idempotent.
	if err := controller.Release(
		context.Background(),
		"instance-1",
	); err != nil {
		t.Fatalf(
			"second Release() error = %v",
			err,
		)
	}
}

func TestEnvironmentControllerReleaseAll(t *testing.T) {
	ctrl := gomock.NewController(t)

	firstErr := nativeerrors.New("first unmount")
	secondErr := nativeerrors.New("second unmount")

	firstService := mock.NewMockService(ctrl)
	secondService := mock.NewMockService(ctrl)

	firstRouter := mock.NewMockRouter(ctrl)
	secondRouter := mock.NewMockRouter(ctrl)

	serviceProvider := mock.NewMockServiceProvider(ctrl)
	routerProvider := mock.NewMockRouterProvider(ctrl)

	controller := newTestEnvironmentController(
		t,
		serviceProvider,
		routerProvider,
	)

	firstEnv := validTestEnvironment()
	secondEnv := validTestEnvironment()

	firstEnv.ID = "7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3891"
	firstEnv.Name = "environment-1"

	secondEnv.ID = "7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3892"
	secondEnv.Name = "environment-2"

	routerProvider.EXPECT().
		Get(gomock.Any(), gomock.Any()).
		Return(firstRouter)

	serviceProvider.EXPECT().
		GetService(&firstEnv.Spec.Services[0]).
		Return(firstService, nil)

	gomock.InOrder(
		firstService.EXPECT().
			Mount(gomock.Any()).
			Return(nil),

		firstService.EXPECT().
			GetRuntimeAddress().
			Return("127.0.0.1:10001"),

		firstRouter.EXPECT().
			Register(gomock.Any()),

		firstRouter.EXPECT().
			Mount(gomock.Any()).
			Return(nil),
	)

	_, created, err := controller.Acquire(
		context.Background(),
		"first",
		firstEnv,
	)
	if err != nil {
		t.Fatalf("Acquire(first) error = %v", err)
	}

	if !created {
		t.Fatal("expected first environment to be created")
	}

	routerProvider.EXPECT().
		Get(gomock.Any(), gomock.Any()).
		Return(secondRouter)

	serviceProvider.EXPECT().
		GetService(&secondEnv.Spec.Services[0]).
		Return(secondService, nil)

	gomock.InOrder(
		secondService.EXPECT().
			Mount(gomock.Any()).
			Return(nil),

		secondService.EXPECT().
			GetRuntimeAddress().
			Return("127.0.0.1:10002"),

		secondRouter.EXPECT().
			Register(gomock.Any()),

		secondRouter.EXPECT().
			Mount(gomock.Any()).
			Return(nil),
	)

	_, created, err = controller.Acquire(
		context.Background(),
		"second",
		secondEnv,
	)
	if err != nil {
		t.Fatalf("Acquire(second) error = %v", err)
	}

	if !created {
		t.Fatal("expected second environment to be created")
	}

	// ReleaseAll iterates over a map, so the order between
	// environments is intentionally unspecified.
	firstRouter.EXPECT().
		Unmount(gomock.Any()).
		Return(nil)

	firstService.EXPECT().
		Unmount(gomock.Any()).
		Return(firstErr)

	secondRouter.EXPECT().
		Unmount(gomock.Any()).
		Return(nil)

	secondService.EXPECT().
		Unmount(gomock.Any()).
		Return(secondErr)

	err = controller.ReleaseAll(context.Background())

	if !errors.Is(err, errors.ErrCannotUnmountEnvironmentService) {
		t.Fatalf(
			"expected ErrCannotUnmountEnvironmentService, got %v",
			err,
		)
	}

	if !errors.Is(err, errors.ErrCannotUnmountEnvironmentService) {
		t.Fatalf(
			"expected ErrCannotUnmountEnvironmentService, got %v",
			err,
		)
	}

	// ReleaseAll removes instances even when Unmount fails.
	// Verify this through the public API: these must now be no-ops.
	if err := controller.Release(
		context.Background(),
		"first",
	); err != nil {
		t.Errorf(
			"Release(first) after ReleaseAll error = %v",
			err,
		)
	}

	if err := controller.Release(
		context.Background(),
		"second",
	); err != nil {
		t.Errorf(
			"Release(second) after ReleaseAll error = %v",
			err,
		)
	}
}

func newTestEnvironment(
	t *testing.T,
	services []runtime.Service,
	router runtime.Router,
) *runtime.Environment {
	t.Helper()

	logger, err := log.NewSilentLogger()
	if err != nil {
		t.Fatalf(
			"cannot create silent logger: %v",
			err,
		)
	}

	spec := model.EnvironmentSpec{
		Services: make([]model.ServiceSpec, len(services)),
	}

	for i := range services {
		spec.Services[i] = model.ServiceSpec{
			ID:      fmt.Sprintf("service-%d", i+1),
			Kind:    model.ServiceKindHTTP,
			Mode:    model.ServiceModeMocked,
			Address: fmt.Sprintf("127.0.0.1:808%d", i+1),
			Fixtures: []model.FixtureSpec{
				{
					When: model.FixtureWhen{
						HTTP: &model.HTTPFixtureWhen{
							Method: "GET",
							Path:   "/",
						},
					},
					Then: model.FixtureThen{
						HTTP: &model.HTTPFixtureThen{
							Status: 200,
						},
					},
				},
			},
		}
	}

	ctrl := gomock.NewController(t)

	serviceProvider := mock.NewMockServiceProvider(ctrl)
	routerProvider := mock.NewMockRouterProvider(ctrl)

	for i := range spec.Services {
		serviceProvider.EXPECT().
			GetService(&spec.Services[i]).
			Return(services[i], nil)
	}

	routerProvider.EXPECT().
		Get(gomock.Any(), gomock.Any()).
		Return(router)

	modelEnv := &model.Environment{
		Kind:    model.ResourceKindEnvironment,
		ID:      "7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3896",
		Version: "1.0.0",
		Name:    "environment",
		Spec:    spec,
	}

	env, err := runtime.NewEnvironment(
		context.Background(),
		&config.Config{
			EnvironmentCleanupTimeout: time.Second,
		},
		logger,
		modelEnv,
		serviceProvider,
		routerProvider,
		call.NewStoreFactory().New(),
	)
	if err != nil {
		t.Fatalf(
			"cannot create runtime environment: %v",
			err,
		)
	}

	return env
}

func newTestEnvironmentController(
	t *testing.T,
	serviceProvider runtime.ServiceProvider,
	routerProvider runtime.RouterProvider,
) *runtime.EnvironmentController {
	t.Helper()

	logger, err := log.NewSilentLogger()
	if err != nil {
		t.Fatalf(
			"cannot create silent logger: %v",
			err,
		)
	}

	return runtime.NewEnvironmentController(
		&config.Config{
			EnvironmentCleanupTimeout: time.Second,
		},
		logger,
		serviceProvider,
		routerProvider,
		call.NewStoreFactory(),
	)
}

func validTestEnvironment() *model.Environment {
	return &model.Environment{
		ID:      "7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3896",
		Kind:    model.ResourceKindEnvironment,
		Version: "1.0.0",
		Name:    "environment",
		Spec: model.EnvironmentSpec{
			Services: []model.ServiceSpec{
				{
					ID:      "7398ab887149ec5aa583c564a659a0b0a933ef27bacb675b081a2a752d8a3896",
					Kind:    model.ServiceKindHTTP,
					Mode:    model.ServiceModeMocked,
					Address: "127.0.0.1:8081",
					Fixtures: []model.FixtureSpec{
						{
							When: model.FixtureWhen{
								HTTP: &model.HTTPFixtureWhen{
									Method: "GET",
									Path:   "/",
								},
							},
							Then: model.FixtureThen{
								HTTP: &model.HTTPFixtureThen{
									Status: 200,
								},
							},
						},
					},
				},
			},
		},
	}
}

func TestEnvironmentMountRouterErrorUsesCleanupContext(
	t *testing.T,
) {
	ctrl := gomock.NewController(t)

	service1 := mock.NewMockService(ctrl)
	service2 := mock.NewMockService(ctrl)
	router := mock.NewMockRouter(ctrl)

	env := newTestEnvironment(
		t,
		[]runtime.Service{service1, service2},
		router,
	)

	ctx, cancel := context.WithCancel(context.Background())

	cleanupContext := gomock.Cond(
		func(cleanupCtx context.Context) bool {
			if cleanupCtx.Err() != nil {
				return false
			}

			_, hasDeadline := cleanupCtx.Deadline()

			return hasDeadline
		},
	)

	gomock.InOrder(
		service1.EXPECT().
			Mount(ctx).
			Return(nil),

		service1.EXPECT().
			GetRuntimeAddress().
			Return("127.0.0.1:10001"),

		router.EXPECT().
			Register(gomock.Any()),

		service2.EXPECT().
			Mount(ctx).
			Return(nil),

		service2.EXPECT().
			GetRuntimeAddress().
			Return("127.0.0.1:10002"),

		router.EXPECT().
			Register(gomock.Any()),

		router.EXPECT().
			Mount(ctx).
			DoAndReturn(
				func(context.Context) error {
					cancel()
					return context.Canceled
				},
			),

		router.EXPECT().
			Unmount(cleanupContext).
			Return(nil),

		service2.EXPECT().
			Unmount(cleanupContext).
			Return(nil),

		service1.EXPECT().
			Unmount(cleanupContext).
			Return(nil),
	)

	err := env.Mount(ctx)

	if !errors.Is(err, errors.ErrFailedToMountRouter) {
		t.Fatalf(
			"expected ErrFailedToMountRouter, got %v",
			err,
		)
	}
}
