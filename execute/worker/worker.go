package worker

import (
	"context"
	"net/http"
	"time"

	"github.com/ygrebnov/errorc"

	"github.com/e2engine/core/execute"
	"github.com/e2engine/core/execute/evaluate"
	grpcservice "github.com/e2engine/core/execute/runtime/service/grpc"
	grpcworker "github.com/e2engine/core/execute/worker/grpc"
	httpworker "github.com/e2engine/core/execute/worker/http"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/keys"
	"github.com/e2engine/core/pkg/log"
)

type Worker struct {
	cfg    *Config
	logger log.Logger

	httpExecutor execute.ProtocolExecutor
	grpcExecutor execute.ProtocolExecutor
	evaluator    *evaluate.Evaluator
}

type Config struct {
	TestTimeout time.Duration `json:"test_timeout" yaml:"test_timeout" default:"30s"` // TODO: add validation.
}

func NewWorker(
	cfg *Config,
	logger log.Logger,
	grpcMethodResolver grpcservice.MethodResolver,
) (*Worker, error) {
	client := http.DefaultClient // TODO: make it configurable.

	httpEvaluator := httpworker.NewEvaluator()

	grpcEvaluator := grpcworker.NewEvaluator(
		grpcMethodResolver,
	)

	evaluator := evaluate.New(
		httpEvaluator,
		grpcEvaluator,
	)

	httpExecutor := httpworker.NewTestExecutor(
		client,
		httpEvaluator,
	)

	grpcExecutor := grpcworker.NewTestExecutor(
		grpcMethodResolver,
		grpcEvaluator,
	)

	return &Worker{
		cfg:    cfg,
		logger: logger,

		httpExecutor: httpExecutor,
		grpcExecutor: grpcExecutor,
		evaluator:    evaluator,
	}, nil
}

func (w *Worker) Execute(
	ctx context.Context,
	job execute.TestJob,
) (*execute.TestExecutionResult, error) {
	runCtx, cancel := context.WithTimeout(
		ctx,
		w.cfg.TestTimeout,
	)
	defer cancel()

	callStore := job.Runtime.Calls

	result := &execute.TestExecutionResult{
		ExecutionID:          job.ExecutionID,
		TestSuiteExecutionID: job.TestSuiteExecutionID,
		Summary:              newSummary(&job.Test.Spec),
	}

	if err := validateExpectedCalls(
		job.Environment.Spec.Services,
		job.Test.Spec.Expect.Calls,
	); err != nil {
		setExecutionError(result, err)

		return result, nil
	}

	executor, err := w.getExecutor(&job.Test.Spec)
	if err != nil {
		return nil, err
	}

	if err := executor.Execute(
		runCtx,
		&job,
		result,
	); err != nil {
		setExecutionError(result, err)

		return result, nil
	}

	if err := w.evaluator.EvaluateCalls(
		runCtx,
		job.Environment.Spec.Services,
		job.Test.Spec.Expect.Calls,
		callStore,
		result,
	); err != nil {
		setExecutionError(result, err)

		return result, nil
	}

	if len(result.Summary.Deviations) == 0 {
		result.Status = model.ExecutionStatusPassed
	} else {
		result.Status = model.ExecutionStatusFailed
	}

	result.FinishedAt = time.Now().UTC()

	return result, nil
}

func (w *Worker) getExecutor(
	spec *model.TestSpec,
) (execute.ProtocolExecutor, error) {
	switch {
	case spec.Request.HTTP != nil:
		return w.httpExecutor, nil

	case spec.Request.GRPC != nil:
		return w.grpcExecutor, nil

	default:
		return nil, errors.ErrUnsupportedTestProtocol
	}
}

func validateExpectedCalls(
	services []model.ServiceSpec,
	expected []model.CallExpectation,
) error {
	byID := make(map[string]model.ServiceSpec, len(services))
	for i := range services {
		byID[services[i].ID] = services[i]
	}

	for _, expectedCall := range expected {
		s, ok := byID[expectedCall.ServiceID]
		if !ok {
			return errorc.With(
				errors.ErrInvalidTestExecutionInput,
				errorc.String(
					keys.EnvironmentServiceID,
					expectedCall.ServiceID,
				),
				errorc.String(
					keys.Validation,
					"expected call references unknown environment service",
				),
			)
		}

		if expectedCall.HTTP != nil &&
			s.Kind != model.ServiceKindHTTP {
			return errorc.With(
				errors.ErrInvalidTestExecutionInput,
				errorc.String(
					keys.EnvironmentServiceID,
					expectedCall.ServiceID,
				),
				errorc.String(
					keys.EnvironmentServiceKind,
					string(s.Kind),
				),
				errorc.String(
					keys.Validation,
					"expected HTTP call references non-HTTP service",
				),
			)
		}

		if expectedCall.GRPC != nil &&
			s.Kind != model.ServiceKindGRPC {
			return errorc.With(
				errors.ErrInvalidTestExecutionInput,
				errorc.String(
					keys.EnvironmentServiceID,
					expectedCall.ServiceID,
				),
				errorc.String(
					keys.EnvironmentServiceKind,
					string(s.Kind),
				),
				errorc.String(
					keys.Validation,
					"expected gRPC call references non-gRPC service",
				),
			)
		}
	}

	return nil
}

func setExecutionError(
	result *execute.TestExecutionResult,
	err error,
) {
	result.Status = model.ExecutionStatusError
	result.FinishedAt = time.Now().UTC()
	result.Summary.Error = err.Error()
}
