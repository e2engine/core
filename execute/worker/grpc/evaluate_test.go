package grpc

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"

	"github.com/e2engine/core/execute"
	"github.com/e2engine/core/execute/call"
	servicegrpc "github.com/e2engine/core/execute/runtime/service/grpc"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/keys"
)

func TestEvaluatorEvaluateResponse(t *testing.T) {
	tests := []struct {
		name string

		callErr error
		header  metadata.MD
		trailer metadata.MD
		message map[string]any
		expect  model.GRPCExpectSpec

		expectedStatus     string
		expectedMetadata   map[string][]string
		expectedMessage    map[string]any
		expectedDeviations []model.TestExecutionDeviation
	}{
		{
			name: "matching response",
			header: metadata.MD{
				"x-header": {"header-value"},
			},
			trailer: metadata.MD{
				"x-trailer": {"trailer-value"},
			},
			message: map[string]any{
				"name": "actual",
			},
			expect: model.GRPCExpectSpec{
				Status: "OK",
				Message: map[string]any{
					"name": "actual",
				},
			},
			expectedStatus: "OK",
			expectedMetadata: map[string][]string{
				"x-header":  {"header-value"},
				"x-trailer": {"trailer-value"},
			},
			expectedMessage: map[string]any{
				"name": "actual",
			},
		},
		{
			name: "status mismatch",
			callErr: status.Error(
				codes.NotFound,
				"not found",
			),
			message: map[string]any{
				"name": "actual",
			},
			expect: model.GRPCExpectSpec{
				Status: "OK",
			},
			expectedStatus:   "NotFound",
			expectedMetadata: map[string][]string{},
			expectedMessage: map[string]any{
				"name": "actual",
			},
			expectedDeviations: []model.TestExecutionDeviation{
				{
					Field:    "status",
					Expected: "OK",
					Actual:   "NotFound",
				},
			},
		},
		{
			name: "message mismatch",
			message: map[string]any{
				"name": "actual",
			},
			expect: model.GRPCExpectSpec{
				Status: "OK",
				Message: map[string]any{
					"name": "expected",
				},
			},
			expectedStatus:   "OK",
			expectedMetadata: map[string][]string{},
			expectedMessage: map[string]any{
				"name": "actual",
			},
			expectedDeviations: []model.TestExecutionDeviation{
				{
					Field:    "message",
					Expected: `{"name":"expected"}`,
					Actual:   `{"name":"actual"}`,
				},
			},
		},
		{
			name: "no message expectation",
			message: map[string]any{
				"name": "actual",
			},
			expect: model.GRPCExpectSpec{
				Status: "OK",
			},
			expectedStatus:   "OK",
			expectedMetadata: map[string][]string{},
			expectedMessage: map[string]any{
				"name": "actual",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := newTestMessage(
				t,
				tt.message,
			)

			result := &execute.TestExecutionResult{
				Summary: &model.TestExecutionSummary{},
			}

			evaluator := newTestEvaluator()

			err := evaluator.EvaluateResponse(
				context.Background(),
				tt.callErr,
				response,
				tt.header,
				tt.trailer,
				&tt.expect,
				result,
			)
			if err != nil {
				t.Fatalf(
					"EvaluateResponse() error = %v",
					err,
				)
			}

			actual := result.Summary.Response.GRPC
			if actual == nil {
				t.Fatal("expected gRPC response summary")
			}

			if actual.Status != tt.expectedStatus {
				t.Errorf(
					"expected status %q, got %q",
					tt.expectedStatus,
					actual.Status,
				)
			}

			if !reflect.DeepEqual(
				actual.Metadata,
				tt.expectedMetadata,
			) {
				t.Errorf(
					"expected metadata %#v, got %#v",
					tt.expectedMetadata,
					actual.Metadata,
				)
			}

			if !reflect.DeepEqual(
				actual.Message,
				tt.expectedMessage,
			) {
				t.Errorf(
					"expected message %#v, got %#v",
					tt.expectedMessage,
					actual.Message,
				)
			}

			assertGRPCDeviations(
				t,
				result.Summary.Deviations,
				tt.expectedDeviations,
			)
		})
	}
}

func TestEvaluatorEvaluateResponseCancelledContext(
	t *testing.T,
) {
	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	result := &execute.TestExecutionResult{
		Summary: &model.TestExecutionSummary{},
	}

	evaluator := newTestEvaluator()

	err := evaluator.EvaluateResponse(
		ctx,
		nil,
		newTestMessage(
			t,
			map[string]any{},
		),
		nil,
		nil,
		&model.GRPCExpectSpec{
			Status: "OK",
		},
		result,
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"expected context.Canceled, got %v",
			err,
		)
	}
}

func TestEvaluatorEvaluateResponseUnknownStatus(
	t *testing.T,
) {
	result := &execute.TestExecutionResult{
		Summary: &model.TestExecutionSummary{},
	}

	evaluator := newTestEvaluator()

	err := evaluator.EvaluateResponse(
		context.Background(),
		nil,
		newTestMessage(
			t,
			map[string]any{},
		),
		nil,
		nil,
		&model.GRPCExpectSpec{
			Status: "invalid",
		},
		result,
	)

	if !errors.Is(err, ErrUnknownStatus) {
		t.Fatalf(
			"expected ErrUnknownStatus, got %v",
			err,
		)
	}
}

func TestEvaluatorMatches(t *testing.T) {
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

	message := dynamicpb.NewMessage(
		method.Input(),
	)

	message.Set(
		method.Input().Fields().ByName("name"),
		protoreflect.ValueOfString("Yaroslav"),
	)

	messageBytes, err := proto.Marshal(message)
	if err != nil {
		t.Fatalf(
			"marshal message: %v",
			err,
		)
	}

	evaluator := NewEvaluator(resolver)

	actual := call.Call{
		ServiceID: "service-1",
		GRPC: &call.GRPCCall{
			Request: call.GRPCRequest{
				RPC: "/users.UserService/Create",
				Metadata: map[string][]string{
					"x-request-id": {"request-1"},
				},
				Message: messageBytes,
			},
		},
	}

	tests := []struct {
		name string

		actual   call.Call
		expected model.CallExpectation

		match bool
	}{
		{
			name:   "matches",
			actual: actual,
			expected: model.CallExpectation{
				ServiceID: "service-1",
				GRPC: &model.GRPCCallExpectation{
					Service: "users.UserService",
					Method:  "Create",
					Metadata: map[string][]string{
						"x-request-id": {"request-1"},
					},
					Message: map[string]any{
						"name": "Yaroslav",
					},
				},
			},
			match: true,
		},
		{
			name:   "different service",
			actual: actual,
			expected: model.CallExpectation{
				ServiceID: "service-2",
				GRPC: &model.GRPCCallExpectation{
					Service: "users.UserService",
					Method:  "Create",
				},
			},
		},
		{
			name: "actual is not gRPC",
			actual: call.Call{
				ServiceID: "service-1",
			},
			expected: model.CallExpectation{
				ServiceID: "service-1",
				GRPC: &model.GRPCCallExpectation{
					Service: "users.UserService",
					Method:  "Create",
				},
			},
		},
		{
			name:   "expectation is not gRPC",
			actual: actual,
			expected: model.CallExpectation{
				ServiceID: "service-1",
			},
		},
		{
			name:   "RPC mismatch",
			actual: actual,
			expected: model.CallExpectation{
				ServiceID: "service-1",
				GRPC: &model.GRPCCallExpectation{
					Service: "users.UserService",
					Method:  "Get",
				},
			},
		},
		{
			name:   "metadata missing",
			actual: actual,
			expected: model.CallExpectation{
				ServiceID: "service-1",
				GRPC: &model.GRPCCallExpectation{
					Service: "users.UserService",
					Method:  "Create",
					Metadata: map[string][]string{
						"x-missing": {"value"},
					},
				},
			},
		},
		{
			name:   "metadata mismatch",
			actual: actual,
			expected: model.CallExpectation{
				ServiceID: "service-1",
				GRPC: &model.GRPCCallExpectation{
					Service: "users.UserService",
					Method:  "Create",
					Metadata: map[string][]string{
						"x-request-id": {"different"},
					},
				},
			},
		},
		{
			name:   "message mismatch",
			actual: actual,
			expected: model.CallExpectation{
				ServiceID: "service-1",
				GRPC: &model.GRPCCallExpectation{
					Service: "users.UserService",
					Method:  "Create",
					Message: map[string]any{
						"name": "Other",
					},
				},
			},
		},
		{
			name:   "no message expectation",
			actual: actual,
			expected: model.CallExpectation{
				ServiceID: "service-1",
				GRPC: &model.GRPCCallExpectation{
					Service: "users.UserService",
					Method:  "Create",
				},
			},
			match: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match, err := evaluator.Matches(
				context.Background(),
				service,
				tt.actual,
				tt.expected,
			)
			if err != nil {
				t.Fatalf(
					"Matches() error = %v",
					err,
				)
			}

			if match != tt.match {
				t.Errorf(
					"expected match %v, got %v",
					tt.match,
					match,
				)
			}
		})
	}
}

func TestEvaluatorMatchesMissingMethod(
	t *testing.T,
) {
	service := testServiceSpec()
	evaluator := newTestEvaluator()

	actual := call.Call{
		ServiceID: "service-1",
		GRPC: &call.GRPCCall{
			Request: call.GRPCRequest{
				RPC: "/users.UserService/Missing",
			},
		},
	}

	expected := model.CallExpectation{
		ServiceID: "service-1",
		GRPC: &model.GRPCCallExpectation{
			Service: "users.UserService",
			Method:  "Missing",
			Message: map[string]any{
				"name": "Yaroslav",
			},
		},
	}

	match, err := evaluator.Matches(
		context.Background(),
		service,
		actual,
		expected,
	)

	if err == nil {
		t.Fatal("expected error")
	}

	if match {
		t.Fatal("expected no match")
	}
}

func TestEvaluatorMatchesInvalidCapturedMessage(
	t *testing.T,
) {
	service := testServiceSpec()
	evaluator := newTestEvaluator()

	actual := call.Call{
		ServiceID: "service-1",
		GRPC: &call.GRPCCall{
			Request: call.GRPCRequest{
				RPC:     "/users.UserService/Create",
				Message: []byte{0xff},
			},
		},
	}

	expected := model.CallExpectation{
		ServiceID: "service-1",
		GRPC: &model.GRPCCallExpectation{
			Service: "users.UserService",
			Method:  "Create",
			Message: map[string]any{
				"name": "Yaroslav",
			},
		},
	}

	_, err := evaluator.Matches(
		context.Background(),
		service,
		actual,
		expected,
	)

	if !errors.Is(
		err,
		ErrCannotDecodeCapturedRequest,
	) {
		t.Fatalf(
			"expected ErrCannotDecodeCapturedRequest, got %v",
			err,
		)
	}
}

func TestEvaluatorDeviation(t *testing.T) {
	evaluator := newTestEvaluator()

	service := testServiceSpec()

	tests := []struct {
		name string

		actual   call.Call
		expected *model.TestExecutionDeviation
	}{
		{
			name: "not gRPC",
			actual: call.Call{
				ServiceID: "service-1",
			},
		},
		{
			name: "no mock miss",
			actual: call.Call{
				ServiceID: "service-1",
				GRPC: &call.GRPCCall{
					Request: call.GRPCRequest{
						RPC: "/users.UserService/Create",
					},
					Response: call.GRPCResponse{
						Metadata: map[string][]string{},
					},
				},
			},
		},
		{
			name: "mock miss false",
			actual: call.Call{
				ServiceID: "service-1",
				GRPC: &call.GRPCCall{
					Request: call.GRPCRequest{
						RPC: "/users.UserService/Create",
					},
					Response: call.GRPCResponse{
						Metadata: map[string][]string{
							string(
								keys.E2EngineMockMissMetadata,
							): {
								"false",
							},
						},
					},
				},
			},
		},
		{
			name: "mock miss",
			actual: call.Call{
				ServiceID: "service-1",
				GRPC: &call.GRPCCall{
					Request: call.GRPCRequest{
						RPC: "/users.UserService/Create",
					},
					Response: call.GRPCResponse{
						Metadata: map[string][]string{
							string(
								keys.E2EngineMockMissMetadata,
							): {
								"true",
							},
						},
					},
				},
			},
			expected: &model.TestExecutionDeviation{
				Field:  "calls.service-1",
				Actual: "/users.UserService/Create",
				Message: "mocked service received a request " +
					"that did not match any fixture",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := evaluator.Deviation(
				service,
				tt.actual,
			)

			if tt.expected == nil {
				if actual != nil {
					t.Fatalf(
						"expected no deviation, got %+v",
						actual,
					)
				}

				return
			}

			if actual == nil {
				t.Fatal("expected deviation")
			}

			if *actual != *tt.expected {
				t.Errorf(
					"expected deviation %+v, got %+v",
					*tt.expected,
					*actual,
				)
			}
		})
	}
}

func TestMessageToMap(t *testing.T) {
	message := newTestMessage(
		t,
		map[string]any{
			"name": "Yaroslav",
		},
	)

	actual, err := messageToMap(message)
	if err != nil {
		t.Fatalf(
			"messageToMap() error = %v",
			err,
		)
	}

	expected := map[string]any{
		"name": "Yaroslav",
	}

	if !reflect.DeepEqual(
		actual,
		expected,
	) {
		t.Errorf(
			"expected %#v, got %#v",
			expected,
			actual,
		)
	}
}

func TestMergeMetadata(t *testing.T) {
	header := metadata.MD{
		"x-header": {"one"},
		"x-shared": {"header"},
	}

	trailer := metadata.MD{
		"x-trailer": {"two"},
		"x-shared":  {"trailer"},
	}

	actual := mergeMetadata(
		header,
		trailer,
	)

	expected := map[string][]string{
		"x-header":  {"one"},
		"x-trailer": {"two"},
		"x-shared":  {"header", "trailer"},
	}

	if !reflect.DeepEqual(
		actual,
		expected,
	) {
		t.Errorf(
			"expected %#v, got %#v",
			expected,
			actual,
		)
	}

	header["x-header"][0] = "changed"

	if actual["x-header"][0] != "one" {
		t.Error(
			"expected merged metadata to be independent of header",
		)
	}
}

func TestGRPCCode(t *testing.T) {
	tests := []struct {
		name string

		value    string
		expected codes.Code
	}{
		{
			name:     "OK",
			value:    "OK",
			expected: codes.OK,
		},
		{
			name:     "Canceled",
			value:    "Canceled",
			expected: codes.Canceled,
		},
		{
			name:     "Unknown",
			value:    "Unknown",
			expected: codes.Unknown,
		},
		{
			name:     "InvalidArgument",
			value:    "InvalidArgument",
			expected: codes.InvalidArgument,
		},
		{
			name:     "DeadlineExceeded",
			value:    "DeadlineExceeded",
			expected: codes.DeadlineExceeded,
		},
		{
			name:     "NotFound",
			value:    "NotFound",
			expected: codes.NotFound,
		},
		{
			name:     "AlreadyExists",
			value:    "AlreadyExists",
			expected: codes.AlreadyExists,
		},
		{
			name:     "PermissionDenied",
			value:    "PermissionDenied",
			expected: codes.PermissionDenied,
		},
		{
			name:     "ResourceExhausted",
			value:    "ResourceExhausted",
			expected: codes.ResourceExhausted,
		},
		{
			name:     "FailedPrecondition",
			value:    "FailedPrecondition",
			expected: codes.FailedPrecondition,
		},
		{
			name:     "Aborted",
			value:    "Aborted",
			expected: codes.Aborted,
		},
		{
			name:     "OutOfRange",
			value:    "OutOfRange",
			expected: codes.OutOfRange,
		},
		{
			name:     "Unimplemented",
			value:    "Unimplemented",
			expected: codes.Unimplemented,
		},
		{
			name:     "Internal",
			value:    "Internal",
			expected: codes.Internal,
		},
		{
			name:     "Unavailable",
			value:    "Unavailable",
			expected: codes.Unavailable,
		},
		{
			name:     "DataLoss",
			value:    "DataLoss",
			expected: codes.DataLoss,
		},
		{
			name:     "Unauthenticated",
			value:    "Unauthenticated",
			expected: codes.Unauthenticated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := grpcCode(
				tt.value,
			)
			if err != nil {
				t.Fatalf(
					"grpcCode() error = %v",
					err,
				)
			}

			if actual != tt.expected {
				t.Errorf(
					"expected %s, got %s",
					tt.expected,
					actual,
				)
			}
		})
	}
}

func TestGRPCCodeUnknown(t *testing.T) {
	actual, err := grpcCode("invalid")

	if !errors.Is(
		err,
		ErrUnknownStatus,
	) {
		t.Fatalf(
			"expected ErrUnknownStatus, got %v",
			err,
		)
	}

	if actual != codes.Unknown {
		t.Errorf(
			"expected Unknown, got %s",
			actual,
		)
	}
}

func newTestEvaluator() *Evaluator {
	return NewEvaluator(
		servicegrpc.NewProtoMethodResolver(nil),
	)
}

func testServiceSpec() *model.ServiceSpec {
	return &model.ServiceSpec{
		ID: "service-1",
		Proto: &model.GRPCProtoSpec{
			Internal: &model.GRPCInternalProtoSpec{
				Package: "users",
				Service: "UserService",
				Methods: []model.GRPCMethodSpec{
					{
						Name: "Create",
						Request: model.GRPCMessageSpec{
							Fields: []model.GRPCFieldSpec{
								{
									Name: "name",
									Type: model.GRPCFieldTypeString,
								},
							},
						},
						Response: model.GRPCMessageSpec{
							Fields: []model.GRPCFieldSpec{
								{
									Name: "id",
									Type: model.GRPCFieldTypeString,
								},
							},
						},
					},
				},
			},
		},
	}
}

func newTestMessage(
	t *testing.T,
	value map[string]any,
) *dynamicpb.Message {
	t.Helper()

	fileDescriptor, err := protodesc.NewFile(
		&descriptorpb.FileDescriptorProto{
			Syntax:  proto.String("proto3"),
			Name:    proto.String("test.proto"),
			Package: proto.String("test"),
			MessageType: []*descriptorpb.DescriptorProto{
				{
					Name: proto.String("Response"),
					Field: []*descriptorpb.FieldDescriptorProto{
						{
							Name: proto.String("name"),
							JsonName: proto.String(
								"name",
							),
							Number: proto.Int32(1),
							Label: descriptorpb.
								FieldDescriptorProto_LABEL_OPTIONAL.
								Enum(),
							Type: descriptorpb.
								FieldDescriptorProto_TYPE_STRING.
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
			"create file descriptor: %v",
			err,
		)
	}

	descriptor := fileDescriptor.Messages().ByName(
		"Response",
	)

	message := dynamicpb.NewMessage(
		descriptor,
	)

	if value != nil {
		data, err := protojson.Marshal(
			newMapMessage(
				t,
				descriptor,
				value,
			),
		)
		if err != nil {
			t.Fatalf(
				"marshal test message: %v",
				err,
			)
		}

		if err := protojson.Unmarshal(
			data,
			message,
		); err != nil {
			t.Fatalf(
				"unmarshal test message: %v",
				err,
			)
		}
	}

	return message
}

func newMapMessage(
	t *testing.T,
	descriptor protoreflect.MessageDescriptor,
	value map[string]any,
) *dynamicpb.Message {
	t.Helper()

	message := dynamicpb.NewMessage(
		descriptor,
	)

	if name, ok := value["name"].(string); ok {
		field := descriptor.Fields().ByName(
			"name",
		)

		message.Set(
			field,
			protoreflect.ValueOfString(name),
		)
	}

	return message
}

func assertGRPCDeviations(
	t *testing.T,
	actual []model.TestExecutionDeviation,
	expected []model.TestExecutionDeviation,
) {
	t.Helper()

	if len(actual) != len(expected) {
		t.Fatalf(
			"expected %d deviations, got %d: %+v",
			len(expected),
			len(actual),
			actual,
		)
	}

	for i := range expected {
		if actual[i] != expected[i] {
			t.Errorf(
				"deviation %d: expected %+v, got %+v",
				i,
				expected[i],
				actual[i],
			)
		}
	}
}
