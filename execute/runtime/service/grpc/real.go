package grpc

import (
	"context"
	nativeerrors "errors"
	"net"

	"github.com/ygrebnov/errorc"
	ggrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/keys"
	"github.com/e2engine/core/pkg/log"
)

type RealGRPCService struct {
	logger log.Logger
	spec   *model.ServiceSpec

	server         *ggrpc.Server
	connection     *ggrpc.ClientConn
	runtimeAddress string
}

func NewRealGRPCService(
	logger log.Logger,
	spec *model.ServiceSpec,
) (*RealGRPCService, error) {
	return &RealGRPCService{
		logger: logger.With(
			log.String(keys.EnvironmentServiceID, spec.ID),
			log.String(keys.EnvironmentServiceKind, string(spec.Kind)),
			log.String(keys.EnvironmentServiceMode, string(spec.Mode)),
		),
		spec: spec,
	}, nil
}

func (s *RealGRPCService) Mount(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	connection, err := ggrpc.NewClient(
		s.spec.GRPCTarget,
		ggrpc.WithTransportCredentials(
			insecure.NewCredentials(),
		),
		ggrpc.WithDefaultCallOptions(
			ggrpc.ForceCodec(rawCodec{}),
		),
	)
	if err != nil {
		return err
	}

	listenerConfig := net.ListenConfig{}
	listener, err := listenerConfig.Listen(
		ctx,
		"tcp",
		"127.0.0.1:0",
	)
	if err != nil {
		_ = connection.Close()
		return err
	}

	s.connection = connection
	s.runtimeAddress = listener.Addr().String()

	s.server = ggrpc.NewServer(
		ggrpc.ForceServerCodec(rawCodec{}),
		ggrpc.UnknownServiceHandler(
			s.handle,
		),
	)

	go func() {
		if err := s.server.Serve(listener); err != nil &&
			!nativeerrors.Is(err, ggrpc.ErrServerStopped) {
			s.logger.Error(
				"gRPC real service proxy stopped unexpectedly",
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

func (s *RealGRPCService) GetRuntimeAddress() string {
	return s.runtimeAddress
}

func (s *RealGRPCService) Unmount(ctx context.Context) error {
	if s.server == nil {
		return nil
	}

	done := make(chan struct{})

	go func() {
		s.server.GracefulStop()
		close(done)
	}()

	select {
	case <-ctx.Done():
		s.server.Stop()

		if s.connection != nil {
			_ = s.connection.Close()
		}

		return ctx.Err()

	case <-done:
	}

	if s.connection != nil {
		return s.connection.Close()
	}

	return nil
}

func (s *RealGRPCService) handle(
	_ any,
	stream ggrpc.ServerStream,
) error {
	fullMethod, ok := ggrpc.MethodFromServerStream(stream)
	if !ok {
		return status.Error(
			codes.Internal,
			"cannot determine gRPC method",
		)
	}

	var request rawMessage

	if err := stream.RecvMsg(&request); err != nil {
		return err
	}

	incomingMetadata, _ := metadata.FromIncomingContext(
		stream.Context(),
	)

	outgoingContext := metadata.NewOutgoingContext(
		stream.Context(),
		incomingMetadata.Copy(),
	)

	var response rawMessage
	var responseHeader metadata.MD
	var responseTrailer metadata.MD

	err := s.connection.Invoke(
		outgoingContext,
		fullMethod,
		&request,
		&response,
		ggrpc.Header(&responseHeader),
		ggrpc.Trailer(&responseTrailer),
		ggrpc.ForceCodec(rawCodec{}),
	)
	if err != nil {
		return err
	}

	if len(responseHeader) > 0 {
		if err := stream.SendHeader(responseHeader); err != nil {
			return err
		}
	}

	if len(responseTrailer) > 0 {
		stream.SetTrailer(responseTrailer)
	}

	return stream.SendMsg(&response)
}

type rawMessage []byte

type rawCodec struct{}

var _ encoding.Codec = rawCodec{}

func (rawCodec) Name() string {
	return "proto"
}

func (rawCodec) Marshal(value any) ([]byte, error) {
	switch message := value.(type) {
	case rawMessage:
		return message, nil

	case *rawMessage:
		return *message, nil

	default:
		return nil, errorc.With(
			errors.ErrUnsupportedGRPCMessageType,
			errorc.String(
				keys.Operation,
				"marshal raw gRPC message",
			),
		)
	}
}

func (rawCodec) Unmarshal(
	data []byte,
	value any,
) error {
	message, ok := value.(*rawMessage)
	if !ok {
		return errorc.With(
			errors.ErrUnsupportedGRPCMessageType,
			errorc.String(
				keys.Operation,
				"unmarshal raw gRPC message",
			),
		)
	}

	*message = append(
		(*message)[:0],
		data...,
	)

	return nil
}
