package grpc

import (
	"context"
	"encoding/json"

	"github.com/ygrebnov/errorc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/dynamicpb"

	"github.com/e2engine/core/execute"
	"github.com/e2engine/core/execute/call"
	"github.com/e2engine/core/execute/evaluate"
	servicegrpc "github.com/e2engine/core/execute/runtime/service/grpc"
	"github.com/e2engine/core/internal/util"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/keys"
)

var (
	ErrCannotDecodeCapturedRequest = errorc.New("cannot decode captured gRPC request")
	ErrUnknownStatus               = errorc.New("unknown gRPC status")
)

var _ evaluate.CallEvaluator = (*Evaluator)(nil)

type Evaluator struct {
	resolver servicegrpc.MethodResolver
}

func NewEvaluator(
	resolver servicegrpc.MethodResolver,
) *Evaluator {
	return &Evaluator{
		resolver: resolver,
	}
}

func (e *Evaluator) EvaluateResponse(
	ctx context.Context,
	callErr error,
	response *dynamicpb.Message,
	header metadata.MD,
	trailer metadata.MD,
	expect *model.GRPCExpectSpec,
	result *execute.TestExecutionResult,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	result.Summary.Response.GRPC = &model.TestExecutionGRPCResponseSummary{}

	actualStatus := status.Code(callErr)

	result.Summary.Response.GRPC.Status =
		actualStatus.String()

	result.Summary.Response.GRPC.Metadata =
		mergeMetadata(header, trailer)

	expectedStatus, err := grpcCode(
		expect.Status,
	)
	if err != nil {
		return err
	}

	if actualStatus != expectedStatus {
		result.Summary.Deviations = append(
			result.Summary.Deviations,
			model.TestExecutionDeviation{
				Field:    "status",
				Expected: expectedStatus.String(),
				Actual:   actualStatus.String(),
			},
		)
	}

	responseMap, err := messageToMap(response)
	if err != nil {
		return err
	}

	result.Summary.Response.GRPC.Message =
		responseMap

	if expect.Message == nil {
		return nil
	}

	expectedMessage := dynamicpb.NewMessage(
		response.Descriptor(),
	)

	if errUnmarshal := unmarshalMap(
		expect.Message,
		expectedMessage,
	); errUnmarshal != nil {
		return errUnmarshal
	}

	if proto.Equal(
		expectedMessage,
		response,
	) {
		return nil
	}

	expectedJSON, err := json.Marshal(
		expect.Message,
	)
	if err != nil {
		return err
	}

	actualJSON, err := protojson.Marshal(
		response,
	)
	if err != nil {
		return err
	}

	result.Summary.Deviations = append(
		result.Summary.Deviations,
		model.TestExecutionDeviation{
			Field:    "message",
			Expected: string(expectedJSON),
			Actual:   string(actualJSON),
		},
	)

	return nil
}

func (e *Evaluator) Matches(
	ctx context.Context,
	service *model.ServiceSpec,
	actual call.Call,
	expected model.CallExpectation,
) (bool, error) {
	if actual.ServiceID != expected.ServiceID {
		return false, nil
	}

	if actual.GRPC == nil ||
		expected.GRPC == nil {
		return false, nil
	}

	expectedRPC :=
		"/" +
			expected.GRPC.Service +
			"/" +
			expected.GRPC.Method

	if actual.GRPC.Request.RPC != expectedRPC {
		return false, nil
	}

	for key, expectedValues := range expected.GRPC.Metadata {
		actualValues, ok :=
			actual.GRPC.Request.Metadata[key]

		if !ok ||
			!util.IsStringSlicesEqual(
				actualValues,
				expectedValues,
			) {
			return false, nil
		}
	}

	if expected.GRPC.Message == nil {
		return true, nil
	}

	method, err := e.resolver.ResolveMethod(
		ctx,
		service.Proto,
		expected.GRPC.Method,
	)
	if err != nil {
		return false, err
	}

	actualMessage := dynamicpb.NewMessage(
		method.Input(),
	)

	if err := proto.Unmarshal(
		actual.GRPC.Request.Message,
		actualMessage,
	); err != nil {
		return false, errorc.With(
			ErrCannotDecodeCapturedRequest,
			errorc.Error(keys.Cause, err),
		)
	}

	expectedMessage := dynamicpb.NewMessage(
		method.Input(),
	)

	if err := unmarshalMap(
		expected.GRPC.Message,
		expectedMessage,
	); err != nil {
		return false, err
	}

	return proto.Equal(
		actualMessage,
		expectedMessage,
	), nil
}

func (e *Evaluator) Deviation(
	service *model.ServiceSpec,
	actual call.Call,
) *model.TestExecutionDeviation {
	if actual.GRPC == nil {
		return nil
	}

	values :=
		actual.GRPC.Response.Metadata[string(keys.E2EngineMockMissMetadata)]

	if len(values) == 0 ||
		values[0] != "true" {
		return nil
	}

	return &model.TestExecutionDeviation{
		Field:  "calls." + service.ID,
		Actual: actual.GRPC.Request.RPC,
		Message: "mocked service received a request " +
			"that did not match any fixture",
	}
}

func messageToMap(
	message proto.Message,
) (map[string]any, error) {
	data, err := protojson.Marshal(
		message,
	)
	if err != nil {
		return nil, err
	}

	var result map[string]any

	if err := json.Unmarshal(
		data,
		&result,
	); err != nil {
		return nil, err
	}

	return result, nil
}

func mergeMetadata(
	header metadata.MD,
	trailer metadata.MD,
) map[string][]string {
	result := make(
		map[string][]string,
		len(header)+len(trailer),
	)

	for key, values := range header {
		result[key] =
			util.CloneStringSlice(values)
	}

	for key, values := range trailer {
		result[key] = append(
			result[key],
			values...,
		)
	}

	return result
}

func grpcCode(
	value string,
) (codes.Code, error) {
	switch value {
	case "OK":
		return codes.OK, nil
	case "Canceled":
		return codes.Canceled, nil
	case "Unknown":
		return codes.Unknown, nil
	case "InvalidArgument":
		return codes.InvalidArgument, nil
	case "DeadlineExceeded":
		return codes.DeadlineExceeded, nil
	case "NotFound":
		return codes.NotFound, nil
	case "AlreadyExists":
		return codes.AlreadyExists, nil
	case "PermissionDenied":
		return codes.PermissionDenied, nil
	case "ResourceExhausted":
		return codes.ResourceExhausted, nil
	case "FailedPrecondition":
		return codes.FailedPrecondition, nil
	case "Aborted":
		return codes.Aborted, nil
	case "OutOfRange":
		return codes.OutOfRange, nil
	case "Unimplemented":
		return codes.Unimplemented, nil
	case "Internal":
		return codes.Internal, nil
	case "Unavailable":
		return codes.Unavailable, nil
	case "DataLoss":
		return codes.DataLoss, nil
	case "Unauthenticated":
		return codes.Unauthenticated, nil
	default:
		return codes.Unknown,
			errorc.With(
				ErrUnknownStatus,
				errorc.String(keys.GRPCStatus, value),
			)
	}
}
