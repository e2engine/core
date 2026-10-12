package evaluator

import (
	"context"
	"strconv"

	"github.com/e2engine/core/execute/call"
	"github.com/e2engine/core/model"
)

// CallEvaluator is an interface for evaluating calls against expectations.
// Implementations are protocol-specific, currently HTTP and gRPC.
type CallEvaluator interface {
	Matches(
		ctx context.Context,
		service *model.ServiceSpec,
		actual call.Call,
		expected model.ServiceExpectedCalls,
	) (bool, error)

	Deviation(
		service *model.ServiceSpec,
		actual call.Call,
	) *model.TestExecutionDeviation
}

type Evaluator struct {
	httpCallEvaluator CallEvaluator
	grpcCallEvaluator CallEvaluator
	reporter          deviationReporter
}

func New(
	httpCallEvaluator CallEvaluator,
	grpcCallEvaluator CallEvaluator,
	reporter deviationReporter,
) *Evaluator {
	return &Evaluator{
		httpCallEvaluator: httpCallEvaluator,
		grpcCallEvaluator: grpcCallEvaluator,
		reporter:          reporter,
	}
}

func (e *Evaluator) EvaluateCalls(
	ctx context.Context,
	services []model.ServiceSpec,
	expectedCalls []model.ServiceExpectedCalls,
	calls call.Reader,
	executionID string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	e.evaluateActualCalls(
		services,
		calls,
		executionID,
	)

	return e.evaluateExpectedCalls(
		ctx,
		services,
		expectedCalls,
		calls,
		executionID,
	)
}

func (e *Evaluator) evaluateActualCalls(
	services []model.ServiceSpec,
	calls call.Reader,
	executionID string,
) {
	for i := range services {
		service := services[i]

		if service.Mode != model.ServiceModeMocked {
			continue
		}

		filter := call.Filter{
			TestExecutionID: executionID,
			ServiceID:       service.ID,
		}
		for _, actual := range calls.Get(filter) {
			var deviation *model.TestExecutionDeviation
			switch service.Kind {
			case model.ServiceKindHTTP:
				deviation = e.httpCallEvaluator.Deviation(&service, actual)

			case model.ServiceKindGRPC:
				deviation = e.grpcCallEvaluator.Deviation(&service, actual)

			default:
				panic("invalid service spec: unsupported service kind")
			}

			if deviation == nil {
				continue
			}

			e.reporter.AddDeviation(deviation)
		}
	}
}

func (e *Evaluator) evaluateExpectedCalls(
	ctx context.Context,
	services []model.ServiceSpec,
	expectedCalls []model.ServiceExpectedCalls,
	calls call.Reader,
	executionID string,
) error {
	serviceByID := make(map[string]*model.ServiceSpec, len(services))
	for i := range services {
		serviceByID[services[i].ID] = &services[i]
	}

	for _, expected := range expectedCalls {
		service := serviceByID[expected.ServiceID]

		filter := call.Filter{
			TestExecutionID: executionID,
			ServiceID:       expected.ServiceID,
		}
		actualCalls := calls.Get(filter)

		matched := 0

		for _, actual := range actualCalls {
			ok, err := e.matches(
				ctx,
				service,
				actual,
				expected,
			)
			if err != nil {
				return err
			}

			if ok {
				matched++
			}
		}

		if expected.Count != nil {
			if matched != *expected.Count {
				e.reporter.AddDeviation(&model.TestExecutionDeviation{
					Field:    "calls." + expected.ServiceID + ".count",
					Expected: strconv.Itoa(*expected.Count),
					Actual:   strconv.Itoa(matched),
				})
			}

			continue
		}

		if matched == 0 {
			e.reporter.AddDeviation(&model.TestExecutionDeviation{
				Field:    "calls." + expected.ServiceID + ".count",
				Expected: "at least 1",
				Actual:   "0",
			})
		}
	}

	return nil
}

func (e *Evaluator) matches(
	ctx context.Context,
	service *model.ServiceSpec,
	actual call.Call,
	expected model.ServiceExpectedCalls,
) (bool, error) {
	switch {
	case expected.HTTP != nil:
		return e.httpCallEvaluator.Matches(
			ctx,
			service,
			actual,
			expected,
		)

	case expected.GRPC != nil:
		return e.grpcCallEvaluator.Matches(
			ctx,
			service,
			actual,
			expected,
		)

	default:
		panic("invalid expected call: unsupported protocol")
	}
}
