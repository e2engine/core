package grpc

import (
	"context"
	"encoding/json"
	nativeerrors "errors"
	"net"
	"strings"

	"github.com/ygrebnov/errorc"
	ggrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"

	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/keys"
	"github.com/e2engine/core/pkg/log"
)

type MockGRPCService struct {
	logger log.Logger

	spec     *model.ServiceSpec
	fixtures []Fixture
	resolver MethodResolver

	server         *ggrpc.Server
	listener       net.Listener
	runtimeAddress string
}

type MethodResolver interface {
	ResolveMethod(
		ctx context.Context,
		proto *model.GRPCProtoSpec,
		method string,
	) (protoreflect.MethodDescriptor, error)
}

type Fixture struct {
	rpc     string
	message map[string]any

	status   codes.Code
	response map[string]any
}

func NewMockGRPCService(
	logger log.Logger,
	spec *model.ServiceSpec,
	resolver MethodResolver,
) *MockGRPCService {
	return &MockGRPCService{
		logger: logger.With(
			log.String(keys.EnvironmentServiceID, spec.ID),
			log.String(keys.EnvironmentServiceKind, string(spec.Kind)),
			log.String(keys.EnvironmentServiceMode, string(spec.Mode)),
		),
		spec:     spec,
		fixtures: compileFixtures(spec.Fixtures),
		resolver: resolver,
	}
}

func (s *MockGRPCService) Mount(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	listenerConfig := net.ListenConfig{}
	listener, err := listenerConfig.Listen(
		ctx,
		"tcp",
		"127.0.0.1:0",
	)
	if err != nil {
		return err
	}

	s.listener = listener
	s.runtimeAddress = listener.Addr().String()

	s.server = ggrpc.NewServer(
		ggrpc.UnknownServiceHandler(s.handle),
	)

	go func() {
		err := s.server.Serve(listener)
		if err != nil &&
			!nativeerrors.Is(err, ggrpc.ErrServerStopped) {
			s.logger.Error(
				"gRPC mock service stopped unexpectedly",
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

func (s *MockGRPCService) GetRuntimeAddress() string {
	return s.runtimeAddress
}

func (s *MockGRPCService) Unmount(ctx context.Context) error {
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
		return ctx.Err()

	case <-done:
		return nil
	}
}

func (s *MockGRPCService) handle(
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

	rpc := methodName(fullMethod)

	method, err := s.resolver.ResolveMethod(
		stream.Context(),
		s.spec.Proto,
		rpc,
	)
	if err != nil {
		return status.Error(
			codes.Unimplemented,
			err.Error(),
		)
	}

	if method.IsStreamingClient() || method.IsStreamingServer() {
		return status.Error(
			codes.Unimplemented,
			"streaming RPCs are not supported",
		)
	}

	request := dynamicpb.NewMessage(method.Input())

	if errRecv := stream.RecvMsg(request); errRecv != nil {
		return errRecv
	}

	fixture, err := s.match(fullMethod, request, method)
	if err != nil {
		return status.Error(
			codes.Internal,
			err.Error(),
		)
	}

	if fixture == nil {
		s.logger.Debug(
			"No fixture matched gRPC request",
			log.String(keys.GRPCMethod, fullMethod),
		)

		stream.SetTrailer(
			metadata.Pairs(
				string(keys.E2EngineMockMissMetadata),
				"true",
			),
		)

		return status.Error(
			codes.Unimplemented,
			"no fixture matched",
		)
	}

	if fixture.status != codes.OK {
		return status.Error(
			fixture.status,
			fixture.status.String(),
		)
	}

	response, err := buildMessage(
		method.Output(),
		fixture.response,
	)
	if err != nil {
		return status.Error(
			codes.Internal,
			err.Error(),
		)
	}

	if err := stream.SendMsg(response); err != nil {
		return err
	}

	return nil
}

func methodName(fullMethod string) string {
	index := strings.LastIndex(fullMethod, "/")
	if index == -1 {
		return fullMethod
	}

	return fullMethod[index+1:]
}

func (s *MockGRPCService) match(
	fullMethod string,
	actual proto.Message,
	method protoreflect.MethodDescriptor,
) (*Fixture, error) {
	rpc := methodName(fullMethod)

	for i := range s.fixtures {
		fixture := &s.fixtures[i]

		if fixture.rpc != rpc &&
			fixture.rpc != fullMethod {
			continue
		}

		expected, err := buildMessage(
			method.Input(),
			fixture.message,
		)
		if err != nil {
			return nil, errorc.With(
				errors.ErrCannotBuildGRPCMessage,
				errorc.String(
					keys.GRPCMethod,
					fullMethod,
				),
				errorc.String(
					keys.Operation,
					"build fixture request message",
				),
				errorc.Error(keys.Cause, err),
			)
		}

		if proto.Equal(expected, actual) {
			return fixture, nil
		}
	}

	return nil, nil
}

func buildMessage(
	descriptor protoreflect.MessageDescriptor,
	value map[string]any,
) (*dynamicpb.Message, error) {
	message := dynamicpb.NewMessage(descriptor)

	if len(value) == 0 {
		return message, nil
	}

	data, err := json.Marshal(value)
	if err != nil {
		return nil, errorc.With(
			errors.ErrCannotBuildGRPCMessage,
			errorc.String(
				keys.Operation,
				"marshal gRPC message value",
			),
			errorc.Error(keys.Cause, err),
		)
	}

	if err := protojson.Unmarshal(data, message); err != nil {
		return nil, errorc.With(
			errors.ErrCannotBuildGRPCMessage,
			errorc.String(
				keys.Operation,
				"unmarshal gRPC message",
			),
			errorc.Error(keys.Cause, err),
		)
	}

	return message, nil
}

func compileFixtures(
	specs []model.FixtureSpec,
) []Fixture {
	fixtures := make([]Fixture, 0, len(specs))

	for i := range specs {
		spec := &specs[i]

		if spec.When.GRPC == nil ||
			spec.Then.GRPC == nil {
			continue
		}

		fixtures = append(
			fixtures,
			Fixture{
				rpc:      spec.When.GRPC.Method,
				message:  spec.When.GRPC.Message,
				status:   grpcCode(spec.Then.GRPC.Status),
				response: spec.Then.GRPC.Message,
			},
		)
	}

	return fixtures
}

func grpcCode(value string) codes.Code {
	switch strings.ToLower(value) {
	case "", "ok":
		return codes.OK

	case "cancelled":
		return codes.Canceled

	case "unknown":
		return codes.Unknown

	case "invalid_argument":
		return codes.InvalidArgument

	case "deadline_exceeded":
		return codes.DeadlineExceeded

	case "not_found":
		return codes.NotFound

	case "already_exists":
		return codes.AlreadyExists

	case "permission_denied":
		return codes.PermissionDenied

	case "resource_exhausted":
		return codes.ResourceExhausted

	case "failed_precondition":
		return codes.FailedPrecondition

	case "aborted":
		return codes.Aborted

	case "out_of_range":
		return codes.OutOfRange

	case "unimplemented":
		return codes.Unimplemented

	case "internal":
		return codes.Internal

	case "unavailable":
		return codes.Unavailable

	case "data_loss":
		return codes.DataLoss

	case "unauthenticated":
		return codes.Unauthenticated

	default:
		return codes.Unknown
	}
}
