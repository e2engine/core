package reporter

import (
	"strings"

	"github.com/e2engine/core/internal/util"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/model/matcher"
)

func (r *Reporter) ensureGRPCResponseSummary() {
	if r.report.Summary.Response == nil {
		r.report.Summary.Response = &model.TestExecutionResponseSummary{}
	}

	if r.report.Summary.Response.GRPC == nil {
		r.report.Summary.Response.GRPC = &model.GRPCResponse{}
	}
}

func (r *Reporter) AddGRPCRequest(req *model.GRPCRequest) {
	r.ensureNotFinalized()

	request := &model.GRPCRequest{
		Target:   req.Target,
		Service:  req.Service,
		Method:   req.Method,
		Metadata: r.redactMetadata(req.Metadata),
	}

	if req.Message != nil {
		if r.cfg.DumpPayload {
			request.Message = util.CloneMap(req.Message)
		} else {
			request.Message = map[string]any{
				dumpDisabled: nil,
			}
		}
	}

	r.report.Summary.Request = &model.TestExecutionRequestSummary{
		GRPC: request,
	}
}

func (r *Reporter) AddMetadataDeviation(
	key string,
	deviation *model.TestExecutionDeviation,
) {
	if deviation == nil {
		return
	}

	if !r.cfg.SkipRedactingHeadersMetadata {
		if _, ok := r.redactedMetadata[strings.ToLower(key)]; ok {
			redactedDeviation := *deviation
			redactedDeviation.Expected = redacted
			redactedDeviation.Actual = redacted

			r.AddDeviation(&redactedDeviation)
			return
		}
	}

	r.AddDeviation(deviation)
}

func (r *Reporter) AddGRPCResponseStatus(status string) {
	r.ensureNotFinalized()

	r.ensureGRPCResponseSummary()
	r.report.Summary.Response.GRPC.Status = status
}

func (r *Reporter) AddGRPCResponseMetadata(metadata map[string][]string) {
	r.ensureNotFinalized()

	r.ensureGRPCResponseSummary()
	r.report.Summary.Response.GRPC.Metadata = r.redactMetadata(
		selectMetadata(
			metadata,
			r.testSpec.Expect.GRPC.Metadata,
		),
	)
}

func (r *Reporter) AddGRPCResponseMessage(message map[string]any) {
	r.ensureNotFinalized()
	r.ensureGRPCResponseSummary()

	switch {
	case len(message) == 0:
		r.report.Summary.Response.GRPC.Message = map[string]any{"<none>": nil}
	case !r.cfg.DumpPayload:
		r.report.Summary.Response.GRPC.Message = map[string]any{dumpDisabled: nil}
	default:
		m := util.CloneMap(message)
		r.report.Summary.Response.GRPC.Message = m
	}
}

func selectMetadata(
	actual map[string][]string,
	expected map[string][]matcher.String,
) map[string][]string {
	if len(expected) == 0 {
		return nil
	}

	result := make(
		map[string][]string,
		len(expected),
	)

	for key := range expected {
		values, ok := actual[strings.ToLower(key)]
		if ok {
			result[key] = util.CloneStringSlice(values)
		}
	}

	if len(result) == 0 {
		return nil
	}

	return result
}

func (r *Reporter) redactMetadata(
	md map[string][]string,
) map[string][]string {
	if r.cfg.SkipRedactingHeadersMetadata {
		return util.CloneStringSliceMap(md)
	}

	out := make(map[string][]string, len(md))

	for key, values := range md {
		if _, ok := r.redactedMetadata[strings.ToLower(key)]; ok {
			out[key] = []string{redacted}
		} else {
			out[key] = util.CloneStringSlice(values)
		}
	}

	return out
}
