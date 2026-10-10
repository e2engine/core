package worker

import (
	"encoding/json"

	"github.com/e2engine/core/internal/util"
	"github.com/e2engine/core/model"
)

func newSummary(spec *model.TestSpec) *model.TestExecutionSummary {
	summary := &model.TestExecutionSummary{
		ExpectedCalls: newExpectedCallsSummary(spec.Expect.Calls),
	}

	switch {
	case spec.Request.HTTP != nil:
		summary.Request = &model.TestExecutionRequestSummary{
			HTTP: &model.TestExecutionHTTPRequestSummary{
				Method:   spec.Request.HTTP.Method,
				URL:      spec.Request.HTTP.URL,
				Headers:  util.CloneHTTPHeader(spec.Request.HTTP.Headers),
				BodyJSON: util.CompactJSON(json.RawMessage(spec.Request.HTTP.Body)),
			},
		}

	case spec.Request.GRPC != nil:
		summary.Request = &model.TestExecutionRequestSummary{
			GRPC: &model.TestExecutionGRPCRequestSummary{
				Service:  spec.Request.GRPC.Service,
				Method:   spec.Request.GRPC.Method,
				Metadata: util.CloneStringSliceMap(spec.Request.GRPC.Metadata),
				Message:  util.CloneMap(spec.Request.GRPC.Message),
			},
		}
	}

	return summary
}

func newExpectedCallsSummary(
	expectedCalls []model.CallExpectation,
) []model.TestExecutionCallExpectationSummary {
	if len(expectedCalls) == 0 {
		return nil
	}

	result := make(
		[]model.TestExecutionCallExpectationSummary,
		0,
		len(expectedCalls),
	)

	for _, expected := range expectedCalls {
		summary := model.TestExecutionCallExpectationSummary{
			ServiceID: expected.ServiceID,
			Count:     expected.Count,
		}

		if expected.HTTP != nil {
			summary.HTTP = &model.TestExecutionHTTPCallExpectationSummary{
				Method:  expected.HTTP.Method,
				Path:    expected.HTTP.Path,
				Query:   util.CloneQuery(expected.HTTP.Query),
				Headers: util.CloneHTTPHeader(expected.HTTP.Headers),
				BodyJSON: util.CompactJSON(
					json.RawMessage(expected.HTTP.Body),
				),
			}
		}

		if expected.GRPC != nil {
			summary.GRPC = &model.TestExecutionGRPCCallExpectationSummary{
				Service:  expected.GRPC.Service,
				Method:   expected.GRPC.Method,
				Metadata: util.CloneStringSliceMap(expected.GRPC.Metadata),
				Message:  util.CloneMap(expected.GRPC.Message),
			}
		}

		result = append(result, summary)
	}

	return result
}
