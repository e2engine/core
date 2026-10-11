package evaluate

import (
	"sort"

	"github.com/e2engine/core/execute/call"
	"github.com/e2engine/core/model"
)

type httpObservedCallKey struct {
	ServiceID          string
	Method             string
	Path               string
	ResponseStatusCode int
}

type grpcObservedCallKey struct {
	ServiceID      string
	Service        string
	Method         string
	ResponseStatus string
}

func SummarizeCalls(
	executionID string,
	calls call.Reader,
) []model.ServiceObservedCalls {
	actualCalls := calls.Get(
		call.Filter{
			TestExecutionID: executionID,
		},
	)

	if len(actualCalls) == 0 {
		return nil
	}

	httpCalls := make(map[httpObservedCallKey]int)
	grpcCalls := make(map[grpcObservedCallKey]int)

	for _, actual := range actualCalls {
		switch {
		case actual.HTTP != nil:
			key := httpObservedCallKey{
				ServiceID:          actual.ServiceID,
				Method:             actual.HTTP.Request.Method,
				Path:               actual.HTTP.Request.Path,
				ResponseStatusCode: actual.HTTP.Response.StatusCode,
			}

			httpCalls[key]++

		case actual.GRPC != nil:
			key := grpcObservedCallKey{
				ServiceID:      actual.ServiceID,
				Service:        actual.GRPC.Request.Service,
				Method:         actual.GRPC.Request.Method,
				ResponseStatus: actual.GRPC.Response.Status,
			}

			grpcCalls[key]++
		}
	}

	result := make(
		[]model.ServiceObservedCalls,
		0,
		len(httpCalls)+len(grpcCalls),
	)

	for key, count := range httpCalls {
		statusCode := key.ResponseStatusCode

		result = append(
			result,
			model.ServiceObservedCalls{
				ServiceID: key.ServiceID,
				Count:     count,
				HTTP: &model.HTTPServiceObservedCall{
					Method:             key.Method,
					Path:               key.Path,
					ResponseStatusCode: &statusCode,
				},
			},
		)
	}

	for key, count := range grpcCalls {
		result = append(
			result,
			model.ServiceObservedCalls{
				ServiceID: key.ServiceID,
				Count:     count,
				GRPC: &model.GRPCServiceObservedCall{
					Service:        key.Service,
					Method:         key.Method,
					ResponseStatus: key.ResponseStatus,
				},
			},
		)
	}

	sort.Slice(result, func(i, j int) bool {
		left := result[i]
		right := result[j]

		if left.ServiceID != right.ServiceID {
			return left.ServiceID < right.ServiceID
		}

		if left.HTTP != nil && right.GRPC != nil {
			return true
		}

		if left.GRPC != nil && right.HTTP != nil {
			return false
		}

		if left.HTTP != nil {
			if left.HTTP.Method != right.HTTP.Method {
				return left.HTTP.Method < right.HTTP.Method
			}
			if left.HTTP.Path != right.HTTP.Path {
				return left.HTTP.Path < right.HTTP.Path
			}

			return observedHTTPStatus(left.HTTP) <
				observedHTTPStatus(right.HTTP)
		}

		if left.GRPC.Service != right.GRPC.Service {
			return left.GRPC.Service < right.GRPC.Service
		}
		if left.GRPC.Method != right.GRPC.Method {
			return left.GRPC.Method < right.GRPC.Method
		}

		return left.GRPC.ResponseStatus <
			right.GRPC.ResponseStatus
	})

	return result
}

func observedHTTPStatus(
	c *model.HTTPServiceObservedCall,
) int {
	if c.ResponseStatusCode == nil {
		return 0
	}

	return *c.ResponseStatusCode
}
