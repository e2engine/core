package grpc

import (
	"context"
	nativeerrors "errors"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"

	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/log"
)

func TestMethodName(t *testing.T) {
	tests := []struct {
		name       string
		fullMethod string
		expected   string
	}{
		{
			name:       "full method",
			fullMethod: "/users.v1.UserService/GetUser",
			expected:   "GetUser",
		},
		{
			name:       "method only",
			fullMethod: "GetUser",
			expected:   "GetUser",
		},
		{
			name:       "trailing slash",
			fullMethod: "/users.v1.UserService/",
			expected:   "",
		},
		{
			name:       "empty",
			fullMethod: "",
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := methodName(tt.fullMethod)

			if actual != tt.expected {
				t.Errorf(
					"expected %q, got %q",
					tt.expected,
					actual,
				)
			}
		})
	}
}

func TestGRPCCode(t *testing.T) {
	tests := []struct {
		value    string
		expected codes.Code
	}{
		{
			value:    "",
			expected: codes.OK,
		},
		{
			value:    "ok",
			expected: codes.OK,
		},
		{
			value:    "OK",
			expected: codes.OK,
		},
		{
			value:    "cancelled",
			expected: codes.Canceled,
		},
		{
			value:    "unknown",
			expected: codes.Unknown,
		},
		{
			value:    "invalid_argument",
			expected: codes.InvalidArgument,
		},
		{
			value:    "deadline_exceeded",
			expected: codes.DeadlineExceeded,
		},
		{
			value:    "not_found",
			expected: codes.NotFound,
		},
		{
			value:    "already_exists",
			expected: codes.AlreadyExists,
		},
		{
			value:    "permission_denied",
			expected: codes.PermissionDenied,
		},
		{
			value:    "resource_exhausted",
			expected: codes.ResourceExhausted,
		},
		{
			value:    "failed_precondition",
			expected: codes.FailedPrecondition,
		},
		{
			value:    "aborted",
			expected: codes.Aborted,
		},
		{
			value:    "out_of_range",
			expected: codes.OutOfRange,
		},
		{
			value:    "unimplemented",
			expected: codes.Unimplemented,
		},
		{
			value:    "internal",
			expected: codes.Internal,
		},
		{
			value:    "unavailable",
			expected: codes.Unavailable,
		},
		{
			value:    "data_loss",
			expected: codes.DataLoss,
		},
		{
			value:    "unauthenticated",
			expected: codes.Unauthenticated,
		},
		{
			value:    "invalid",
			expected: codes.Unknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			actual := grpcCode(tt.value)

			if actual != tt.expected {
				t.Errorf(
					"grpcCode(%q): expected %v, got %v",
					tt.value,
					tt.expected,
					actual,
				)
			}
		})
	}
}

func TestCompileFixtures(t *testing.T) {
	specs := []model.FixtureSpec{
		{
			When: model.FixtureWhen{
				GRPC: &model.GRPCFixtureWhen{
					Method: "GetUser",
					Message: map[string]any{
						"id": "42",
					},
				},
			},
			Then: model.FixtureThen{
				GRPC: &model.GRPCFixtureThen{
					Status: "not_found",
					Message: map[string]any{
						"message": "not found",
					},
				},
			},
		},
		{
			When: model.FixtureWhen{},
			Then: model.FixtureThen{
				GRPC: &model.GRPCFixtureThen{
					Status: "ok",
				},
			},
		},
		{
			When: model.FixtureWhen{
				GRPC: &model.GRPCFixtureWhen{
					Method: "Ignored",
				},
			},
			Then: model.FixtureThen{},
		},
	}

	fixtures := compileFixtures(specs)

	if len(fixtures) != 1 {
		t.Fatalf(
			"expected 1 fixture, got %d",
			len(fixtures),
		)
	}

	fixture := fixtures[0]

	if fixture.rpc != "GetUser" {
		t.Errorf(
			"expected rpc %q, got %q",
			"GetUser",
			fixture.rpc,
		)
	}

	if fixture.status != codes.NotFound {
		t.Errorf(
			"expected status %v, got %v",
			codes.NotFound,
			fixture.status,
		)
	}

	if fixture.message["id"] != "42" {
		t.Errorf(
			"expected request id %q, got %v",
			"42",
			fixture.message["id"],
		)
	}

	if fixture.response["message"] != "not found" {
		t.Errorf(
			"expected response message %q, got %v",
			"not found",
			fixture.response["message"],
		)
	}
}

func TestBuildMessage(t *testing.T) {
	descriptor := testMessageDescriptor(t)

	tests := []struct {
		name     string
		value    map[string]any
		expected func(*testing.T, *dynamicpb.Message)
	}{
		{
			name:  "empty",
			value: nil,
			expected: func(
				t *testing.T,
				message *dynamicpb.Message,
			) {
				t.Helper()

				if message.String() != "" {
					t.Errorf(
						"expected empty message, got %v",
						message,
					)
				}
			},
		},
		{
			name: "scalar fields",
			value: map[string]any{
				"id":     "42",
				"active": true,
			},
			expected: func(
				t *testing.T,
				message *dynamicpb.Message,
			) {
				t.Helper()

				id := descriptor.Fields().ByName("id")
				active := descriptor.Fields().ByName("active")

				if actual := message.Get(id).String(); actual != "42" {
					t.Errorf(
						"expected id %q, got %q",
						"42",
						actual,
					)
				}

				if actual := message.Get(active).Bool(); !actual {
					t.Error("expected active to be true")
				}
			},
		},
		{
			name: "repeated field",
			value: map[string]any{
				"tags": []any{
					"one",
					"two",
				},
			},
			expected: func(
				t *testing.T,
				message *dynamicpb.Message,
			) {
				t.Helper()

				tags := descriptor.Fields().ByName("tags")
				list := message.Get(tags).List()

				if list.Len() != 2 {
					t.Fatalf(
						"expected 2 tags, got %d",
						list.Len(),
					)
				}

				if actual := list.Get(0).String(); actual != "one" {
					t.Errorf(
						"expected first tag %q, got %q",
						"one",
						actual,
					)
				}

				if actual := list.Get(1).String(); actual != "two" {
					t.Errorf(
						"expected second tag %q, got %q",
						"two",
						actual,
					)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message, err := buildMessage(
				descriptor,
				tt.value,
			)
			if err != nil {
				t.Fatalf(
					"buildMessage() error = %v",
					err,
				)
			}

			tt.expected(t, message)
		})
	}
}

func TestBuildMessageInvalidField(t *testing.T) {
	descriptor := testMessageDescriptor(t)

	_, err := buildMessage(
		descriptor,
		map[string]any{
			"unknown": "value",
		},
	)
	if err == nil {
		t.Fatal("expected error")
	}

	if !nativeerrors.Is(
		err,
		errors.ErrCannotBuildGRPCMessage,
	) {
		t.Errorf(
			"expected error %v, got %v",
			errors.ErrCannotBuildGRPCMessage,
			err,
		)
	}
}

func TestMockGRPCServiceMatch(t *testing.T) {
	method := testMethodDescriptor(t)

	request, err := buildMessage(
		method.Input(),
		map[string]any{
			"id": "42",
		},
	)
	if err != nil {
		t.Fatalf("cannot build request: %v", err)
	}

	tests := []struct {
		name       string
		fullMethod string
		fixtures   []Fixture
		expected   bool
	}{
		{
			name:       "short rpc",
			fullMethod: "/users.v1.UserService/GetUser",
			fixtures: []Fixture{
				{
					rpc: "GetUser",
					message: map[string]any{
						"id": "42",
					},
				},
			},
			expected: true,
		},
		{
			name:       "full method",
			fullMethod: "/users.v1.UserService/GetUser",
			fixtures: []Fixture{
				{
					rpc: "/users.v1.UserService/GetUser",
					message: map[string]any{
						"id": "42",
					},
				},
			},
			expected: true,
		},
		{
			name:       "different rpc",
			fullMethod: "/users.v1.UserService/GetUser",
			fixtures: []Fixture{
				{
					rpc: "CreateUser",
					message: map[string]any{
						"id": "42",
					},
				},
			},
			expected: false,
		},
		{
			name:       "different message",
			fullMethod: "/users.v1.UserService/GetUser",
			fixtures: []Fixture{
				{
					rpc: "GetUser",
					message: map[string]any{
						"id": "43",
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &MockGRPCService{
				fixtures: tt.fixtures,
			}

			fixture, err := service.match(
				tt.fullMethod,
				request,
				method,
			)
			if err != nil {
				t.Fatalf("match() error = %v", err)
			}

			if tt.expected && fixture == nil {
				t.Fatal("expected fixture")
			}

			if !tt.expected && fixture != nil {
				t.Errorf(
					"expected no fixture, got %+v",
					fixture,
				)
			}
		})
	}
}

func TestMockGRPCServiceMatchInvalidFixtureMessage(
	t *testing.T,
) {
	method := testMethodDescriptor(t)

	request, err := buildMessage(
		method.Input(),
		map[string]any{
			"id": "42",
		},
	)
	if err != nil {
		t.Fatalf("cannot build request: %v", err)
	}

	service := &MockGRPCService{
		fixtures: []Fixture{
			{
				rpc: "GetUser",
				message: map[string]any{
					"unknown": "value",
				},
			},
		},
	}

	_, err = service.match(
		"/users.v1.UserService/GetUser",
		request,
		method,
	)
	if err == nil {
		t.Fatal("expected error")
	}

	if !nativeerrors.Is(
		err,
		errors.ErrCannotBuildGRPCMessage,
	) {
		t.Errorf(
			"expected error %v, got %v",
			errors.ErrCannotBuildGRPCMessage,
			err,
		)
	}
}

func TestMockGRPCServiceMountCanceledContext(t *testing.T) {
	service := &MockGRPCService{}

	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	err := service.Mount(ctx)

	if !nativeerrors.Is(err, context.Canceled) {
		t.Errorf(
			"expected %v, got %v",
			context.Canceled,
			err,
		)
	}

	if service.listener != nil {
		t.Error("expected listener not to be created")
	}

	if service.server != nil {
		t.Error("expected server not to be created")
	}
}

func TestMockGRPCServiceUnmountBeforeMount(t *testing.T) {
	service := &MockGRPCService{}

	if err := service.Unmount(
		context.Background(),
	); err != nil {
		t.Errorf(
			"Unmount() error = %v",
			err,
		)
	}
}

func testMethodDescriptor(
	t *testing.T,
) protoreflect.MethodDescriptor {
	t.Helper()

	files, err := protodesc.NewFiles(
		testFileDescriptorSet(),
	)
	if err != nil {
		t.Fatalf(
			"protodesc.NewFiles() error = %v",
			err,
		)
	}

	descriptor, err := files.FindDescriptorByName(
		"users.v1.UserService",
	)
	if err != nil {
		t.Fatalf(
			"FindDescriptorByName() error = %v",
			err,
		)
	}

	service, ok := descriptor.(protoreflect.ServiceDescriptor)
	if !ok {
		t.Fatalf(
			"expected service descriptor, got %T",
			descriptor,
		)
	}

	method := service.Methods().ByName("GetUser")
	if method == nil {
		t.Fatal("GetUser method not found")
	}

	return method
}

func testMessageDescriptor(
	t *testing.T,
) protoreflect.MessageDescriptor {
	t.Helper()

	return testMethodDescriptor(t).Input()
}

func testFileDescriptorSet() *descriptorpb.FileDescriptorSet {
	optional :=
		descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL
	repeated :=
		descriptorpb.FieldDescriptorProto_LABEL_REPEATED

	stringType :=
		descriptorpb.FieldDescriptorProto_TYPE_STRING
	boolType :=
		descriptorpb.FieldDescriptorProto_TYPE_BOOL

	return &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{
			{
				Name:    proto.String("users.proto"),
				Package: proto.String("users.v1"),
				Syntax:  proto.String("proto3"),
				MessageType: []*descriptorpb.DescriptorProto{
					{
						Name: proto.String(
							"GetUserRequest",
						),
						Field: []*descriptorpb.FieldDescriptorProto{
							{
								Name: proto.String(
									"id",
								),
								Number: proto.Int32(1),
								Label:  &optional,
								Type:   &stringType,
							},
							{
								Name: proto.String(
									"active",
								),
								Number: proto.Int32(2),
								Label:  &optional,
								Type:   &boolType,
							},
							{
								Name: proto.String(
									"tags",
								),
								Number: proto.Int32(3),
								Label:  &repeated,
								Type:   &stringType,
							},
						},
					},
					{
						Name: proto.String(
							"GetUserResponse",
						),
						Field: []*descriptorpb.FieldDescriptorProto{
							{
								Name: proto.String(
									"name",
								),
								Number: proto.Int32(1),
								Label:  &optional,
								Type:   &stringType,
							},
						},
					},
				},
				Service: []*descriptorpb.ServiceDescriptorProto{
					{
						Name: proto.String(
							"UserService",
						),
						Method: []*descriptorpb.MethodDescriptorProto{
							{
								Name: proto.String(
									"GetUser",
								),
								InputType: proto.String(
									".users.v1.GetUserRequest",
								),
								OutputType: proto.String(
									".users.v1.GetUserResponse",
								),
							},
						},
					},
				},
			},
		},
	}
}

type testMethodResolver struct {
	method protoreflect.MethodDescriptor
	err    error
}

func (r *testMethodResolver) ResolveMethod(
	_ context.Context,
	_ *model.GRPCProtoSpec,
	_ string,
) (protoreflect.MethodDescriptor, error) {
	return r.method, r.err
}

func TestMockGRPCServiceHandle(t *testing.T) {
	logger, err := log.NewSilentLogger()
	if err != nil {
		t.Fatalf("Unexpected NewSilentLogger error: %q", err)
	}

	method := testMethodDescriptor(t)

	service := &MockGRPCService{
		logger: logger,
		spec: &model.ServiceSpec{
			Proto: &model.GRPCProtoSpec{},
		},
		fixtures: []Fixture{
			{
				rpc: "GetUser",
				message: map[string]any{
					"id": "42",
				},
				status: codes.OK,
				response: map[string]any{
					"name": "Yaroslav",
				},
			},
		},
		resolver: &testMethodResolver{
			method: method,
		},
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	if err := service.Mount(ctx); err != nil {
		t.Fatalf(
			"Mount() error = %v",
			err,
		)
	}

	defer func() {
		ctx, cancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cancel()

		if err := service.Unmount(ctx); err != nil {
			t.Errorf(
				"Unmount() error = %v",
				err,
			)
		}
	}()

	conn, err := grpc.NewClient(
		service.GetRuntimeAddress(),
		grpc.WithTransportCredentials(
			insecure.NewCredentials(),
		),
	)
	if err != nil {
		t.Fatalf(
			"grpc.NewClient() error = %v",
			err,
		)
	}
	defer conn.Close()

	request, err := buildMessage(
		method.Input(),
		map[string]any{
			"id": "42",
		},
	)
	if err != nil {
		t.Fatalf(
			"cannot build request: %v",
			err,
		)
	}

	response := dynamicpb.NewMessage(
		method.Output(),
	)

	if err := conn.Invoke(
		ctx,
		"/users.v1.UserService/GetUser",
		request,
		response,
	); err != nil {
		t.Fatalf(
			"Invoke() error = %v",
			err,
		)
	}

	name := method.Output().
		Fields().
		ByName("name")

	if name == nil {
		t.Fatal("name field not found")
	}

	if actual := response.Get(name).String(); actual != "Yaroslav" {
		t.Errorf(
			"expected name %q, got %q",
			"Yaroslav",
			actual,
		)
	}
}
