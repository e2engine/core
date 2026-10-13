package reporter

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/e2engine/core/execute"
	"github.com/e2engine/core/execute/call"
	"github.com/e2engine/core/internal/util"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/model/matcher"
)

type Reporter struct {
	cfg      *Config
	testSpec *model.TestSpec
	report   *execute.TestExecutionResult
}

type Config struct {
	DumpPayload bool `json:"dump_payload" yaml:"dump_payload" env:"DUMP_PAYLOAD"`
}

func New(
	cfg *Config,
	testSpec *model.TestSpec,
	executionID, testSuiteExecutionID string,
) *Reporter {
	return &Reporter{
		cfg:      cfg,
		testSpec: testSpec,
		report: &execute.TestExecutionResult{
			ExecutionID:          executionID,
			TestSuiteExecutionID: testSuiteExecutionID,
			Summary:              &model.TestExecutionSummary{},
		},
	}
}

func (r *Reporter) Result() *execute.TestExecutionResult {
	return r.report
}

func (r *Reporter) SetError(err error) {
	r.ensureNotFinalized()

	r.report.Status = model.ExecutionStatusError
	r.report.FinishedAt = time.Now().UTC()
	r.report.Summary.Error = err.Error()
}

func (r *Reporter) Finalize() {
	r.ensureNotFinalized()

	if len(r.report.Summary.Deviations) == 0 {
		r.report.Status = model.ExecutionStatusPassed
	} else {
		r.report.Status = model.ExecutionStatusFailed
	}

	r.report.FinishedAt = time.Now().UTC()
}

func (r *Reporter) ensureNotFinalized() {
	if !r.report.FinishedAt.IsZero() {
		panic("cannot modify report after it has been finalized")
	}
}

func (r *Reporter) SummarizeCalls(calls call.Reader) {
	r.ensureNotFinalized()

	r.report.Summary.Calls = summarizeCalls(
		r.report.ExecutionID,
		calls,
	)
}

func (r *Reporter) AddDeviation(deviation *model.TestExecutionDeviation) {
	if deviation == nil {
		return
	}
	r.ensureNotFinalized()

	r.report.Summary.Deviations = append(r.report.Summary.Deviations, *deviation)
}

const dumpDisabled = "<dump disabled>"

func (r *Reporter) AddPayloadDeviation(deviation *model.PayloadDeviation) {
	if deviation == nil {
		return
	}

	r.ensureNotFinalized()

	d := model.TestExecutionDeviation{
		Field:   deviation.Field,
		Message: deviation.Message,
	}

	switch {
	case deviation.Expected == nil:
	case !r.cfg.DumpPayload:
		d.Expected = dumpDisabled
	default:
		d.Expected = deviation.Expected.String()
	}

	switch {
	case len(deviation.Actual) == 0:
	case !r.cfg.DumpPayload:
		d.Actual = dumpDisabled
	default:
		d.Actual = string(deviation.Actual)
	}

	r.report.Summary.Deviations = append(r.report.Summary.Deviations, d)
}

func (r *Reporter) AddHTTPRequest(req *http.Request) {
	r.ensureNotFinalized()

	request := &model.HTTPRequest{
		Method:  req.Method,
		URL:     req.URL.String(),
		Headers: req.Header.Clone(),
	}

	if r.testSpec.Request.HTTP.Body != "" {
		switch {
		case !r.cfg.DumpPayload:
			request.Body = dumpDisabled
		default:
			request.Body = r.testSpec.Request.HTTP.Body
		}
	}

	if r.testSpec.Request.HTTP.BodyJSON != "" {
		switch {
		case !r.cfg.DumpPayload:
			request.BodyJSON = dumpDisabled
		default:
			request.BodyJSON = string(util.CompactJSON(
				json.RawMessage(r.testSpec.Request.HTTP.BodyJSON),
			))
		}
	}

	r.report.Summary.Request = &model.TestExecutionRequestSummary{
		HTTP: request,
	}
}

func (r *Reporter) ensureHTTPResponseSummary() {
	if r.report.Summary.Response == nil {
		r.report.Summary.Response = &model.TestExecutionResponseSummary{}
	}

	if r.report.Summary.Response.HTTP == nil {
		r.report.Summary.Response.HTTP = &model.HTTPResponse{}
	}
}

func (r *Reporter) AddHTTPResponseStatus(status int) {
	r.ensureNotFinalized()

	r.ensureHTTPResponseSummary()
	r.report.Summary.Response.HTTP.StatusCode = status
}

func (r *Reporter) AddHTTPResponseHeaders(headers http.Header) {
	r.ensureNotFinalized()

	r.ensureHTTPResponseSummary()
	r.report.Summary.Response.HTTP.Headers = selectHeaders(
		headers,
		r.testSpec.Expect.HTTP.Headers,
	)
}

func selectHeaders(
	actual http.Header,
	expected map[string][]matcher.String,
) map[string][]string {
	if len(expected) == 0 {
		return nil
	}

	result := make(
		map[string][]string,
		len(expected),
	)

	for k := range expected {
		values := actual.Values(k)
		if len(values) > 0 {
			result[k] = util.CloneStringSlice(values)
		}
	}

	if len(result) == 0 {
		return nil
	}

	return result
}

func (r *Reporter) AddHTTPResponseBody(body []byte) {
	r.ensureNotFinalized()
	r.ensureHTTPResponseSummary()

	switch {
	case len(body) == 0:
	case !r.cfg.DumpPayload:
		r.report.Summary.Response.HTTP.Body = dumpDisabled
	default:
		r.report.Summary.Response.HTTP.Body = string(body)
	}
}

func (r *Reporter) AddHTTPResponseBodyJSON(body []byte) {
	r.ensureNotFinalized()
	r.ensureHTTPResponseSummary()

	switch {
	case len(body) == 0:
	case !r.cfg.DumpPayload:
		r.report.Summary.Response.HTTP.BodyJSON = dumpDisabled
	default:
		r.report.Summary.Response.HTTP.BodyJSON = string(body)
	}
}

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
		Metadata: util.CloneStringSliceMap(req.Metadata),
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

func (r *Reporter) AddGRPCResponseStatus(status string) {
	r.ensureNotFinalized()

	r.ensureGRPCResponseSummary()
	r.report.Summary.Response.GRPC.Status = status
}

func (r *Reporter) AddGRPCResponseMetadata(metadata map[string][]string) {
	r.ensureNotFinalized()

	r.ensureGRPCResponseSummary()
	r.report.Summary.Response.GRPC.Metadata = selectMetadata(
		metadata,
		r.testSpec.Expect.GRPC.Metadata,
	)
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
		if values, ok := actual[key]; ok {
			result[key] = util.CloneStringSlice(values)
		}
	}

	if len(result) == 0 {
		return nil
	}

	return result
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
