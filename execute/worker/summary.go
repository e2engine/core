package worker

import (
	"encoding/json"

	"github.com/e2engine/core/execute/call"
	"github.com/e2engine/core/internal/util"
	"github.com/e2engine/core/model"
)

func newSummary(spec *model.TestSpec) *model.TestExecutionSummary {
	summary := &model.TestExecutionSummary{
		Expect: model.TestExecutionExpectSummary{
			Calls: newExpectedCallsSummary(spec.Expect.Calls),
		},
	}

	switch {
	case spec.Request.HTTP != nil:
		setHTTPSummary(summary, spec)

	case spec.Request.GRPC != nil:
		setGRPCSummary(summary, spec)
	}

	return summary
}

func setHTTPSummary(
	summary *model.TestExecutionSummary,
	spec *model.TestSpec,
) {
	summary.Request.HTTP = &model.TestExecutionHTTPRequestSummary{
		Method:   spec.Request.HTTP.Method,
		URL:      spec.Request.HTTP.URL,
		Headers:  util.CloneHTTPHeader(spec.Request.HTTP.Headers),
		BodyJSON: util.CompactJSON(json.RawMessage(spec.Request.HTTP.Body)),
	}

	summary.Expect.HTTP = &model.TestExecutionHTTPExpectSummary{
		StatusCode: spec.Expect.HTTP.Status,
		BodyJSON:   util.CompactJSON(json.RawMessage(spec.Expect.HTTP.Body)),
	}
}

func setGRPCSummary(
	summary *model.TestExecutionSummary,
	spec *model.TestSpec,
) {
	summary.Request.GRPC = &model.TestExecutionGRPCRequestSummary{
		Service:  spec.Request.GRPC.Service,
		Method:   spec.Request.GRPC.Method,
		Metadata: util.CloneStringSliceMap(spec.Request.GRPC.Metadata),
		Message:  util.CloneMap(spec.Request.GRPC.Message),
	}

	summary.Expect.GRPC = &model.TestExecutionGRPCExpectSummary{
		Status:  spec.Expect.GRPC.Status,
		Message: util.CloneMap(spec.Expect.GRPC.Message),
	}
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

func newCallsSummary(
	calls []call.Call,
) []model.TestExecutionCallSummary {
	if len(calls) == 0 {
		return nil
	}

	result := make(
		[]model.TestExecutionCallSummary,
		0,
		len(calls),
	)

	for _, actual := range calls {
		summary := model.TestExecutionCallSummary{
			ServiceID: actual.ServiceID,
		}

		if actual.HTTP != nil {
			summary.HTTP = &model.TestExecutionHTTPCallSummary{
				Request: model.TestExecutionHTTPCallRequestSummary{
					Method: actual.HTTP.Request.Method,
					Path:   actual.HTTP.Request.Path,
					Query:  util.CloneQuery(actual.HTTP.Request.Query),
					Headers: util.CloneHTTPHeader(
						actual.HTTP.Request.Headers,
					),
					BodyJSON: util.CompactJSON(
						actual.HTTP.Request.Body,
					),
				},
				Response: model.TestExecutionHTTPCallResponseSummary{
					StatusCode: actual.HTTP.Response.StatusCode,
					Headers: util.CloneHTTPHeader(
						actual.HTTP.Response.Headers,
					),
					BodyJSON: util.CompactJSON(
						actual.HTTP.Response.Body,
					),
				},
			}
		}

		if actual.GRPC != nil {
			summary.GRPC = &model.TestExecutionGRPCCallSummary{
				Request: model.TestExecutionGRPCCallRequestSummary{
					RPC:      actual.GRPC.Request.RPC,
					Metadata: util.CloneStringSliceMap(actual.GRPC.Request.Metadata),
					Message:  util.CloneBytes(actual.GRPC.Request.Message),
				},
				Response: model.TestExecutionGRPCCallResponseSummary{
					Status:   actual.GRPC.Response.Status,
					Metadata: util.CloneStringSliceMap(actual.GRPC.Response.Metadata),
					Message:  util.CloneBytes(actual.GRPC.Response.Message),
				},
			}
		}

		result = append(result, summary)
	}

	return result
}
