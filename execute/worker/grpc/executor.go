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
	resolver        servicegrpc.MethodResolver
	evaluator       *Evaluator
	reporter        executionReporter
	addressResolver addressResolver
}

func NewTestExecutor(
	resolver servicegrpc.MethodResolver,
	evaluator *Evaluator,
	reporter executionReporter,
	addressResolver addressResolver,
) *TestExecutor {
	return &TestExecutor{
		resolver:        resolver,
		evaluator:       evaluator,
		reporter:        reporter,
		addressResolver: addressResolver,
	}
}

func (e *TestExecutor) Execute(
	ctx context.Context,
	job *execute.TestJob,
) error {
	spec := job.Test.Spec.Request.GRPC

	protoSpec := spec.Proto

	if protoSpec == nil {
		// TODO: for standalone test, return an error is no proto spec is provided.

		var err error
		protoSpec, err = findProtoSpec(
			job.Environment.Spec.Services,
			spec.Service,
		)
		if err != nil {
			return err
		}
	}

	method, err := e.resolver.ResolveMethod(
		ctx,
		protoSpec,
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

	resolvedTarget, err := e.addressResolver.ResolveGRPCTarget(spec.Target)
	if err != nil {
		return err
	}

	connection, err := ggrpc.NewClient(
		resolvedTarget,
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

	r := &model.GRPCRequest{
		Target:   resolvedTarget,
		Service:  spec.Service,
		Method:   spec.Method,
		Metadata: callMetadata,
		Message:  spec.Message,
	}

	e.reporter.AddGRPCRequest(r)

	callErr := connection.Invoke(
		callCtx,
		"/"+spec.Service+"/"+spec.Method,
		request,
		response,
		ggrpc.Header(&header),
		ggrpc.Trailer(&trailer),
	)

	return e.evaluator.EvaluateResponse(
		callErr,
		response,
		header,
		trailer,
		job.Test.Spec.Expect.GRPC,
	)
}

func findProtoSpec(
	services []model.ServiceSpec,
	serviceName string,
) (*model.GRPCProtoSpec, error) {
	var found *model.GRPCProtoSpec

	for i := range services {
		service := &services[i]

		if service.Kind != model.ServiceKindGRPC || service.Proto == nil {
			continue
		}

		var name string

		switch {
		case service.Proto.External != nil:
			name = service.Proto.External.Service

		case service.Proto.Internal != nil:
			internal := service.Proto.Internal

			name = internal.Service
			if internal.Package != "" {
				name = internal.Package + "." + internal.Service
			}
		}

		if name != serviceName {
			continue
		}

		if found != nil {
			return nil, errorc.New(
				"multiple protobuf specifications match gRPC service",
			)
		}

		found = service.Proto
	}

	if found == nil {
		return nil, errorc.New(
			"protobuf specification not found for gRPC service",
		)
	}

	return found, nil
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
