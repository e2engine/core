package grpc

import (
	"context"
	nativeerrors "errors"
	"fmt"
	"net"

	"github.com/ygrebnov/errorc"
	grpcpkg "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

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
		calls:    calls,
		services: make(map[string]*runtime.Route),
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
	route      *runtime.Route
	listener   net.Listener
	server     *grpcpkg.Server
	connection *grpcpkg.ClientConn
}

type rawMessage []byte

type rawCodec struct{}

var _ encoding.Codec = rawCodec{}

func (rawCodec) Name() string {
	return "proto"
}

func (rawCodec) Marshal(v any) ([]byte, error) {
	switch message := v.(type) {
	case rawMessage:
		return message, nil

	case *rawMessage:
		return *message, nil

	default:
		return nil, fmt.Errorf(
			"unsupported raw gRPC message type %T",
			v,
		)
	}
}

func (rawCodec) Unmarshal(
	data []byte,
	v any,
) error {
	message, ok := v.(*rawMessage)
	if !ok {
		return fmt.Errorf(
			"unsupported raw gRPC message type %T",
			v,
		)
	}

	*message = append(
		(*message)[:0],
		data...,
	)

	return nil
}

func (r *Router) Mount(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	mounted := make(
		[]mountedRoute,
		0,
		len(r.services),
	)

	for _, route := range r.services {
		connection, err := grpcpkg.NewClient(
			route.RuntimeAddress,
			grpcpkg.WithTransportCredentials(
				insecure.NewCredentials(),
			),
			grpcpkg.WithDefaultCallOptions(
				grpcpkg.ForceCodec(rawCodec{}),
			),
		)
		if err != nil {
			return nativeerrors.Join(
				err,
				r.unmountMountedRoutes(
					ctx,
					mounted,
				),
			)
		}

		listenerConfig := net.ListenConfig{}
		listener, err := listenerConfig.Listen(
			ctx,
			"tcp",
			route.ListenAddress,
		)
		if err != nil {
			_ = connection.Close()

			return nativeerrors.Join(
				err,
				r.unmountMountedRoutes(
					ctx,
					mounted,
				),
			)
		}

		handler := r.newHandler(
			route,
			connection,
		)

		server := grpcpkg.NewServer(
			grpcpkg.ForceServerCodec(rawCodec{}),
			grpcpkg.UnknownServiceHandler(handler),
		)

		mounted = append(
			mounted,
			mountedRoute{
				route:      route,
				listener:   listener,
				server:     server,
				connection: connection,
			},
		)

		go func() {
			err := server.Serve(listener)
			if err != nil {
				r.logger.Error(
					"Environment gRPC router listener stopped unexpectedly",
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

func (r *Router) newHandler(
	route *runtime.Route,
	connection *grpcpkg.ClientConn,
) grpcpkg.StreamHandler {
	return func(
		_ any,
		stream grpcpkg.ServerStream,
	) error {
		fullMethod, ok :=
			grpcpkg.MethodFromServerStream(stream)

		if !ok {
			return status.Error(
				codes.Internal,
				"cannot determine gRPC method",
			)
		}

		r.logger.Debug(
			"Routing environment gRPC request",
			log.String(
				keys.EnvironmentServiceID,
				route.ServiceID,
			),
			log.String(
				keys.GRPCMethod,
				fullMethod,
			),
		)

		var request rawMessage

		if err := stream.RecvMsg(&request); err != nil {
			captureErr := errorc.With(
				errors.ErrFailedToCaptureEnvironmentServiceCall,
				errorc.String(
					keys.EnvironmentServiceID,
					route.ServiceID,
				),
				errorc.Error(
					keys.Cause,
					err,
				),
			)

			r.logger.Error(
				errors.ErrFailedToCaptureEnvironmentServiceCall.Error(),
				log.String(
					keys.EnvironmentServiceID,
					route.ServiceID,
				),
				log.String(
					keys.GRPCMethod,
					fullMethod,
				),
				log.Err(
					keys.Cause,
					captureErr,
				),
			)

			return status.Error(
				codes.Internal,
				"failed to receive gRPC request",
			)
		}

		incomingMetadata, _ :=
			metadata.FromIncomingContext(
				stream.Context(),
			)

		var testExecutionID string
		if values := incomingMetadata.Get(
			string(keys.E2EngineTestExecutionID),
		); len(values) > 0 {
			testExecutionID = values[0]
		}

		outgoingMetadata := incomingMetadata.Copy()
		outgoingMetadata.Delete(
			string(keys.E2EngineTestExecutionID),
		)

		outgoingContext := metadata.NewOutgoingContext(
			stream.Context(),
			outgoingMetadata,
		)

		var response rawMessage
		var responseHeader metadata.MD
		var responseTrailer metadata.MD

		err := connection.Invoke(
			outgoingContext,
			fullMethod,
			&request,
			&response,
			grpcpkg.Header(&responseHeader),
			grpcpkg.Trailer(&responseTrailer),
			grpcpkg.ForceCodec(rawCodec{}),
		)

		responseStatus := status.Convert(err)

		r.calls.Record(
			call.Call{
				TestExecutionID: testExecutionID,
				ServiceID:       route.ServiceID,
				GRPC: &call.GRPCCall{
					Request: call.GRPCRequest{
						RPC: fullMethod,
						Metadata: cloneMetadata(
							incomingMetadata,
						),
						Message: cloneBytes(
							request,
						),
					},
					Response: call.GRPCResponse{
						Status: responseStatus.Code().String(),
						Metadata: mergeMetadata(
							responseHeader,
							responseTrailer,
						),
						Message: cloneBytes(
							response,
						),
					},
				},
			},
		)

		if len(responseHeader) > 0 {
			if errSend := stream.SendHeader(
				responseHeader,
			); errSend != nil {
				return errSend
			}
		}

		if len(responseTrailer) > 0 {
			stream.SetTrailer(
				responseTrailer,
			)
		}

		if err != nil {
			return err
		}

		return stream.SendMsg(&response)
	}
}

func cloneMetadata(
	source metadata.MD,
) map[string][]string {
	result := make(
		map[string][]string,
		len(source),
	)

	for key, values := range source {
		if key == string(keys.E2EngineTestExecutionID) {
			continue
		}

		result[key] = append(
			[]string(nil),
			values...,
		)
	}

	return result
}

func mergeMetadata(
	sources ...metadata.MD,
) map[string][]string {
	result := make(map[string][]string)

	for _, source := range sources {
		for key, values := range source {
			result[key] = append(
				result[key],
				values...,
			)
		}
	}

	return result
}

func cloneBytes(
	source []byte,
) []byte {
	return append(
		[]byte(nil),
		source...,
	)
}

func (r *Router) Unmount(
	ctx context.Context,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	err := r.unmountMountedRoutes(
		ctx,
		r.mounted,
	)

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

		done := make(chan struct{})

		go func() {
			route.server.GracefulStop()
			close(done)
		}()

		select {
		case <-ctx.Done():
			route.server.Stop()

			accErr = nativeerrors.Join(
				accErr,
				ctx.Err(),
			)

		case <-done:
		}

		if err := route.connection.Close(); err != nil {
			r.logger.Error(
				"Failed to close gRPC router connection",
				log.String(
					keys.EnvironmentServiceID,
					route.route.ServiceID,
				),
				log.Err(
					keys.Cause,
					err,
				),
			)

			accErr = nativeerrors.Join(
				accErr,
				err,
			)
		}
	}

	return accErr
}
