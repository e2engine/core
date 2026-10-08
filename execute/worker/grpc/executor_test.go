package grpc

import (
	"context"
	"errors"
	"net"
	"reflect"
	"testing"

	e2enginegrpc "github.com/e2engine/instrumentation-go/grpc"
	ggrpc "google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"

	"github.com/e2engine/core/execute"
	servicegrpc "github.com/e2engine/core/execute/runtime/service/grpc"
	"github.com/e2engine/core/model"
	coreerrors "github.com/e2engine/core/pkg/errors"
)

func TestFindService(t *testing.T) {
	services := []model.ServiceSpec{
		{
			ID:      "service-1",
			Address: "localhost:50051",
		},
		{
			ID:      "service-2",
			Address: "localhost:50052",
		},
	}

	tests := []struct {
		name     string
		target   string
		expected *model.ServiceSpec
		err      error
	}{
		{
			name:     "found",
			target:   "localhost:50052",
			expected: &services[1],
		},
		{
			name:   "not found",
			target: "localhost:50053",
			err:    coreerrors.ErrGRPCTargetServiceNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := findService(
				services,
				tt.target,
			)

			if tt.err != nil {
				if !errors.Is(err, tt.err) {
					t.Fatalf(
						"expected error %v, got %v",
						tt.err,
						err,
					)
				}

				if actual != nil {
					t.Errorf(
						"expected nil service, got %+v",
						actual,
					)
				}

				return
			}

			if err != nil {
				t.Fatalf(
					"findService() error = %v",
					err,
				)
			}

			if actual != tt.expected {
				t.Errorf(
					"expected service %+v, got %+v",
					tt.expected,
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
			message := dynamicpb.NewMessage(
				descriptor,
			)

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
				t.Fatalf(
					"unmarshalMap() error = %v",
					err,
				)
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

	resolver := servicegrpc.NewProtoMethodResolver(nil)

	method, err := resolver.ResolveMethod(
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
		t.Fatalf(
			"listen: %v",
			err,
		)
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

				if id := e2enginegrpc.TestExecutionID(md); id != "execution-123" {
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

	service.Address = listener.Addr().String()

	executor := NewTestExecutor(
		resolver,
		NewEvaluator(resolver),
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
					GRPC: &model.GRPCRequestSpec{
						Target:  service.Address,
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
					GRPC: &model.GRPCExpectSpec{
						Status: "OK",
						Message: map[string]any{
							"id": "user-123",
						},
					},
				},
			},
		},
	}

	result := &execute.TestExecutionResult{
		Summary: &model.TestExecutionSummary{},
	}

	err = executor.Execute(
		context.Background(),
		job,
		result,
	)
	if err != nil {
		t.Fatalf(
			"Execute() error = %v",
			err,
		)
	}

	if result.Summary.Response.GRPC == nil {
		t.Fatal("expected gRPC response summary")
	}

	response := result.Summary.Response.GRPC

	if response.Status != "OK" {
		t.Errorf(
			"expected status %q, got %q",
			"OK",
			response.Status,
		)
	}

	expectedMessage := map[string]any{
		"id": "user-123",
	}

	if !reflect.DeepEqual(
		response.Message,
		expectedMessage,
	) {
		t.Errorf(
			"expected message %#v, got %#v",
			expectedMessage,
			response.Message,
		)
	}

	expectedMetadata := map[string][]string{
		"content-type": {
			"application/grpc",
		},
		"x-header": {
			"header-value",
		},
		"x-trailer": {
			"trailer-value",
		},
	}

	if !reflect.DeepEqual(
		response.Metadata,
		expectedMetadata,
	) {
		t.Errorf(
			"expected metadata %#v, got %#v",
			expectedMetadata,
			response.Metadata,
		)
	}

	if len(result.Summary.Deviations) != 0 {
		t.Errorf(
			"expected no deviations, got %+v",
			result.Summary.Deviations,
		)
	}
}
