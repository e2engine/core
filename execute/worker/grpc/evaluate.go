package grpc

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"github.com/ygrebnov/errorc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/dynamicpb"

	"github.com/e2engine/core/execute/call"
	"github.com/e2engine/core/execute/match"
	servicegrpc "github.com/e2engine/core/execute/runtime/service/grpc"
	"github.com/e2engine/core/execute/worker/evaluator"
	"github.com/e2engine/core/internal/util"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/keys"
)

var (
	ErrCannotDecodeCapturedRequest = errorc.New("cannot decode captured gRPC request")
	ErrUnknownStatus               = errorc.New("unknown gRPC status")
)

var _ evaluator.CallEvaluator = (*Evaluator)(nil)

type Evaluator struct {
	resolver servicegrpc.MethodResolver
	reporter executionReporter
}

func NewEvaluator(
	resolver servicegrpc.MethodResolver,
	reporter executionReporter,
) *Evaluator {
	return &Evaluator{
		resolver: resolver,
		reporter: reporter,
	}
}

func (e *Evaluator) EvaluateResponse(
	callErr error,
	response *dynamicpb.Message,
	header metadata.MD,
	trailer metadata.MD,
	expect *model.GRPCResponsePattern,
) error {
	if err := e.evaluateStatus(callErr, expect); err != nil {
		return err
	}

	if err := e.evaluateMetadata(header, trailer, expect); err != nil {
		return err
	}

	return e.evaluateMessage(response, expect)
}

func (e *Evaluator) evaluateStatus(
	callErr error,
	expect *model.GRPCResponsePattern,
) error {
	actualStatus := status.Code(callErr)
	e.reporter.AddGRPCResponseStatus(actualStatus.String())

	expectedStatus, err := grpcCode(
		expect.Status,
	)
	if err != nil {
		return err
	}

	if actualStatus != expectedStatus {
		e.reporter.AddDeviation(&model.TestExecutionDeviation{
			Field:    string(keys.FieldStatus),
			Expected: expectedStatus.String(),
			Actual:   actualStatus.String(),
		})
	}

	return nil
}

func (e *Evaluator) evaluateMetadata(
	header metadata.MD,
	trailer metadata.MD,
	expect *model.GRPCResponsePattern,
) error {
	actualMetadata := mergeMetadata(header, trailer)
	e.reporter.AddGRPCResponseMetadata(actualMetadata)

	for key, expectedValues := range expect.Metadata {
		actualValues, exists := actualMetadata[strings.ToLower(key)]

		if match.IsStringSliceMatched(
			actualValues,
			exists,
			expectedValues,
		) {
			continue
		}

		sortedActual := slices.Clone(actualValues)
		slices.Sort(sortedActual)

		expectedStrings := make([]string, len(expectedValues))
		for i, v := range expectedValues {
			expectedStrings[i] = v.String()
		}
		slices.Sort(expectedStrings)

		e.reporter.AddMetadataDeviation(
			key,
			&model.TestExecutionDeviation{
				Field:    "metadata." + key,
				Expected: strings.Join(expectedStrings, ","),
				Actual:   strings.Join(sortedActual, ","),
			},
		)
	}

	return nil
}

func (e *Evaluator) evaluateMessage(
	response *dynamicpb.Message,
	expect *model.GRPCResponsePattern,
) error {
	responseMap, err := messageToMap(response)
	if err != nil {
		return err
	}

	e.reporter.AddGRPCResponseMessage(responseMap)

	if expect.Message == nil {
		return nil
	}

	matched, err := match.IsGRPCMessageMatched(
		response,
		expect.Message,
	)
	if err != nil {
		return err
	}

	if matched {
		return nil
	}

	actualJSON, err := protojson.Marshal(response)
	if err != nil {
		return err
	}

	e.reporter.AddPayloadDeviation(&model.PayloadDeviation{
		Field:    string(keys.FieldMessage),
		Expected: expect.Message,
		Actual:   actualJSON,
	})

	return nil
}

func (e *Evaluator) Matches(
	ctx context.Context,
	service *model.ServiceSpec,
	actual call.Call,
	expected model.ServiceExpectedCalls,
) (bool, error) {
	if actual.ServiceID != expected.ServiceID {
		return false, nil
	}

	if actual.GRPC == nil ||
		expected.GRPC == nil {
		return false, nil
	}

	if actual.GRPC.Request.Service != expected.GRPC.Service ||
		actual.GRPC.Request.Method != expected.GRPC.Method {
		return false, nil
	}

	for key, expectedValues := range expected.GRPC.Metadata {
		actualValues, exists := actual.GRPC.Request.Metadata[strings.ToLower(key)]

		if !match.IsStringSliceMatched(
			actualValues,
			exists,
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

	return match.IsGRPCMessageMatched(
		actualMessage,
		expected.GRPC.Message,
	)
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

	actualRPC := "/" +
		actual.GRPC.Request.Service +
		"/" +
		actual.GRPC.Request.Method

	return &model.TestExecutionDeviation{
		Field:  "calls." + service.ID,
		Actual: actualRPC,
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
