package grpc

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"

	"github.com/e2engine/core/execute/call"
	servicegrpc "github.com/e2engine/core/execute/runtime/service/grpc"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/model/matcher"
	"github.com/e2engine/core/pkg/keys"
)

func TestEvaluatorEvaluateResponse(t *testing.T) {
	tests := []struct {
		name string

		callErr error
		header  metadata.MD
		trailer metadata.MD
		message map[string]any
		expect  model.GRPCResponsePattern

		expectedStatus            string
		expectedDeviations        []model.TestExecutionDeviation
		expectedPayloadDeviations []model.PayloadDeviation
	}{
		{
			name: "matching response",
			header: metadata.MD{
				"x-header":  {"header-value"},
				"x-ignored": {"ignored-value"},
			},
			trailer: metadata.MD{
				"x-trailer": {"trailer-value"},
			},
			message: map[string]any{
				"name": "actual",
			},
			expect: model.GRPCResponsePattern{
				Status: "OK",
				Metadata: map[string][]matcher.String{
					"x-header":  {grpcStringEquals("header-value")},
					"x-trailer": {grpcStringEquals("trailer-value")},
				},
				Message: mapPtr(map[string]any{
					"name": "actual",
				}),
			},
			expectedStatus: "OK",
		},
		{
			name: "unrelated metadata",
			header: metadata.MD{
				"x-header": {"header-value"},
			},
			trailer: metadata.MD{
				"x-trailer": {"trailer-value"},
			},
			message: map[string]any{
				"name": "actual",
			},
			expect: model.GRPCResponsePattern{
				Status: "OK",
			},
			expectedStatus: "OK",
		},
		{
			name: "metadata mismatch",
			header: metadata.MD{
				"x-header": {"actual"},
			},
			message: map[string]any{
				"name": "actual",
			},
			expect: model.GRPCResponsePattern{
				Status: "OK",
				Metadata: map[string][]matcher.String{
					"x-header": {grpcStringEquals("expected")},
				},
			},
			expectedStatus: "OK",
			expectedDeviations: []model.TestExecutionDeviation{
				{
					Field:    "metadata.x-header",
					Expected: "expected",
					Actual:   "actual",
				},
			},
		},
		{
			name: "expected metadata missing",
			message: map[string]any{
				"name": "actual",
			},
			expect: model.GRPCResponsePattern{
				Status: "OK",
				Metadata: map[string][]matcher.String{
					"x-header": {grpcStringEquals("expected")},
				},
			},
			expectedStatus: "OK",
			expectedDeviations: []model.TestExecutionDeviation{
				{
					Field:    "metadata.x-header",
					Expected: "expected",
					Actual:   "",
				},
			},
		},
		{
			name: "metadata exists",
			header: metadata.MD{
				"x-header": {""},
			},
			expect: model.GRPCResponsePattern{
				Status: "OK",
				Metadata: map[string][]matcher.String{
					"x-header": {grpcStringOperator(matcher.StringMatchOperatorExists)},
				},
			},
			expectedStatus: "OK",
		},
		{
			name: "metadata exists but missing",
			expect: model.GRPCResponsePattern{
				Status: "OK",
				Metadata: map[string][]matcher.String{
					"x-header": {grpcStringOperator(matcher.StringMatchOperatorExists)},
				},
			},
			expectedStatus: "OK",
			expectedDeviations: []model.TestExecutionDeviation{
				{
					Field:    "metadata.x-header",
					Expected: "exists",
				},
			},
		},
		{
			name: "metadata nonempty",
			header: metadata.MD{
				"x-header": {"value"},
			},
			expect: model.GRPCResponsePattern{
				Status: "OK",
				Metadata: map[string][]matcher.String{
					"x-header": {grpcStringOperator(matcher.StringMatchOperatorNonEmpty)},
				},
			},
			expectedStatus: "OK",
		},
		{
			name: "metadata nonempty rejects empty value",
			header: metadata.MD{
				"x-header": {""},
			},
			expect: model.GRPCResponsePattern{
				Status: "OK",
				Metadata: map[string][]matcher.String{
					"x-header": {grpcStringOperator(matcher.StringMatchOperatorNonEmpty)},
				},
			},
			expectedStatus: "OK",
			expectedDeviations: []model.TestExecutionDeviation{
				{
					Field:    "metadata.x-header",
					Expected: "nonempty",
				},
			},
		},
		{
			name: "metadata key is case insensitive",
			header: metadata.MD{
				"x-header": {"value"},
			},
			expect: model.GRPCResponsePattern{
				Status: "OK",
				Metadata: map[string][]matcher.String{
					"X-Header": {grpcStringEquals("value")},
				},
			},
			expectedStatus: "OK",
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
			expect: model.GRPCResponsePattern{
				Status: "OK",
			},
			expectedStatus: "NotFound",
			expectedDeviations: []model.TestExecutionDeviation{
				{
					Field:    "status",
					Expected: "OK",
					Actual:   "NotFound",
				},
			},
		},
		{
			name: "expected error status",
			callErr: status.Error(
				codes.NotFound,
				"not found",
			),
			expect: model.GRPCResponsePattern{
				Status: "NotFound",
			},
			expectedStatus: "NotFound",
		},
		{
			name: "message mismatch",
			message: map[string]any{
				"name": "actual",
			},
			expect: model.GRPCResponsePattern{
				Status: "OK",
				Message: mapPtr(map[string]any{
					"name": "expected",
				}),
			},
			expectedStatus: "OK",
			expectedPayloadDeviations: []model.PayloadDeviation{
				{
					Field: "message",
					Expected: mapPtr(map[string]any{
						"name": "expected",
					}),
					Actual: []byte(`{"name":"actual"}`),
				},
			},
		},
		{
			name: "no message expectation",
			message: map[string]any{
				"name": "actual",
			},
			expect: model.GRPCResponsePattern{
				Status: "OK",
			},
			expectedStatus: "OK",
		},
		{
			name:    "explicit empty message",
			message: map[string]any{},
			expect: model.GRPCResponsePattern{
				Status:  "OK",
				Message: mapPtr(map[string]any{}),
			},
			expectedStatus: "OK",
		},
		{
			name: "nonempty message matches",
			message: map[string]any{
				"name": "actual",
			},
			expect: model.GRPCResponsePattern{
				Status: "OK",
				Message: &matcher.GRPCMessage{
					Operator: matcher.GRPCMessageMatchOperatorNonEmpty,
				},
			},
			expectedStatus: "OK",
		},
		{
			name:    "nonempty message rejects empty message",
			message: map[string]any{},
			expect: model.GRPCResponsePattern{
				Status: "OK",
				Message: &matcher.GRPCMessage{
					Operator: matcher.GRPCMessageMatchOperatorNonEmpty,
				},
			},
			expectedStatus: "OK",
			expectedPayloadDeviations: []model.PayloadDeviation{
				{
					Field: "message",
					Expected: &matcher.GRPCMessage{
						Operator: matcher.GRPCMessageMatchOperatorNonEmpty,
					},
					Actual: []byte(`{}`),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			reporter := NewMockexecutionReporter(ctrl)

			evaluator := newTestEvaluator(reporter)
			response := newTestMessage(t, tt.message)

			reporter.EXPECT().
				AddGRPCResponseStatus(tt.expectedStatus)

			for i := range tt.expectedDeviations {
				deviation := &tt.expectedDeviations[i]

				if strings.HasPrefix(deviation.Field, "metadata.") {
					reporter.EXPECT().
						AddMetadataDeviation(
							strings.TrimPrefix(deviation.Field, "metadata."),
							deviation,
						)
				} else {
					reporter.EXPECT().
						AddDeviation(deviation)
				}
			}

			expectedMetadata := mergeMetadata(
				tt.header,
				tt.trailer,
			)

			reporter.EXPECT().
				AddGRPCResponseMetadata(expectedMetadata)

			expectedMessage := tt.message
			if expectedMessage == nil {
				expectedMessage = map[string]any{}
			}

			reporter.EXPECT().
				AddGRPCResponseMessage(expectedMessage)

			for i := range tt.expectedPayloadDeviations {
				reporter.EXPECT().
					AddPayloadDeviation(
						&tt.expectedPayloadDeviations[i],
					)
			}

			err := evaluator.EvaluateResponse(
				tt.callErr,
				response,
				tt.header,
				tt.trailer,
				&tt.expect,
			)
			if err != nil {
				t.Fatalf(
					"EvaluateResponse() error = %v",
					err,
				)
			}
		})
	}
}

func TestEvaluatorEvaluateResponseUnknownStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	reporter := NewMockexecutionReporter(ctrl)

	evaluator := newTestEvaluator(reporter)

	reporter.EXPECT().
		AddGRPCResponseStatus("OK")

	err := evaluator.EvaluateResponse(
		nil,
		newTestMessage(t, map[string]any{}),
		nil,
		nil,
		&model.GRPCResponsePattern{
			Status: "invalid",
		},
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
		t.Fatalf("ResolveMethod() error = %v", err)
	}

	message := dynamicpb.NewMessage(method.Input())

	message.Set(
		method.Input().Fields().ByName("name"),
		protoreflect.ValueOfString("Yaroslav"),
	)

	messageBytes, err := proto.Marshal(message)
	if err != nil {
		t.Fatalf("marshal message: %v", err)
	}

	actual := call.Call{
		ServiceID: "service-1",
		GRPC: &call.GRPCCall{
			Request: call.GRPCRequest{
				Service: "users.UserService",
				Method:  "Create",
				Metadata: map[string][]string{
					"x-request-id": {"request-1"},
					"x-empty":      {""},
				},
				Message: messageBytes,
			},
		},
	}

	tests := []struct {
		name string

		actual   call.Call
		expected model.ServiceExpectedCalls
		match    bool
	}{
		{
			name:   "matches",
			actual: actual,
			expected: model.ServiceExpectedCalls{
				ServiceID: "service-1",
				GRPC: &model.GRPCRequestPattern{
					Service: "users.UserService",
					Method:  "Create",
					Metadata: map[string][]matcher.String{
						"x-request-id": {grpcStringEquals("request-1")},
					},
					Message: mapPtr(map[string]any{
						"name": "Yaroslav",
					}),
				},
			},
			match: true,
		},
		{
			name:   "different service",
			actual: actual,
			expected: model.ServiceExpectedCalls{
				ServiceID: "service-2",
				GRPC: &model.GRPCRequestPattern{
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
			expected: model.ServiceExpectedCalls{
				ServiceID: "service-1",
				GRPC: &model.GRPCRequestPattern{
					Service: "users.UserService",
					Method:  "Create",
				},
			},
		},
		{
			name:   "expectation is not gRPC",
			actual: actual,
			expected: model.ServiceExpectedCalls{
				ServiceID: "service-1",
			},
		},
		{
			name:   "RPC mismatch",
			actual: actual,
			expected: model.ServiceExpectedCalls{
				ServiceID: "service-1",
				GRPC: &model.GRPCRequestPattern{
					Service: "users.UserService",
					Method:  "Get",
				},
			},
		},
		{
			name:   "metadata missing",
			actual: actual,
			expected: model.ServiceExpectedCalls{
				ServiceID: "service-1",
				GRPC: &model.GRPCRequestPattern{
					Service: "users.UserService",
					Method:  "Create",
					Metadata: map[string][]matcher.String{
						"x-missing": {grpcStringEquals("value")},
					},
				},
			},
		},
		{
			name:   "metadata mismatch",
			actual: actual,
			expected: model.ServiceExpectedCalls{
				ServiceID: "service-1",
				GRPC: &model.GRPCRequestPattern{
					Service: "users.UserService",
					Method:  "Create",
					Metadata: map[string][]matcher.String{
						"x-request-id": {grpcStringEquals("different")},
					},
				},
			},
		},
		{
			name:   "metadata exists",
			actual: actual,
			expected: model.ServiceExpectedCalls{
				ServiceID: "service-1",
				GRPC: &model.GRPCRequestPattern{
					Service: "users.UserService",
					Method:  "Create",
					Metadata: map[string][]matcher.String{
						"x-empty": {grpcStringOperator(matcher.StringMatchOperatorExists)},
					},
				},
			},
			match: true,
		},
		{
			name:   "metadata exists missing",
			actual: actual,
			expected: model.ServiceExpectedCalls{
				ServiceID: "service-1",
				GRPC: &model.GRPCRequestPattern{
					Service: "users.UserService",
					Method:  "Create",
					Metadata: map[string][]matcher.String{
						"x-missing": {grpcStringOperator(matcher.StringMatchOperatorExists)},
					},
				},
			},
		},
		{
			name:   "metadata nonempty",
			actual: actual,
			expected: model.ServiceExpectedCalls{
				ServiceID: "service-1",
				GRPC: &model.GRPCRequestPattern{
					Service: "users.UserService",
					Method:  "Create",
					Metadata: map[string][]matcher.String{
						"x-request-id": {grpcStringOperator(matcher.StringMatchOperatorNonEmpty)},
					},
				},
			},
			match: true,
		},
		{
			name:   "metadata nonempty rejects empty value",
			actual: actual,
			expected: model.ServiceExpectedCalls{
				ServiceID: "service-1",
				GRPC: &model.GRPCRequestPattern{
					Service: "users.UserService",
					Method:  "Create",
					Metadata: map[string][]matcher.String{
						"x-empty": {grpcStringOperator(matcher.StringMatchOperatorNonEmpty)},
					},
				},
			},
		},
		{
			name:   "metadata case insensitive",
			actual: actual,
			expected: model.ServiceExpectedCalls{
				ServiceID: "service-1",
				GRPC: &model.GRPCRequestPattern{
					Service: "users.UserService",
					Method:  "Create",
					Metadata: map[string][]matcher.String{
						"X-Request-Id": {grpcStringEquals("request-1")},
					},
				},
			},
			match: true,
		},
		{
			name:   "message mismatch",
			actual: actual,
			expected: model.ServiceExpectedCalls{
				ServiceID: "service-1",
				GRPC: &model.GRPCRequestPattern{
					Service: "users.UserService",
					Method:  "Create",
					Message: mapPtr(map[string]any{
						"name": "Other",
					}),
				},
			},
		},
		{
			name:   "no message expectation",
			actual: actual,
			expected: model.ServiceExpectedCalls{
				ServiceID: "service-1",
				GRPC: &model.GRPCRequestPattern{
					Service: "users.UserService",
					Method:  "Create",
				},
			},
			match: true,
		},
		{
			name:   "explicit empty message does not match",
			actual: actual,
			expected: model.ServiceExpectedCalls{
				ServiceID: "service-1",
				GRPC: &model.GRPCRequestPattern{
					Service: "users.UserService",
					Method:  "Create",
					Message: mapPtr(map[string]any{}),
				},
			},
		},
		{
			name:   "nonempty message matches",
			actual: actual,
			expected: model.ServiceExpectedCalls{
				ServiceID: "service-1",
				GRPC: &model.GRPCRequestPattern{
					Service: "users.UserService",
					Method:  "Create",
					Message: &matcher.GRPCMessage{
						Operator: matcher.GRPCMessageMatchOperatorNonEmpty,
					},
				},
			},
			match: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			reporter := NewMockexecutionReporter(ctrl)

			evaluator := NewEvaluator(resolver, reporter)

			matched, err := evaluator.Matches(
				context.Background(),
				service,
				tt.actual,
				tt.expected,
			)
			if err != nil {
				t.Fatalf("Matches() error = %v", err)
			}

			if matched != tt.match {
				t.Errorf(
					"expected match %v, got %v",
					tt.match,
					matched,
				)
			}
		})
	}
}

func TestEvaluatorMatchesMissingMethod(t *testing.T) {
	ctrl := gomock.NewController(t)
	reporter := NewMockexecutionReporter(ctrl)

	service := testServiceSpec()
	evaluator := newTestEvaluator(reporter)

	actual := call.Call{
		ServiceID: "service-1",
		GRPC: &call.GRPCCall{
			Request: call.GRPCRequest{
				Service: "users.UserService",
				Method:  "Missing",
			},
		},
	}

	expected := model.ServiceExpectedCalls{
		ServiceID: "service-1",
		GRPC: &model.GRPCRequestPattern{
			Service: "users.UserService",
			Method:  "Missing",
			Message: mapPtr(map[string]any{
				"name": "Yaroslav",
			}),
		},
	}

	matched, err := evaluator.Matches(
		context.Background(),
		service,
		actual,
		expected,
	)

	if err == nil {
		t.Fatal("expected error")
	}

	if matched {
		t.Fatal("expected no match")
	}
}

func TestEvaluatorMatchesInvalidCapturedMessage(t *testing.T) {
	ctrl := gomock.NewController(t)
	reporter := NewMockexecutionReporter(ctrl)

	service := testServiceSpec()
	evaluator := newTestEvaluator(reporter)

	actual := call.Call{
		ServiceID: "service-1",
		GRPC: &call.GRPCCall{
			Request: call.GRPCRequest{
				Service: "users.UserService",
				Method:  "Create",
				Message: []byte{0xff},
			},
		},
	}

	expected := model.ServiceExpectedCalls{
		ServiceID: "service-1",
		GRPC: &model.GRPCRequestPattern{
			Service: "users.UserService",
			Method:  "Create",
			Message: mapPtr(map[string]any{
				"name": "Yaroslav",
			}),
		},
	}

	_, err := evaluator.Matches(
		context.Background(),
		service,
		actual,
		expected,
	)

	if !errors.Is(err, ErrCannotDecodeCapturedRequest) {
		t.Fatalf(
			"expected ErrCannotDecodeCapturedRequest, got %v",
			err,
		)
	}
}

func TestEvaluatorDeviation(t *testing.T) {
	ctrl := gomock.NewController(t)
	reporter := NewMockexecutionReporter(ctrl)

	evaluator := newTestEvaluator(reporter)
	service := testServiceSpec()

	tests := []struct {
		name     string
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
						Service: "users.UserService",
						Method:  "Create",
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
						Service: "users.UserService",
						Method:  "Create",
					},
					Response: call.GRPCResponse{
						Metadata: map[string][]string{
							string(keys.E2EngineMockMissMetadata): {"false"},
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
						Service: "users.UserService",
						Method:  "Create",
					},
					Response: call.GRPCResponse{
						Metadata: map[string][]string{
							string(keys.E2EngineMockMissMetadata): {"true"},
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

			if !reflect.DeepEqual(actual, tt.expected) {
				t.Errorf(
					"expected deviation %+v, got %+v",
					tt.expected,
					actual,
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
		t.Fatalf("messageToMap() error = %v", err)
	}

	expected := map[string]any{
		"name": "Yaroslav",
	}

	if !reflect.DeepEqual(actual, expected) {
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

	actual := mergeMetadata(header, trailer)

	expected := map[string][]string{
		"x-header":  {"one"},
		"x-trailer": {"two"},
		"x-shared":  {"header", "trailer"},
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf(
			"expected %#v, got %#v",
			expected,
			actual,
		)
	}

	header["x-header"][0] = "changed"

	if actual["x-header"][0] != "one" {
		t.Error("expected merged metadata to be independent of header")
	}
}

func TestGRPCCode(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected codes.Code
	}{
		{"OK", "OK", codes.OK},
		{"Canceled", "Canceled", codes.Canceled},
		{"Unknown", "Unknown", codes.Unknown},
		{"InvalidArgument", "InvalidArgument", codes.InvalidArgument},
		{"DeadlineExceeded", "DeadlineExceeded", codes.DeadlineExceeded},
		{"NotFound", "NotFound", codes.NotFound},
		{"AlreadyExists", "AlreadyExists", codes.AlreadyExists},
		{"PermissionDenied", "PermissionDenied", codes.PermissionDenied},
		{"ResourceExhausted", "ResourceExhausted", codes.ResourceExhausted},
		{"FailedPrecondition", "FailedPrecondition", codes.FailedPrecondition},
		{"Aborted", "Aborted", codes.Aborted},
		{"OutOfRange", "OutOfRange", codes.OutOfRange},
		{"Unimplemented", "Unimplemented", codes.Unimplemented},
		{"Internal", "Internal", codes.Internal},
		{"Unavailable", "Unavailable", codes.Unavailable},
		{"DataLoss", "DataLoss", codes.DataLoss},
		{"Unauthenticated", "Unauthenticated", codes.Unauthenticated},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := grpcCode(tt.value)
			if err != nil {
				t.Fatalf("grpcCode() error = %v", err)
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

	if !errors.Is(err, ErrUnknownStatus) {
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

func newTestEvaluator(
	reporter executionReporter,
) *Evaluator {
	return NewEvaluator(
		servicegrpc.NewProtoMethodResolver(nil),
		reporter,
	)
}

func testServiceSpec() *model.ServiceSpec {
	return &model.ServiceSpec{
		ID:   "service-1",
		Kind: model.ServiceKindGRPC,
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
							Name:     proto.String("name"),
							JsonName: proto.String("name"),
							Number:   proto.Int32(1),
							Label: descriptorpb.
								FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
							Type: descriptorpb.
								FieldDescriptorProto_TYPE_STRING.Enum(),
						},
					},
				},
			},
		},
		nil,
	)
	if err != nil {
		t.Fatalf("create file descriptor: %v", err)
	}

	descriptor := fileDescriptor.Messages().ByName("Response")
	message := dynamicpb.NewMessage(descriptor)

	if value != nil {
		data, err := protojson.Marshal(
			newMapMessage(t, descriptor, value),
		)
		if err != nil {
			t.Fatalf("marshal test message: %v", err)
		}

		if err := protojson.Unmarshal(data, message); err != nil {
			t.Fatalf("unmarshal test message: %v", err)
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

	message := dynamicpb.NewMessage(descriptor)

	if name, ok := value["name"].(string); ok {
		field := descriptor.Fields().ByName("name")

		message.Set(
			field,
			protoreflect.ValueOfString(name),
		)
	}

	return message
}

func mapPtr(value map[string]any) *matcher.GRPCMessage {
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

func grpcStringOperator(
	operator matcher.StringMatchOperator,
) matcher.String {
	return matcher.String{
		Operator: operator,
	}
}
