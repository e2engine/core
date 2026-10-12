package reporter

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/e2engine/core/execute"
	"github.com/e2engine/core/execute/call"
	"github.com/e2engine/core/internal/util"
	"github.com/e2engine/core/model"
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

func (r *Reporter) AddPayloadDeviation(deviation *model.TestExecutionDeviation) {
	if deviation == nil {
		return
	}

	d := &model.TestExecutionDeviation{
		Field:   deviation.Field,
		Message: deviation.Message,
	}

	if r.cfg.DumpPayload {
		d.Actual = deviation.Actual
		d.Expected = deviation.Expected
	}

	r.AddDeviation(d)
}

func (r *Reporter) AddHTTPRequest(req *http.Request) {
	r.ensureNotFinalized()

	request := &model.HTTPRequest{
		Method:  req.Method,
		URL:     req.URL.String(),
		Headers: req.Header.Clone(),
	}

	if r.cfg.DumpPayload {
		request.Body = r.testSpec.Request.HTTP.Body

		if r.testSpec.Request.HTTP.BodyJSON != "" {
			request.BodyJSON = util.CompactJSON(
				json.RawMessage(r.testSpec.Request.HTTP.BodyJSON),
			)
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
	expected map[string][]string,
) map[string][]string {
	if len(expected) == 0 {
		return nil
	}

	result := make(
		map[string][]string,
		len(expected),
	)

	for key := range expected {
		values := actual.Values(key)
		if len(values) > 0 {
			result[key] = util.CloneStringSlice(values)
		}
	}

	if len(result) == 0 {
		return nil
	}

	return result
}

func (r *Reporter) AddHTTPResponseBody(body string) {
	if !r.cfg.DumpPayload {
		return
	}

	r.ensureNotFinalized()

	r.ensureHTTPResponseSummary()
	r.report.Summary.Response.HTTP.Body = &body
}

func (r *Reporter) AddHTTPResponseBodyJSON(body string) {
	if !r.cfg.DumpPayload {
		return
	}

	r.ensureNotFinalized()

	r.ensureHTTPResponseSummary()
	r.report.Summary.Response.HTTP.BodyJSON = &body
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

	if r.cfg.DumpPayload {
		request.Message = util.CloneMap(req.Message)
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
	expected map[string][]string,
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
	if !r.cfg.DumpPayload {
		return
	}

	r.ensureNotFinalized()

	r.ensureGRPCResponseSummary()

	m := util.CloneMap(message)
	r.report.Summary.Response.GRPC.Message = &m
}
