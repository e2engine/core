package http

import (
	"bytes"
	"context"
	"io"
	"net/http"

	"github.com/e2engine/core/execute"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/keys"
)

type TestExecutor struct {
	client    *http.Client
	evaluator *Evaluator
}

func NewTestExecutor(
	client *http.Client,
	evaluator *Evaluator,
) *TestExecutor {
	return &TestExecutor{
		client:    client,
		evaluator: evaluator,
	}
}

func (e *TestExecutor) Execute(
	ctx context.Context,
	job *execute.TestJob,
	result *execute.TestExecutionResult,
) error {
	req, err := buildHTTPRequest(
		ctx,
		job.Test.Spec.Request.HTTP,
		job.ExecutionID,
	)
	if err != nil {
		return err
	}

	resp, err := doHTTPRequest(
		e.client,
		req,
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := e.evaluator.EvaluateResponse(
		ctx,
		resp,
		job.Test.Spec.Expect.HTTP,
		result,
	); err != nil {
		return err
	}

	return nil
}

func buildHTTPRequest(
	ctx context.Context,
	spec *model.HTTPRequestSpec,
	testExecutionID string,
) (*http.Request, error) {
	var body io.Reader

	if spec.Body != "" {
		body = bytes.NewReader([]byte(spec.Body))
	}

	req, err := http.NewRequestWithContext(
		ctx,
		spec.Method,
		spec.URL,
		body,
	)
	if err != nil {
		return nil, err
	}

	query := req.URL.Query()
	for key, value := range spec.Query {
		query.Set(key, value)
	}
	req.URL.RawQuery = query.Encode()

	for key, values := range spec.Headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	req.Header.Set(string(keys.E2EngineTestExecutionID), testExecutionID)

	if spec.Body != "" &&
		req.Header.Get("Content-Type") == "" {
		req.Header.Set(
			"Content-Type",
			"application/json",
		)
	}

	return req, nil
}

func doHTTPRequest(
	client *http.Client,
	req *http.Request,
) (*http.Response, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
