package reporter

import (
	"strings"
	"time"

	"github.com/e2engine/core/execute"
	"github.com/e2engine/core/execute/call"
	"github.com/e2engine/core/model"
)

type Reporter struct {
	cfg              *Config
	testSpec         *model.TestSpec
	report           *execute.TestExecutionResult
	redactedHeaders  map[string]struct{}
	redactedMetadata map[string]struct{}
}

func New(
	cfg *Config,
	testSpec *model.TestSpec,
	executionID, testSuiteExecutionID string,
) *Reporter {
	const (
		authorization      = "authorization"
		proxyAuthorization = "proxy-authorization"
		cookie             = "cookie"
		setCookie          = "set-cookie"
		xAPIKey            = "x-api-key"
	)
	redactedHeaders := map[string]struct{}{
		authorization:      {},
		proxyAuthorization: {},
		cookie:             {},
		setCookie:          {},
		xAPIKey:            {},
	}
	for _, h := range cfg.RedactHeaders {
		redactedHeaders[strings.ToLower(strings.TrimSpace(h))] = struct{}{}
	}

	redactedMetadata := map[string]struct{}{
		authorization:      {},
		proxyAuthorization: {},
		cookie:             {},
		setCookie:          {},
		xAPIKey:            {},
	}
	for _, m := range cfg.RedactMetadata {
		redactedMetadata[strings.ToLower(strings.TrimSpace(m))] = struct{}{}
	}

	return &Reporter{
		cfg:      cfg,
		testSpec: testSpec,
		report: &execute.TestExecutionResult{
			ExecutionID:          executionID,
			TestSuiteExecutionID: testSuiteExecutionID,
			Summary:              &model.TestExecutionSummary{},
		},
		redactedHeaders:  redactedHeaders,
		redactedMetadata: redactedMetadata,
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

const (
	dumpDisabled = "<dump disabled>"
	redacted     = "<redacted>"
)

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
