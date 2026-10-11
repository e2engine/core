package worker

import (
	"encoding/json"

	"github.com/e2engine/core/internal/util"
	"github.com/e2engine/core/model"
)

func newSummary(spec *model.TestSpec) *model.TestExecutionSummary {
	if spec.Request.HTTP != nil {
		return &model.TestExecutionSummary{
			Request: &model.TestExecutionRequestSummary{
				HTTP: &model.HTTPRequest{
					Method:   spec.Request.HTTP.Method,
					URL:      spec.Request.HTTP.URL,
					Query:    util.CloneStringSliceMap(spec.Request.HTTP.Query),
					Headers:  util.CloneHTTPHeader(spec.Request.HTTP.Headers),
					Body:     spec.Request.HTTP.Body,
					BodyJSON: util.CompactJSON(json.RawMessage(spec.Request.HTTP.BodyJSON)),
				},
			},
		}
	}

	return &model.TestExecutionSummary{
		Request: &model.TestExecutionRequestSummary{
			GRPC: &model.GRPCRequest{
				Target:   spec.Request.GRPC.Target,
				Service:  spec.Request.GRPC.Service,
				Method:   spec.Request.GRPC.Method,
				Metadata: util.CloneStringSliceMap(spec.Request.GRPC.Metadata),
				Message:  util.CloneMap(spec.Request.GRPC.Message),
			},
		},
	}
}
