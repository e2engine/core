package grpc

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"reflect"
	"testing"

	e2enginegrpc "github.com/e2engine/instrumentation-go/grpc"
	"go.uber.org/mock/gomock"
	ggrpc "google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"

	"github.com/e2engine/core/execute"
	servicegrpc "github.com/e2engine/core/execute/runtime/service/grpc"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/model/matcher"
)

func TestFindProtoSpec(t *testing.T) {
	service := testServiceSpec()

	tests := []struct {
		name       string
		services   []model.ServiceSpec
		nameToFind string
		wantErr    bool
	}{
		{
			name:       "found",
			services:   []model.ServiceSpec{*service},
			nameToFind: "users.UserService",
		},
		{
			name:       "not found",
			services:   []model.ServiceSpec{*service},
			nameToFind: "users.MissingService",
			wantErr:    true,
		},
		{
			name:       "ambiguous",
			services:   []model.ServiceSpec{*service, *service},
			nameToFind: "users.UserService",
			wantErr:    true,
		},
		{
			name:       "empty services",
			nameToFind: "users.UserService",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := findProtoSpec(
				tt.services,
				tt.nameToFind,
			)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}

				if actual != nil {
					t.Errorf(
						"expected nil proto specification, got %+v",
						actual,
					)
				}

				return
			}

			if err != nil {
				t.Fatalf("findProtoSpec() error = %v", err)
			}

			if !reflect.DeepEqual(actual, service.Proto) {
				t.Errorf(
					"expected proto specification %+v, got %+v",
					service.Proto,
					actual,
				)
			}
		})
	}
}

func TestUnmarshalMap(t *testing.T) {
	descriptor := newTestMessageDescriptor(t)

	tests := []struct {
		name  string
		value map[string]any

		expectedName   string
		expectedCount  int32
		expectedActive bool

		wantErr bool
	}{
		{
			name: "nil",
		},
		{
			name: "valid",
			value: map[string]any{
				"name":   "test",
				"count":  42,
				"active": true,
			},
			expectedName:   "test",
			expectedCount:  42,
			expectedActive: true,
		},
		{
			name: "invalid field type",
			value: map[string]any{
				"count": "not-a-number",
			},
			wantErr: true,
		},
		{
			name: "unknown field",
			value: map[string]any{
				"unknown": "value",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message := dynamicpb.NewMessage(descriptor)

			err := unmarshalMap(
				tt.value,
				message,
			)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}

			if err != nil {
				t.Fatalf("unmarshalMap() error = %v", err)
			}

			if tt.value == nil {
				return
			}

			fields := descriptor.Fields()

			name := message.Get(
				fields.ByName("name"),
			).String()

			if name != tt.expectedName {
				t.Errorf(
					"expected name %q, got %q",
					tt.expectedName,
					name,
				)
			}

			count := int32(
				message.Get(
					fields.ByName("count"),
				).Int(),
			)

			if count != tt.expectedCount {
				t.Errorf(
					"expected count %d, got %d",
					tt.expectedCount,
					count,
				)
			}

			active := message.Get(
				fields.ByName("active"),
			).Bool()

			if active != tt.expectedActive {
				t.Errorf(
					"expected active %v, got %v",
					tt.expectedActive,
					active,
				)
			}
		})
	}
}

func newTestMessageDescriptor(
	t *testing.T,
) protoreflect.MessageDescriptor {
	t.Helper()

	fileDescriptor, err := protodesc.NewFile(
		&descriptorpb.FileDescriptorProto{
			Name:    stringPtr("test.proto"),
			Package: stringPtr("test"),
			Syntax:  stringPtr("proto3"),
			MessageType: []*descriptorpb.DescriptorProto{
				{
					Name: stringPtr("Request"),
					Field: []*descriptorpb.FieldDescriptorProto{
						{
							Name:   stringPtr("name"),
							Number: int32Ptr(1),
							Label: descriptorpb.
								FieldDescriptorProto_LABEL_OPTIONAL.
								Enum(),
							Type: descriptorpb.
								FieldDescriptorProto_TYPE_STRING.
								Enum(),
						},
						{
							Name:   stringPtr("count"),
							Number: int32Ptr(2),
							Label: descriptorpb.
								FieldDescriptorProto_LABEL_OPTIONAL.
								Enum(),
							Type: descriptorpb.
								FieldDescriptorProto_TYPE_INT32.
								Enum(),
						},
						{
							Name:   stringPtr("active"),
							Number: int32Ptr(3),
							Label: descriptorpb.
								FieldDescriptorProto_LABEL_OPTIONAL.
								Enum(),
							Type: descriptorpb.
								FieldDescriptorProto_TYPE_BOOL.
								Enum(),
						},
					},
				},
			},
		},
		nil,
	)
	if err != nil {
		t.Fatalf(
			"cannot create test descriptor: %v",
			err,
		)
	}

	return fileDescriptor.
		Messages().
		ByName("Request")
}

func stringPtr(value string) *string {
	return &value
}

func int32Ptr(value int32) *int32 {
	return &value
}

func TestExecutorExecute(t *testing.T) {
	service := testServiceSpec()

	methodResolver := servicegrpc.NewProtoMethodResolver(nil)

	method, err := methodResolver.ResolveMethod(
		context.Background(),
		service.Proto,
		"Create",
	)
	if err != nil {
		t.Fatalf(
			"ResolveMethod() error = %v",
			err,
		)
	}

	listenerConfig := net.ListenConfig{}
	listener, err := listenerConfig.Listen(
		t.Context(),
		"tcp",
		"127.0.0.1:0",
	)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	server := ggrpc.NewServer(
		ggrpc.UnknownServiceHandler(
			func(
				_ any,
				stream ggrpc.ServerStream,
			) error {
				md, ok := metadata.FromIncomingContext(
					stream.Context(),
				)
				if !ok {
					t.Error("expected request metadata")
				}

				if id := e2enginegrpc.TestExecutionID(
					md,
				); id != "execution-123" {
					t.Errorf(
						"unexpected execution ID metadata: %s",
						id,
					)
				}

				request := dynamicpb.NewMessage(
					method.Input(),
				)

				if err := stream.RecvMsg(request); err != nil {
					return err
				}

				name := request.Get(
					method.Input().
						Fields().
						ByName("name"),
				).String()

				if name != "Yaroslav" {
					t.Errorf(
						"expected request name %q, got %q",
						"Yaroslav",
						name,
					)
				}

				if err := stream.SetHeader(
					metadata.Pairs(
						"x-header",
						"header-value",
					),
				); err != nil {
					return err
				}

				stream.SetTrailer(
					metadata.Pairs(
						"x-trailer",
						"trailer-value",
					),
				)

				response := dynamicpb.NewMessage(
					method.Output(),
				)

				response.Set(
					method.Output().
						Fields().
						ByName("id"),
					protoreflect.ValueOfString(
						"user-123",
					),
				)

				return stream.SendMsg(response)
			},
		),
	)

	go func() {
		_ = server.Serve(listener)
	}()
	defer server.Stop()

	const logicalTarget = "gateway-host:50051"

	resolvedTarget := listener.Addr().String()

	ctrl := gomock.NewController(t)

	reporter := NewMockexecutionReporter(ctrl)
	addressResolver := NewMockaddressResolver(ctrl)

	addressResolver.EXPECT().
		ResolveGRPCTarget(logicalTarget).
		Return(resolvedTarget, nil)

	evaluator := NewEvaluator(
		methodResolver,
		reporter,
	)

	executor := NewTestExecutor(
		methodResolver,
		evaluator,
		reporter,
		addressResolver,
	)

	job := &execute.TestJob{
		ExecutionID: "execution-123",
		Environment: model.Environment{
			Spec: model.EnvironmentSpec{
				Services: []model.ServiceSpec{
					*service,
				},
			},
		},
		Test: model.Test{
			Spec: model.TestSpec{
				Request: model.RequestSpec{
					GRPC: &model.GRPCRequest{
						Target:  logicalTarget,
						Service: "users.UserService",
						Method:  "Create",
						Metadata: map[string][]string{
							"x-request-id": {
								"request-123",
							},
						},
						Message: map[string]any{
							"name": "Yaroslav",
						},
					},
				},
				Expect: model.ExpectSpec{
					GRPC: &model.GRPCResponsePattern{
						Status: "OK",
						Message: mapPtrExecutor(
							map[string]any{
								"id": "user-123",
							},
						),
					},
				},
			},
		},
	}

	reporter.EXPECT().
		AddGRPCRequest(gomock.Any()).
		Do(func(req *model.GRPCRequest) {
			if req.Target != resolvedTarget {
				t.Errorf(
					"expected target %q, got %q",
					resolvedTarget,
					req.Target,
				)
			}

			if req.Service != "users.UserService" {
				t.Errorf(
					"expected service %q, got %q",
					"users.UserService",
					req.Service,
				)
			}

			if req.Method != "Create" {
				t.Errorf(
					"expected method %q, got %q",
					"Create",
					req.Method,
				)
			}

			if !reflect.DeepEqual(
				req.Metadata["x-request-id"],
				[]string{"request-123"},
			) {
				t.Errorf(
					"unexpected request metadata: %#v",
					req.Metadata,
				)
			}

			if id := e2enginegrpc.TestExecutionID(
				metadata.MD(req.Metadata),
			); id != "execution-123" {
				t.Errorf(
					"expected execution ID %q, got %q",
					"execution-123",
					id,
				)
			}

			expectedMessage := map[string]any{
				"name": "Yaroslav",
			}

			if !reflect.DeepEqual(
				req.Message,
				expectedMessage,
			) {
				t.Errorf(
					"expected message %#v, got %#v",
					expectedMessage,
					req.Message,
				)
			}
		})

	reporter.EXPECT().
		AddGRPCResponseStatus("OK")

	reporter.EXPECT().
		AddGRPCResponseMetadata(gomock.Any()).
		Do(func(md map[string][]string) {
			if !reflect.DeepEqual(
				md["x-header"],
				[]string{"header-value"},
			) {
				t.Errorf(
					"unexpected header metadata: %#v",
					md,
				)
			}

			if !reflect.DeepEqual(
				md["x-trailer"],
				[]string{"trailer-value"},
			) {
				t.Errorf(
					"unexpected trailer metadata: %#v",
					md,
				)
			}
		})

	reporter.EXPECT().
		AddGRPCResponseMessage(
			map[string]any{
				"id": "user-123",
			},
		)

	err = executor.Execute(
		context.Background(),
		job,
	)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Hostname resolution must not mutate the persisted Test specification.
	if job.Test.Spec.Request.GRPC.Target != logicalTarget {
		t.Errorf(
			"expected original target %q, got %q",
			logicalTarget,
			job.Test.Spec.Request.GRPC.Target,
		)
	}
}

func TestExecutorExecuteWithTestLevelProto(t *testing.T) {
	service := testServiceSpec()
	methodResolver := servicegrpc.NewProtoMethodResolver(nil)

	method, err := methodResolver.ResolveMethod(
		context.Background(),
		service.Proto,
		"Create",
	)
	if err != nil {
		t.Fatalf("ResolveMethod() error = %v", err)
	}

	listener, err := (&net.ListenConfig{}).Listen(
		t.Context(),
		"tcp",
		"127.0.0.1:0",
	)
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer listener.Close()

	server := ggrpc.NewServer(
		ggrpc.UnknownServiceHandler(
			func(
				_ any,
				stream ggrpc.ServerStream,
			) error {
				request := dynamicpb.NewMessage(method.Input())

				if err := stream.RecvMsg(request); err != nil {
					return err
				}

				name := request.Get(
					method.Input().Fields().ByName("name"),
				).String()

				if name != "Yaroslav" {
					t.Errorf(
						"expected request name %q, got %q",
						"Yaroslav",
						name,
					)
				}

				response := dynamicpb.NewMessage(method.Output())
				response.Set(
					method.Output().Fields().ByName("id"),
					protoreflect.ValueOfString("user-123"),
				)

				return stream.SendMsg(response)
			},
		),
	)

	go func() {
		_ = server.Serve(listener)
	}()
	defer server.Stop()

	const logicalTarget = "gateway-host:50051"
	resolvedTarget := listener.Addr().String()

	ctrl := gomock.NewController(t)

	reporter := NewMockexecutionReporter(ctrl)
	addressResolver := NewMockaddressResolver(ctrl)

	addressResolver.EXPECT().
		ResolveGRPCTarget(logicalTarget).
		Return(resolvedTarget, nil)

	reporter.EXPECT().
		AddGRPCRequest(gomock.Any()).
		Do(func(request *model.GRPCRequest) {
			if request.Target != resolvedTarget {
				t.Errorf(
					"expected resolved target %q, got %q",
					resolvedTarget,
					request.Target,
				)
			}
		})

	reporter.EXPECT().
		AddGRPCResponseStatus("OK")

	reporter.EXPECT().
		AddGRPCResponseMetadata(gomock.Any()).
		AnyTimes()

	reporter.EXPECT().
		AddGRPCResponseMessage(gomock.Any()).
		AnyTimes()

	executor := NewTestExecutor(
		methodResolver,
		NewEvaluator(methodResolver, reporter),
		reporter,
		addressResolver,
	)

	job := &execute.TestJob{
		ExecutionID: "execution-123",

		// No matching gRPC service or protobuf specification.
		Environment: model.Environment{},

		Test: model.Test{
			Spec: model.TestSpec{
				Request: model.RequestSpec{
					GRPC: &model.GRPCRequest{
						Target:  logicalTarget,
						Service: "users.UserService",
						Method:  "Create",

						// Protobuf specification is provided by the Test.
						Proto: service.Proto,

						Message: map[string]any{
							"name": "Yaroslav",
						},
					},
				},
				Expect: model.ExpectSpec{
					GRPC: &model.GRPCResponsePattern{
						Status: "OK",
						Message: mapPtrExecutor(map[string]any{
							"id": "user-123",
						}),
					},
				},
			},
		},
	}

	if err := executor.Execute(t.Context(), job); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if job.Test.Spec.Request.GRPC.Target != logicalTarget {
		t.Errorf(
			"expected original target %q, got %q",
			logicalTarget,
			job.Test.Spec.Request.GRPC.Target,
		)
	}
}

func TestExecutorExecuteAddressResolutionError(t *testing.T) {
	service := testServiceSpec()

	ctrl := gomock.NewController(t)

	reporter := NewMockexecutionReporter(ctrl)
	addressResolver := NewMockaddressResolver(ctrl)

	methodResolver := servicegrpc.NewProtoMethodResolver(nil)

	evaluator := NewEvaluator(
		methodResolver,
		reporter,
	)

	executor := NewTestExecutor(
		methodResolver,
		evaluator,
		reporter,
		addressResolver,
	)

	const logicalTarget = "gateway-host:50051"

	resolveErr := errors.New("cannot resolve gRPC target")

	addressResolver.EXPECT().
		ResolveGRPCTarget(logicalTarget).
		Return("", resolveErr)

	job := &execute.TestJob{
		ExecutionID: "execution-123",
		Environment: model.Environment{
			Spec: model.EnvironmentSpec{
				Services: []model.ServiceSpec{
					*service,
				},
			},
		},
		Test: model.Test{
			Spec: model.TestSpec{
				Request: model.RequestSpec{
					GRPC: &model.GRPCRequest{
						Target:  logicalTarget,
						Service: "users.UserService",
						Method:  "Create",
						Message: map[string]any{
							"name": "Yaroslav",
						},
					},
				},
			},
		},
	}

	err := executor.Execute(
		context.Background(),
		job,
	)

	if !errors.Is(err, resolveErr) {
		t.Errorf(
			"expected error %v, got %v",
			resolveErr,
			err,
		)
	}
}

func mapPtrExecutor(
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
