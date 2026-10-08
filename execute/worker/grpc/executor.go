package grpc

import (
	"context"
	"encoding/json"

	e2enginegrpc "github.com/e2engine/instrumentation-go/grpc"
	"github.com/ygrebnov/errorc"
	ggrpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/dynamicpb"

	"github.com/e2engine/core/execute"
	servicegrpc "github.com/e2engine/core/execute/runtime/service/grpc"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/keys"
)

type TestExecutor struct {
	resolver  servicegrpc.MethodResolver
	evaluator *Evaluator
}

func NewTestExecutor(
	resolver servicegrpc.MethodResolver,
	evaluator *Evaluator,
) *TestExecutor {
	return &TestExecutor{
		resolver:  resolver,
		evaluator: evaluator,
	}
}

func (e *TestExecutor) Execute(
	ctx context.Context,
	job *execute.TestJob,
	result *execute.TestExecutionResult,
) error {
	spec := job.Test.Spec.Request.GRPC

	service, err := findService(
		job.Environment.Spec.Services,
		spec.Target,
	)
	if err != nil {
		return err
	}

	method, err := e.resolver.ResolveMethod(
		ctx,
		service.Proto,
		spec.Method,
	)
	if err != nil {
		return err
	}

	request := dynamicpb.NewMessage(
		method.Input(),
	)

	if errUnmarshal := unmarshalMap(
		spec.Message,
		request,
	); errUnmarshal != nil {
		return errorc.With(
			errors.ErrCannotBuildGRPCRequest,
			errorc.Error(keys.Cause, errUnmarshal),
		)
	}

	connection, err := ggrpc.NewClient(
		spec.Target,
		ggrpc.WithTransportCredentials(
			insecure.NewCredentials(),
		),
	)
	if err != nil {
		return err
	}
	defer connection.Close()

	callMetadata := metadata.MD(spec.Metadata).Copy()
	callMetadata = e2enginegrpc.WithTestExecutionID(callMetadata, job.ExecutionID)

	callCtx := metadata.NewOutgoingContext(
		ctx,
		callMetadata,
	)

	response := dynamicpb.NewMessage(
		method.Output(),
	)

	var header metadata.MD
	var trailer metadata.MD

	callErr := connection.Invoke(
		callCtx,
		"/"+spec.Service+"/"+spec.Method,
		request,
		response,
		ggrpc.Header(&header),
		ggrpc.Trailer(&trailer),
	)

	return e.evaluator.EvaluateResponse(
		ctx,
		callErr,
		response,
		header,
		trailer,
		job.Test.Spec.Expect.GRPC,
		result,
	)
}

func findService(
	services []model.ServiceSpec,
	target string,
) (*model.ServiceSpec, error) {
	for i := range services {
		if services[i].Address == target {
			return &services[i], nil
		}
	}

	return nil, errorc.With(
		errors.ErrGRPCTargetServiceNotFound,
		errorc.String(keys.GRPCServiceTargetAddress, target),
	)
}

func unmarshalMap(
	value map[string]any,
	message *dynamicpb.Message,
) error {
	if value == nil {
		return nil
	}

	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return protojson.Unmarshal(
		data,
		message,
	)
}
