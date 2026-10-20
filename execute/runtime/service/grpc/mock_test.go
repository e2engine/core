package grpc

import (
	"context"
	"encoding/json"
	nativeerrors "errors"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"

	"github.com/e2engine/core/model"
	"github.com/e2engine/core/model/matcher"
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
		{"OK", codes.OK},
		{"Canceled", codes.Canceled},
		{"Unknown", codes.Unknown},
		{"InvalidArgument", codes.InvalidArgument},
		{"DeadlineExceeded", codes.DeadlineExceeded},
		{"NotFound", codes.NotFound},
		{"AlreadyExists", codes.AlreadyExists},
		{"PermissionDenied", codes.PermissionDenied},
		{"ResourceExhausted", codes.ResourceExhausted},
		{"FailedPrecondition", codes.FailedPrecondition},
		{"Aborted", codes.Aborted},
		{"OutOfRange", codes.OutOfRange},
		{"Unimplemented", codes.Unimplemented},
		{"Internal", codes.Internal},
		{"Unavailable", codes.Unavailable},
		{"DataLoss", codes.DataLoss},
		{"Unauthenticated", codes.Unauthenticated},
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

func TestGRPCCodeInvalid(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{
			name:  "empty status",
			value: "",
		},
		{
			name:  "lowercase status",
			value: "ok",
		},
		{
			name:  "snake case status",
			value: "not_found",
		},
		{
			name:  "unknown status",
			value: "invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf(
						"expected grpcCode(%q) to panic",
						tt.value,
					)
				}
			}()

			grpcCode(tt.value)
		})
	}
}

func TestCompileFixtures(t *testing.T) {
	specs := []model.FixtureSpec{
		{
			When: model.FixtureWhen{
				GRPC: &model.GRPCRequestPattern{
					Service: "users.v1.UserService",
					Method:  "GetUser",
					Metadata: map[string][]matcher.String{
						"x-request-id": {
							grpcStringEquals("one"),
							grpcStringEquals("two"),
						},
					},
					Message: grpcMessageMatcher(map[string]any{
						"id": "42",
					}),
				},
			},
			Then: model.FixtureThen{
				GRPC: &model.GRPCResponse{
					Status: "NotFound",
					Metadata: map[string][]string{
						"x-response-id": {
							"three",
							"four",
						},
					},
					Message: map[string]any{
						"message": "not found",
					},
				},
			},
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

	if fixture.service != "users.v1.UserService" {
		t.Errorf(
			"expected service %q, got %q",
			"users.v1.UserService",
			fixture.service,
		)
	}

	if fixture.method != "GetUser" {
		t.Errorf(
			"expected method %q, got %q",
			"GetUser",
			fixture.method,
		)
	}

	if fixture.status != codes.NotFound {
		t.Errorf(
			"expected status %v, got %v",
			codes.NotFound,
			fixture.status,
		)
	}

	if fixture.message == nil {
		t.Fatal("expected request message matcher")
	}

	if fixture.message.Operator != matcher.GRPCMessageMatchOperatorEquals {
		t.Errorf(
			"expected equals operator, got %q",
			fixture.message.Operator,
		)
	}

	if fixture.message.Value != `{"id":"42"}` {
		t.Errorf(
			"expected request message %q, got %q",
			`{"id":"42"}`,
			fixture.message.Value,
		)
	}

	if fixture.response["message"] != "not found" {
		t.Errorf(
			"expected response message %q, got %v",
			"not found",
			fixture.response["message"],
		)
	}

	requestMetadata := fixture.metadata["x-request-id"]

	if len(requestMetadata) != 2 ||
		requestMetadata[0].Value != "one" ||
		requestMetadata[1].Value != "two" {
		t.Errorf(
			"expected request metadata [one two], got %v",
			requestMetadata,
		)
	}

	responseMetadata := fixture.responseMetadata["x-response-id"]

	if len(responseMetadata) != 2 ||
		responseMetadata[0] != "three" ||
		responseMetadata[1] != "four" {
		t.Errorf(
			"expected response metadata [three four], got %v",
			responseMetadata,
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

func TestAreMetadataEqual(t *testing.T) {
	tests := []struct {
		name     string
		expected map[string][]matcher.String
		actual   metadata.MD
		want     bool
	}{
		{
			name: "nil expected",
			actual: metadata.MD{
				"additional": {"value"},
			},
			want: true,
		},
		{
			name: "matching value",
			expected: map[string][]matcher.String{
				"x-request-id": {grpcStringEquals("one")},
			},
			actual: metadata.MD{
				"x-request-id": {"one"},
			},
			want: true,
		},
		{
			name: "matching repeated values",
			expected: map[string][]matcher.String{
				"x-request-id": {
					grpcStringEquals("one"),
					grpcStringEquals("two"),
				},
			},
			actual: metadata.MD{
				"x-request-id": {
					"two",
					"one",
				},
			},
			want: true,
		},
		{
			name: "additional actual key ignored",
			expected: map[string][]matcher.String{
				"x-request-id": {grpcStringEquals("one")},
			},
			actual: metadata.MD{
				"x-request-id": {"one"},
				"additional":   {"value"},
			},
			want: true,
		},
		{
			name: "missing expected key",
			expected: map[string][]matcher.String{
				"x-request-id": {grpcStringEquals("one")},
			},
			actual: metadata.MD{},
		},
		{
			name: "different value",
			expected: map[string][]matcher.String{
				"x-request-id": {grpcStringEquals("one")},
			},
			actual: metadata.MD{
				"x-request-id": {"two"},
			},
		},
		{
			name: "additional value for expected key",
			expected: map[string][]matcher.String{
				"x-request-id": {grpcStringEquals("one")},
			},
			actual: metadata.MD{
				"x-request-id": {
					"one",
					"two",
				},
			},
		},
		{
			name: "metadata key case insensitive",
			expected: map[string][]matcher.String{
				"X-Request-Id": {grpcStringEquals("one")},
			},
			actual: metadata.MD{
				"x-request-id": {"one"},
			},
			want: true,
		},
		{
			name: "exists with value",
			expected: map[string][]matcher.String{
				"x-request-id": {
					{Operator: matcher.StringMatchOperatorExists},
				},
			},
			actual: metadata.MD{
				"x-request-id": {"one"},
			},
			want: true,
		},
		{
			name: "exists with empty value",
			expected: map[string][]matcher.String{
				"x-request-id": {
					{Operator: matcher.StringMatchOperatorExists},
				},
			},
			actual: metadata.MD{
				"x-request-id": {""},
			},
			want: true,
		},
		{
			name: "exists with missing key",
			expected: map[string][]matcher.String{
				"x-request-id": {
					{Operator: matcher.StringMatchOperatorExists},
				},
			},
			actual: metadata.MD{},
		},
		{
			name: "nonempty with value",
			expected: map[string][]matcher.String{
				"x-request-id": {
					{Operator: matcher.StringMatchOperatorNonEmpty},
				},
			},
			actual: metadata.MD{
				"x-request-id": {"one"},
			},
			want: true,
		},
		{
			name: "nonempty with empty value",
			expected: map[string][]matcher.String{
				"x-request-id": {
					{Operator: matcher.StringMatchOperatorNonEmpty},
				},
			},
			actual: metadata.MD{
				"x-request-id": {""},
			},
		},
		{
			name: "nonempty with multiple values",
			expected: map[string][]matcher.String{
				"x-request-id": {
					{Operator: matcher.StringMatchOperatorNonEmpty},
				},
			},
			actual: metadata.MD{
				"x-request-id": {"", "one"},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := areMetadataEqual(
				tt.expected,
				tt.actual,
			)

			if actual != tt.want {
				t.Errorf(
					"expected %v, got %v",
					tt.want,
					actual,
				)
			}
		})
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
		metadata   metadata.MD
		fixtures   []Fixture
		expected   bool
	}{
		{
			name:       "service method and message",
			fullMethod: "/users.v1.UserService/GetUser",
			fixtures: []Fixture{
				{
					service: "users.v1.UserService",
					method:  "GetUser",
					message: grpcMessageMatcher(map[string]any{
						"id": "42",
					}),
				},
			},
			expected: true,
		},
		{
			name:       "metadata",
			fullMethod: "/users.v1.UserService/GetUser",
			metadata: metadata.MD{
				"x-request-id": {"two", "one"},
			},
			fixtures: []Fixture{
				{
					service: "users.v1.UserService",
					method:  "GetUser",
					metadata: map[string][]matcher.String{
						"x-request-id": {
							grpcStringEquals("one"),
							grpcStringEquals("two"),
						},
					},
					message: grpcMessageMatcher(map[string]any{
						"id": "42",
					}),
				},
			},
			expected: true,
		},
		{
			name:       "additional metadata ignored",
			fullMethod: "/users.v1.UserService/GetUser",
			metadata: metadata.MD{
				"x-request-id": {"one"},
				"additional":   {"value"},
			},
			fixtures: []Fixture{
				{
					service: "users.v1.UserService",
					method:  "GetUser",
					metadata: map[string][]matcher.String{
						"x-request-id": {grpcStringEquals("one")},
					},
					message: grpcMessageMatcher(map[string]any{
						"id": "42",
					}),
				},
			},
			expected: true,
		},
		{
			name:       "different service",
			fullMethod: "/users.v1.UserService/GetUser",
			fixtures: []Fixture{
				{
					service: "users.v1.AdminService",
					method:  "GetUser",
					message: grpcMessageMatcher(map[string]any{
						"id": "42",
					}),
				},
			},
		},
		{
			name:       "different method",
			fullMethod: "/users.v1.UserService/GetUser",
			fixtures: []Fixture{
				{
					service: "users.v1.UserService",
					method:  "CreateUser",
					message: grpcMessageMatcher(map[string]any{
						"id": "42",
					}),
				},
			},
		},
		{
			name:       "different message",
			fullMethod: "/users.v1.UserService/GetUser",
			fixtures: []Fixture{
				{
					service: "users.v1.UserService",
					method:  "GetUser",
					message: grpcMessageMatcher(map[string]any{
						"id": "43",
					}),
				},
			},
		},
		{
			name:       "missing metadata",
			fullMethod: "/users.v1.UserService/GetUser",
			fixtures: []Fixture{
				{
					service: "users.v1.UserService",
					method:  "GetUser",
					metadata: map[string][]matcher.String{
						"x-request-id": {grpcStringEquals("one")},
					},
					message: grpcMessageMatcher(map[string]any{
						"id": "42",
					}),
				},
			},
		},
		{
			name:       "different metadata",
			fullMethod: "/users.v1.UserService/GetUser",
			metadata: metadata.MD{
				"x-request-id": {"two"},
			},
			fixtures: []Fixture{
				{
					service: "users.v1.UserService",
					method:  "GetUser",
					metadata: map[string][]matcher.String{
						"x-request-id": {grpcStringEquals("one")},
					},
					message: grpcMessageMatcher(map[string]any{
						"id": "42",
					}),
				},
			},
		},
		{
			name:       "additional value for specified metadata key",
			fullMethod: "/users.v1.UserService/GetUser",
			metadata: metadata.MD{
				"x-request-id": {"one", "two"},
			},
			fixtures: []Fixture{
				{
					service: "users.v1.UserService",
					method:  "GetUser",
					metadata: map[string][]matcher.String{
						"x-request-id": {grpcStringEquals("one")},
					},
					message: grpcMessageMatcher(map[string]any{
						"id": "42",
					}),
				},
			},
		},
		{
			name:       "message expectation omitted",
			fullMethod: "/users.v1.UserService/GetUser",
			fixtures: []Fixture{
				{
					service: "users.v1.UserService",
					method:  "GetUser",
				},
			},
			expected: true,
		},
		{
			name:       "message nonempty matches",
			fullMethod: "/users.v1.UserService/GetUser",
			fixtures: []Fixture{
				{
					service: "users.v1.UserService",
					method:  "GetUser",
					message: &matcher.GRPCMessage{
						Operator: matcher.GRPCMessageMatchOperatorNonEmpty,
					},
				},
			},
			expected: true,
		},
		{
			name:       "explicit empty message does not match",
			fullMethod: "/users.v1.UserService/GetUser",
			fixtures: []Fixture{
				{
					service: "users.v1.UserService",
					method:  "GetUser",
					message: grpcMessageMatcher(map[string]any{}),
				},
			},
		},
		{
			name:       "metadata exists with empty value",
			fullMethod: "/users.v1.UserService/GetUser",
			metadata: metadata.MD{
				"x-request-id": {""},
			},
			fixtures: []Fixture{
				{
					service: "users.v1.UserService",
					method:  "GetUser",
					metadata: map[string][]matcher.String{
						"x-request-id": {
							{Operator: matcher.StringMatchOperatorExists},
						},
					},
				},
			},
			expected: true,
		},
		{
			name:       "metadata nonempty rejects empty value",
			fullMethod: "/users.v1.UserService/GetUser",
			metadata: metadata.MD{
				"x-request-id": {""},
			},
			fixtures: []Fixture{
				{
					service: "users.v1.UserService",
					method:  "GetUser",
					metadata: map[string][]matcher.String{
						"x-request-id": {
							{Operator: matcher.StringMatchOperatorNonEmpty},
						},
					},
				},
			},
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
				tt.metadata,
			)
			if err != nil {
				t.Fatalf(
					"match() error = %v",
					err,
				)
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

func TestMockGRPCServiceMatchEmptyMessage(t *testing.T) {
	method := testMethodDescriptor(t)

	request := dynamicpb.NewMessage(method.Input())

	tests := []struct {
		name     string
		message  *matcher.GRPCMessage
		expected bool
	}{
		{
			name:     "equals empty",
			message:  grpcMessageMatcher(map[string]any{}),
			expected: true,
		},
		{
			name: "nonempty rejects empty message",
			message: &matcher.GRPCMessage{
				Operator: matcher.GRPCMessageMatchOperatorNonEmpty,
			},
		},
		{
			name:     "no message expectation",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &MockGRPCService{
				fixtures: []Fixture{
					{
						service: "users.v1.UserService",
						method:  "GetUser",
						message: tt.message,
					},
				},
			}

			fixture, err := service.match(
				"/users.v1.UserService/GetUser",
				request,
				nil,
			)
			if err != nil {
				t.Fatalf("match() error = %v", err)
			}

			if (fixture != nil) != tt.expected {
				t.Errorf(
					"expected match %v, got %v",
					tt.expected,
					fixture != nil,
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
		t.Fatalf(
			"cannot build request: %v",
			err,
		)
	}

	service := &MockGRPCService{
		fixtures: []Fixture{
			{
				service: "users.v1.UserService",
				method:  "GetUser",
				message: grpcMessageMatcher(map[string]any{
					"unknown": "value",
				}),
			},
		},
	}

	_, err = service.match(
		"/users.v1.UserService/GetUser",
		request,
		nil,
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

func TestMockGRPCServiceMountCanceledContext(
	t *testing.T,
) {
	service := &MockGRPCService{}

	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	err := service.Mount(ctx)

	if !nativeerrors.Is(
		err,
		context.Canceled,
	) {
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

func TestMockGRPCServiceUnmountBeforeMount(
	t *testing.T,
) {
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
	optional := descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL
	repeated := descriptorpb.FieldDescriptorProto_LABEL_REPEATED

	stringType := descriptorpb.FieldDescriptorProto_TYPE_STRING
	boolType := descriptorpb.FieldDescriptorProto_TYPE_BOOL

	return &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{
			{
				Name:    proto.String("users.proto"),
				Package: proto.String("users.v1"),
				Syntax:  proto.String("proto3"),
				MessageType: []*descriptorpb.DescriptorProto{
					{
						Name: proto.String("GetUserRequest"),
						Field: []*descriptorpb.FieldDescriptorProto{
							{
								Name:   proto.String("id"),
								Number: proto.Int32(1),
								Label:  &optional,
								Type:   &stringType,
							},
							{
								Name:   proto.String("active"),
								Number: proto.Int32(2),
								Label:  &optional,
								Type:   &boolType,
							},
							{
								Name:   proto.String("tags"),
								Number: proto.Int32(3),
								Label:  &repeated,
								Type:   &stringType,
							},
						},
					},
					{
						Name: proto.String("GetUserResponse"),
						Field: []*descriptorpb.FieldDescriptorProto{
							{
								Name:   proto.String("name"),
								Number: proto.Int32(1),
								Label:  &optional,
								Type:   &stringType,
							},
						},
					},
				},
				Service: []*descriptorpb.ServiceDescriptorProto{
					{
						Name: proto.String("UserService"),
						Method: []*descriptorpb.MethodDescriptorProto{
							{
								Name: proto.String("GetUser"),
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
		t.Fatalf(
			"Unexpected NewSilentLogger error: %q",
			err,
		)
	}

	method := testMethodDescriptor(t)

	service := &MockGRPCService{
		cfg: &MockGRPCServiceConfig{
			MaxMessageSize:  4 << 20,
			MaxMetadataSize: 64 << 10,
		},
		logger: logger,
		spec: &model.ServiceSpec{
			Proto: &model.GRPCProtoSpec{},
		},
		fixtures: []Fixture{
			{
				service: "users.v1.UserService",
				method:  "GetUser",
				metadata: map[string][]matcher.String{
					"x-request-id": {
						grpcStringEquals("one"),
						grpcStringEquals("two"),
					},
				},
				message: grpcMessageMatcher(map[string]any{
					"id": "42",
				}),
				status: codes.OK,
				responseMetadata: map[string][]string{
					"x-response-id": {
						"three",
						"four",
					},
				},
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

	ctx = metadata.NewOutgoingContext(
		ctx,
		metadata.MD{
			"x-request-id": {
				"one",
				"two",
			},
		},
	)

	response := dynamicpb.NewMessage(
		method.Output(),
	)

	var header metadata.MD

	if err := conn.Invoke(
		ctx,
		"/users.v1.UserService/GetUser",
		request,
		response,
		grpc.Header(&header),
	); err != nil {
		t.Fatalf(
			"Invoke() error = %v",
			err,
		)
	}

	responseMetadata := header.Get("x-response-id")

	if len(responseMetadata) != 2 {
		t.Fatalf(
			"expected 2 response metadata values, got %v",
			responseMetadata,
		)
	}

	if !containsString(responseMetadata, "three") {
		t.Errorf(
			"expected response metadata to contain %q, got %v",
			"three",
			responseMetadata,
		)
	}

	if !containsString(responseMetadata, "four") {
		t.Errorf(
			"expected response metadata to contain %q, got %v",
			"four",
			responseMetadata,
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

func TestMockGRPCServiceMessageSizeLimit(t *testing.T) {
	method := testMethodDescriptor(t)

	tests := []struct {
		name         string
		requestID    string
		responseName string
		limit        int
		wantCode     codes.Code
	}{
		{
			name:      "request exactly at limit",
			requestID: "abc",
			limit:     5,
			wantCode:  codes.OK,
		},
		{
			name:      "request exceeds limit",
			requestID: "abcd",
			limit:     5,
			wantCode:  codes.ResourceExhausted,
		},
		{
			name:         "response exactly at limit",
			responseName: "abc",
			limit:        5,
			wantCode:     codes.OK,
		},
		{
			name:         "response exceeds limit",
			responseName: "abcd",
			limit:        5,
			wantCode:     codes.ResourceExhausted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := log.NewSilentLogger()
			if err != nil {
				t.Fatal(err)
			}

			service := &MockGRPCService{
				cfg: &MockGRPCServiceConfig{
					MaxMessageSize:  tt.limit,
					MaxMetadataSize: 64 << 10,
				},
				logger: logger,
				spec: &model.ServiceSpec{
					Proto: &model.GRPCProtoSpec{},
				},
				fixtures: []Fixture{
					{
						service: "users.v1.UserService",
						method:  "GetUser",
						status:  codes.OK,
						response: map[string]any{
							"name": tt.responseName,
						},
					},
				},
				resolver: &testMethodResolver{
					method: method,
				},
			}

			if err := service.Mount(t.Context()); err != nil {
				t.Fatalf("Mount() error = %v", err)
			}

			defer func() {
				ctx, cancel := context.WithTimeout(
					context.Background(),
					5*time.Second,
				)
				defer cancel()

				if err := service.Unmount(ctx); err != nil {
					t.Errorf("Unmount() error = %v", err)
				}
			}()

			conn, err := grpc.NewClient(
				service.GetRuntimeAddress(),
				grpc.WithTransportCredentials(
					insecure.NewCredentials(),
				),
			)
			if err != nil {
				t.Fatalf("NewClient() error = %v", err)
			}
			defer conn.Close()

			request := dynamicpb.NewMessage(method.Input())

			if tt.requestID != "" {
				request.Set(
					method.Input().Fields().ByName("id"),
					protoreflect.ValueOfString(tt.requestID),
				)
			}

			response := dynamicpb.NewMessage(method.Output())

			err = conn.Invoke(
				t.Context(),
				"/users.v1.UserService/GetUser",
				request,
				response,
			)

			if code := status.Code(err); code != tt.wantCode {
				t.Fatalf(
					"status = %s, want %s (error: %v)",
					code,
					tt.wantCode,
					err,
				)
			}

			if tt.wantCode == codes.OK {
				name := method.Output().Fields().ByName("name")

				if got := response.Get(name).String(); got != tt.responseName {
					t.Errorf(
						"response name = %q, want %q",
						got,
						tt.responseName,
					)
				}
			}
		})
	}
}

func TestMockGRPCServiceMetadataSizeLimit(t *testing.T) {
	method := testMethodDescriptor(t)

	tests := []struct {
		name      string
		metadata  string
		wantError bool
	}{
		{
			name:     "metadata within limit",
			metadata: "small",
		},
		{
			name:      "metadata exceeds limit",
			metadata:  strings.Repeat("a", 4096),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := log.NewSilentLogger()
			if err != nil {
				t.Fatal(err)
			}

			service := &MockGRPCService{
				cfg: &MockGRPCServiceConfig{
					MaxMessageSize:  4 << 20,
					MaxMetadataSize: 1024,
				},
				logger: logger,
				spec: &model.ServiceSpec{
					Proto: &model.GRPCProtoSpec{},
				},
				fixtures: []Fixture{
					{
						service: "users.v1.UserService",
						method:  "GetUser",
						status:  codes.OK,
						response: map[string]any{
							"name": "matched",
						},
					},
				},
				resolver: &testMethodResolver{
					method: method,
				},
			}

			if err := service.Mount(t.Context()); err != nil {
				t.Fatalf("Mount() error = %v", err)
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
				t.Fatalf("NewClient() error = %v", err)
			}
			defer conn.Close()

			ctx, cancel := context.WithTimeout(
				t.Context(),
				5*time.Second,
			)
			defer cancel()

			ctx = metadata.NewOutgoingContext(
				ctx,
				metadata.Pairs(
					"x-test",
					tt.metadata,
				),
			)

			request := dynamicpb.NewMessage(method.Input())
			response := dynamicpb.NewMessage(method.Output())

			err = conn.Invoke(
				ctx,
				"/users.v1.UserService/GetUser",
				request,
				response,
			)

			if tt.wantError {
				if err == nil {
					t.Fatal(
						"Invoke() unexpectedly succeeded with oversized metadata",
					)
				}
				return
			}

			if err != nil {
				t.Fatalf("Invoke() error = %v", err)
			}

			name := method.Output().
				Fields().
				ByName("name")

			if got := response.Get(name).String(); got != "matched" {
				t.Errorf(
					"response name = %q, want %q",
					got,
					"matched",
				)
			}
		})
	}
}

func containsString(
	values []string,
	expected string,
) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}

	return false
}

func grpcMessageMatcher(
	value map[string]any,
) *matcher.GRPCMessage {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}

	return &matcher.GRPCMessage{
		Operator: matcher.GRPCMessageMatchOperatorEquals,
		Value:    string(data),
	}
}

func grpcStringEquals(value string) matcher.String {
	return matcher.String{
		Operator: matcher.StringMatchOperatorEquals,
		Value:    value,
	}
}
