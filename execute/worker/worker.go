package worker

import (
	"context"
	"net/http"
	"time"

	"github.com/ygrebnov/errorc"

	"github.com/e2engine/core/execute"
	grpcservice "github.com/e2engine/core/execute/runtime/service/grpc"
	evaluatorPkg "github.com/e2engine/core/execute/worker/evaluator"
	grpcworker "github.com/e2engine/core/execute/worker/grpc"
	httpworker "github.com/e2engine/core/execute/worker/http"
	reporterPkg "github.com/e2engine/core/execute/worker/reporter"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/keys"
	"github.com/e2engine/core/pkg/log"
)

type Worker struct {
	cfg    *Config
	logger log.Logger

	client             *http.Client
	grpcMethodResolver grpcservice.MethodResolver
	addressResolver    addressResolver
}

type Config struct {
	TestTimeout   time.Duration               `json:"test_timeout" yaml:"test_timeout" default:"30s" validate:"min(1s)"`
	HTTPEvaluator *httpworker.EvaluatorConfig `json:"http_evaluator" yaml:"http_evaluator" default:"dive"`
	Reporter      *reporterPkg.Config         `json:"reporter" yaml:"reporter"`
}

func NewWorker(
	cfg *Config,
	logger log.Logger,
	grpcMethodResolver grpcservice.MethodResolver,
	addressResolver addressResolver,
) *Worker {
	client := http.DefaultClient // TODO: make it configurable.

	return &Worker{
		cfg:    cfg,
		logger: logger,

		client:             client,
		grpcMethodResolver: grpcMethodResolver,
		addressResolver:    addressResolver,
	}
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

	reporter := reporterPkg.New(
		w.cfg.Reporter,
		&job.Test.Spec,
		job.ExecutionID,
		job.TestSuiteExecutionID,
	)

	// In some cases this validation can be the second validation of the test spec,
	// but it is better than failing the test execution because of an invalid test spec.
	if err := job.Test.Validate(runCtx); err != nil {
		reporter.SetError(err)

		return reporter.Result(), nil
	}

	httpEvaluator := httpworker.NewEvaluator(
		w.cfg.HTTPEvaluator,
		reporter,
	)

	grpcEvaluator := grpcworker.NewEvaluator(
		w.grpcMethodResolver,
		reporter,
	)

	evaluator := evaluatorPkg.New(
		httpEvaluator,
		grpcEvaluator,
		reporter,
	)

	var executor execute.ProtocolExecutor
	switch {
	case job.Test.Spec.Request.HTTP != nil:
		executor = httpworker.NewTestExecutor(
			w.client,
			httpEvaluator,
			reporter,
			w.addressResolver,
		)

	case job.Test.Spec.Request.GRPC != nil:
		executor = grpcworker.NewTestExecutor(
			w.grpcMethodResolver,
			grpcEvaluator,
			reporter,
			w.addressResolver,
		)

	default:
		// Test validation guarantees a supported request protocol.
		panic("invalid test spec: unsupported test protocol")
	}

	callStore := job.Runtime.Calls

	if errValidate := validateExpectedCalls(
		job.Environment.Spec.Services,
		job.Test.Spec.Expect.Calls,
	); errValidate != nil {
		reporter.SetError(errValidate)

		return reporter.Result(), nil
	}

	if err := executor.Execute(
		runCtx,
		&job,
	); err != nil {
		reporter.SummarizeCalls(callStore)
		reporter.SetError(err)

		return reporter.Result(), nil
	}

	if err := evaluator.EvaluateCalls(
		runCtx,
		job.Environment.Spec.Services,
		job.Test.Spec.Expect.Calls,
		callStore,
		job.ExecutionID,
	); err != nil {
		reporter.SummarizeCalls(callStore)
		reporter.SetError(err)

		return reporter.Result(), nil
	}

	reporter.SummarizeCalls(callStore)

	reporter.Finalize()

	return reporter.Result(), nil
}

func validateExpectedCalls(
	services []model.ServiceSpec,
	expected []model.ServiceExpectedCalls,
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
