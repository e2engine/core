package evaluate

import (
	"context"
	"strconv"

	"github.com/e2engine/core/execute"
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
		expected model.CallExpectation,
	) (bool, error)

	Deviation(
		service *model.ServiceSpec,
		actual call.Call,
	) *model.TestExecutionDeviation
}

type Evaluator struct {
	httpCallEvaluator CallEvaluator
	grpcCallEvaluator CallEvaluator
}

func New(
	httpCallEvaluator CallEvaluator,
	grpcCallEvaluator CallEvaluator,
) *Evaluator {
	return &Evaluator{
		httpCallEvaluator: httpCallEvaluator,
		grpcCallEvaluator: grpcCallEvaluator,
	}
}

func (e *Evaluator) EvaluateCalls(
	ctx context.Context,
	services []model.ServiceSpec,
	expectedCalls []model.CallExpectation,
	calls call.Reader,
	result *execute.TestExecutionResult,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	e.evaluateActualCalls(
		services,
		calls,
		result,
	)

	return e.evaluateExpectedCalls(
		ctx,
		services,
		expectedCalls,
		calls,
		result,
	)
}

func (e *Evaluator) evaluateActualCalls(
	services []model.ServiceSpec,
	calls call.Reader,
	result *execute.TestExecutionResult,
) {
	for i := range services {
		service := services[i]

		if service.Mode != model.ServiceModeMocked {
			continue
		}

		filter := call.Filter{
			TestExecutionID: result.ExecutionID,
			ServiceID:       service.ID,
		}
		for _, actual := range calls.Get(filter) {
			var deviation *model.TestExecutionDeviation
			switch service.Kind {
			case model.ServiceKindHTTP:
				deviation = e.httpCallEvaluator.Deviation(&service, actual)

			case model.ServiceKindGRPC:
				deviation = e.grpcCallEvaluator.Deviation(&service, actual)
			}

			if deviation == nil {
				continue
			}

			result.Summary.Deviations = append(
				result.Summary.Deviations,
				*deviation,
			)
		}
	}
}

func (e *Evaluator) evaluateExpectedCalls(
	ctx context.Context,
	services []model.ServiceSpec,
	expectedCalls []model.CallExpectation,
	calls call.Reader,
	result *execute.TestExecutionResult,
) error {
	serviceByID := make(map[string]*model.ServiceSpec, len(services))
	for i := range services {
		serviceByID[services[i].ID] = &services[i]
	}

	for _, expected := range expectedCalls {
		service := serviceByID[expected.ServiceID]

		filter := call.Filter{
			TestExecutionID: result.ExecutionID,
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
				result.Summary.Deviations = append(
					result.Summary.Deviations,
					model.TestExecutionDeviation{
						Field:    "calls." + expected.ServiceID + ".count",
						Expected: strconv.Itoa(*expected.Count),
						Actual:   strconv.Itoa(matched),
					},
				)
			}

			continue
		}

		if matched == 0 {
			result.Summary.Deviations = append(
				result.Summary.Deviations,
				model.TestExecutionDeviation{
					Field:    "calls." + expected.ServiceID + ".count",
					Expected: "at least 1",
					Actual:   "0",
				},
			)
		}
	}

	return nil
}

func (e *Evaluator) matches(
	ctx context.Context,
	service *model.ServiceSpec,
	actual call.Call,
	expected model.CallExpectation,
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
		return false, nil
	}
}
