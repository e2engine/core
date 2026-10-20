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

	"github.com/e2engine/core/execute/match"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/model/matcher"
	"github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/keys"
	"github.com/e2engine/core/pkg/log"
)

type MockGRPCService struct {
	cfg    *MockGRPCServiceConfig
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
	service  string
	method   string
	metadata map[string][]matcher.String
	message  *matcher.GRPCMessage

	status           codes.Code
	responseMetadata map[string][]string
	response         map[string]any
}

func NewMockGRPCService(
	cfg *MockGRPCServiceConfig,
	logger log.Logger,
	spec *model.ServiceSpec,
	resolver MethodResolver,
) *MockGRPCService {
	return &MockGRPCService{
		cfg: cfg,
		logger: logger.With(
			log.String(
				keys.EnvironmentServiceID,
				spec.ID,
			),
			log.String(
				keys.EnvironmentServiceKind,
				string(spec.Kind),
			),
			log.String(
				keys.EnvironmentServiceMode,
				string(spec.Mode),
			),
		),
		spec:     spec,
		fixtures: compileFixtures(spec.Fixtures),
		resolver: resolver,
	}
}

func (s *MockGRPCService) Mount(
	ctx context.Context,
) error {
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
		ggrpc.MaxRecvMsgSize(s.cfg.MaxMessageSize),
		ggrpc.MaxSendMsgSize(s.cfg.MaxMessageSize),
		ggrpc.MaxHeaderListSize(s.cfg.MaxMetadataSize),
		ggrpc.UnknownServiceHandler(s.handle),
	)

	go func() {
		err := s.server.Serve(listener)
		if err != nil &&
			!nativeerrors.Is(
				err,
				ggrpc.ErrServerStopped,
			) {
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

func (s *MockGRPCService) Unmount(
	ctx context.Context,
) error {
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
	fullMethod, ok := ggrpc.MethodFromServerStream(
		stream,
	)
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

	if method.IsStreamingClient() ||
		method.IsStreamingServer() {
		return status.Error(
			codes.Unimplemented,
			"streaming RPCs are not supported",
		)
	}

	request := dynamicpb.NewMessage(
		method.Input(),
	)

	if errRecv := stream.RecvMsg(request); errRecv != nil {
		return errRecv
	}

	incomingMetadata, _ := metadata.FromIncomingContext(
		stream.Context(),
	)

	fixture, err := s.match(
		fullMethod,
		request,
		incomingMetadata,
	)
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
			metadata.Pairs(string(keys.E2EngineMockMissMetadata), "true"),
		)

		return status.Error(
			codes.Unimplemented,
			"no fixture matched",
		)
	}

	if len(fixture.responseMetadata) > 0 {
		responseMetadata := metadata.MD{}

		for key, values := range fixture.responseMetadata {
			responseMetadata[key] = append(
				[]string(nil),
				values...,
			)
		}

		if errSetHeader := stream.SetHeader(responseMetadata); errSetHeader != nil {
			return errSetHeader
		}
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
	index := strings.LastIndex(
		fullMethod,
		"/",
	)
	if index == -1 {
		return fullMethod
	}

	return fullMethod[index+1:]
}

func (s *MockGRPCService) match(
	fullMethod string,
	actual proto.Message,
	actualMetadata metadata.MD,
) (*Fixture, error) {
	service, methodName, _ := strings.Cut(
		strings.TrimPrefix(fullMethod, "/"),
		"/",
	)

	for i := range s.fixtures {
		fixture := &s.fixtures[i]

		if fixture.service != service ||
			fixture.method != methodName {
			continue
		}

		if !areMetadataEqual(
			fixture.metadata,
			actualMetadata,
		) {
			continue
		}

		if fixture.message == nil {
			return fixture, nil
		}

		matched, err := match.IsGRPCMessageMatched(
			actual,
			fixture.message,
		)
		if err != nil {
			return nil, errorc.With(
				errors.ErrCannotBuildGRPCMessage,
				errorc.String(keys.GRPCMethod, fullMethod),
				errorc.String(keys.Operation, "match fixture request message"),
				errorc.Error(keys.Cause, err),
			)
		}

		if matched {
			return fixture, nil
		}
	}

	return nil, nil
}

func areMetadataEqual(
	expected map[string][]matcher.String,
	actual metadata.MD,
) bool {
	for key, expectedValues := range expected {
		actualValues, exists := actual[strings.ToLower(key)]

		if !match.IsStringSliceMatched(
			actualValues,
			exists,
			expectedValues,
		) {
			return false
		}
	}

	return true
}

func buildMessage(
	descriptor protoreflect.MessageDescriptor,
	value map[string]any,
) (*dynamicpb.Message, error) {
	message := dynamicpb.NewMessage(
		descriptor,
	)

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
			errorc.Error(
				keys.Cause,
				err,
			),
		)
	}

	if err := protojson.Unmarshal(
		data,
		message,
	); err != nil {
		return nil, errorc.With(
			errors.ErrCannotBuildGRPCMessage,
			errorc.String(
				keys.Operation,
				"unmarshal gRPC message",
			),
			errorc.Error(
				keys.Cause,
				err,
			),
		)
	}

	return message, nil
}

func compileFixtures(
	specs []model.FixtureSpec,
) []Fixture {
	fixtures := make(
		[]Fixture,
		0,
		len(specs),
	)

	for i := range specs {
		spec := &specs[i]

		fixture := Fixture{
			service:  spec.When.GRPC.Service,
			method:   spec.When.GRPC.Method,
			metadata: spec.When.GRPC.Metadata,

			status: grpcCode(
				spec.Then.GRPC.Status,
			),
			responseMetadata: spec.Then.GRPC.Metadata,
		}

		if spec.When.GRPC.Message != nil {
			fixture.message = spec.When.GRPC.Message
		}

		if spec.Then.GRPC.Message != nil {
			fixture.response = spec.Then.GRPC.Message
		}

		fixtures = append(
			fixtures,
			fixture,
		)
	}

	return fixtures
}

func grpcCode(value string) codes.Code {
	for code := codes.OK; code <= codes.Unauthenticated; code++ {
		if code.String() == value {
			return code
		}
	}

	panic("invalid gRPC status")
}
