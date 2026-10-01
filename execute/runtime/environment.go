package runtime

import (
	"context"
	nativeerrors "errors"
	"sync"

	"github.com/ygrebnov/errorc"

	"github.com/e2engine/core/execute/call"
	"github.com/e2engine/core/execute/runtime/config"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/keys"
	"github.com/e2engine/core/pkg/log"
)

type Environment struct {
	cfg      *config.Config
	logger   log.Logger
	id       string
	spec     model.EnvironmentSpec
	services []Service
	router   Router
}

func NewEnvironment(
	ctx context.Context,
	cfg *config.Config,
	logger log.Logger,
	env *model.Environment,
	serviceProvider ServiceProvider,
	routerProvider RouterProvider,
	callRecorder call.Recorder,
) (*Environment, error) {
	if err := env.Validate(ctx); err != nil {
		return nil, err
	}

	envLogger := logger.With(log.String(keys.EnvironmentID, env.ID))
	router := routerProvider.Get(envLogger, callRecorder)

	services := make([]Service, len(env.Spec.Services))
	for i := range env.Spec.Services {
		service, err := serviceProvider.GetService(&env.Spec.Services[i])
		if err != nil {
			return nil, err
		}
		services[i] = service
	}

	return &Environment{
		cfg:      cfg,
		logger:   envLogger,
		id:       env.ID,
		spec:     env.Spec,
		services: services,
		router:   router,
	}, nil
}

func (e *Environment) Mount(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	for i := range e.services {
		if err := e.services[i].Mount(ctx); err != nil {
			mErr := errorc.With(
				errors.ErrCannotMountEnvironmentService,
				errorc.String(keys.EnvironmentID, e.id),
				errorc.String(keys.EnvironmentServiceID, e.spec.Services[i].ID),
				errorc.Error(keys.Cause, err),
			)

			if i > 0 {
				return nativeerrors.Join(
					mErr,
					e.cleanupServices(i-1),
				)
			}
			return mErr
		}

		route := &Route{
			ServiceID:      e.spec.Services[i].ID,
			Kind:           e.spec.Services[i].Kind,
			ListenAddress:  e.spec.Services[i].Address,
			RuntimeAddress: e.services[i].GetRuntimeAddress(),
		}

		e.router.Register(route)
	}

	if err := e.router.Mount(ctx); err != nil {
		rErr := errorc.With(
			errors.ErrFailedToMountRouter,
			errorc.Error(keys.Cause, err),
		)
		return nativeerrors.Join(rErr, e.cleanupRouterAndServices(len(e.services)-1))
	}

	return nil
}

func (e *Environment) Unmount(ctx context.Context) error {
	routerErr := e.router.Unmount(ctx)
	servicesErr := e.unmountFrom(ctx, len(e.services)-1)
	return nativeerrors.Join(routerErr, servicesErr)
}

func (e *Environment) unmountFrom(
	ctx context.Context,
	start int,
) error {
	var accErr error
	for i := start; i >= 0; i-- {
		if err := e.services[i].Unmount(ctx); err != nil {
			serviceID := e.spec.Services[i].ID
			e.logger.Error(
				"Failed to unmount environment service",
				log.String(keys.EnvironmentServiceID, serviceID),
				log.Err(keys.Cause, err),
			)
			accErr = nativeerrors.Join(
				accErr,
				errorc.With(
					errors.ErrCannotUnmountEnvironmentService,
					errorc.String(keys.EnvironmentID, e.id),
					errorc.String(keys.EnvironmentServiceID, serviceID),
					errorc.Error(keys.Cause, err),
				),
			)
		}
	}
	return accErr
}

func (e *Environment) cleanupServices(start int) error {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		e.cfg.EnvironmentCleanupTimeout,
	)
	defer cancel()

	return e.unmountFrom(ctx, start)
}

func (e *Environment) cleanupRouterAndServices(start int) error {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		e.cfg.EnvironmentCleanupTimeout,
	)
	defer cancel()

	routerErr := e.router.Unmount(ctx)
	servicesErr := e.unmountFrom(ctx, start)

	return nativeerrors.Join(routerErr, servicesErr)
}

type EnvironmentInstance struct {
	ID          string
	Environment *Environment
	Calls       *call.Store
}

func NewEnvironmentController(
	cfg *config.Config,
	logger log.Logger,
	serviceProvider ServiceProvider,
	routerProvider RouterProvider,
	callStoreProvider call.StoreFactory,
) *EnvironmentController {
	return &EnvironmentController{
		cfg:              cfg,
		logger:           logger.With(log.String(keys.Component, "environment_controller")),
		serviceProvider:  serviceProvider,
		routerProvider:   routerProvider,
		callStoreFactory: callStoreProvider,
		instances:        make(map[string]*EnvironmentInstance),
	}
}

type EnvironmentController struct {
	cfg              *config.Config
	logger           log.Logger
	serviceProvider  ServiceProvider
	routerProvider   RouterProvider
	callStoreFactory call.StoreFactory

	mu        sync.Mutex // guards access to the instances map. Coarse, will be redesigned in v2.
	instances map[string]*EnvironmentInstance
}

func (c *EnvironmentController) Acquire(
	ctx context.Context,
	instanceID string, // TestExecutionID for tests, TestSuiteExecutionID for test suites
	env *model.Environment,
) (*EnvironmentInstance, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if instance, ok := c.instances[instanceID]; ok {
		return instance, false, nil
	}

	callStore := c.callStoreFactory.New()

	runtimeEnv, err := NewEnvironment(
		ctx,
		c.cfg,
		c.logger,
		env,
		c.serviceProvider,
		c.routerProvider,
		callStore,
	)
	if err != nil {
		return nil, false, err
	}

	if err := runtimeEnv.Mount(ctx); err != nil {
		return nil, false, err
	}

	instance := &EnvironmentInstance{
		ID:          instanceID,
		Environment: runtimeEnv,
		Calls:       callStore,
	}

	c.instances[instanceID] = instance

	return instance, true, nil
}

func (c *EnvironmentController) Release(
	ctx context.Context,
	instanceID string,
) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	instance, ok := c.instances[instanceID]
	if !ok {
		return nil
	}

	delete(c.instances, instanceID)

	return instance.Environment.Unmount(ctx)
}

func (c *EnvironmentController) ReleaseAll(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var accErr error

	for id, instance := range c.instances {
		if err := instance.Environment.Unmount(ctx); err != nil {
			accErr = nativeerrors.Join(accErr, err)
		}

		delete(c.instances, id)
	}

	return accErr
}
