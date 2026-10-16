package http

import (
	"bytes"
	"context"
	"io"
	"net/http"

	e2enginehttp "github.com/e2engine/instrumentation-go/http"

	"github.com/e2engine/core/execute"
	"github.com/e2engine/core/model"
)

type TestExecutor struct {
	client          *http.Client
	evaluator       *Evaluator
	reporter        executionReporter
	addressResolver addressResolver
}

func NewTestExecutor(
	client *http.Client,
	evaluator *Evaluator,
	reporter executionReporter,
	addressResolver addressResolver,
) *TestExecutor {
	return &TestExecutor{
		client:          client,
		evaluator:       evaluator,
		reporter:        reporter,
		addressResolver: addressResolver,
	}
}

func (e *TestExecutor) Execute(
	ctx context.Context,
	job *execute.TestJob,
) error {
	resolvedURL, err := e.addressResolver.ResolveHTTPURL(
		job.Test.Spec.Request.HTTP.URL,
	)
	if err != nil {
		return err
	}

	req, err := buildHTTPRequest(
		ctx,
		resolvedURL,
		job.Test.Spec.Request.HTTP,
		job.ExecutionID,
	)
	if err != nil {
		return err
	}

	e.reporter.AddHTTPRequest(req)

	resp, err := doHTTPRequest(
		e.client,
		req,
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := e.evaluator.EvaluateResponse(
		resp,
		job.Test.Spec.Expect.HTTP,
	); err != nil {
		return err
	}

	return nil
}

func buildHTTPRequest(
	ctx context.Context,
	resolvedURL string,
	spec *model.HTTPRequest,
	testExecutionID string,
) (*http.Request, error) {
	var body io.Reader

	if spec.Body != "" {
		body = bytes.NewReader([]byte(spec.Body))
	} else if spec.BodyJSON != "" {
		body = bytes.NewReader([]byte(spec.BodyJSON))
	}

	req, err := http.NewRequestWithContext(
		ctx,
		spec.Method,
		resolvedURL,
		body,
	)
	if err != nil {
		return nil, err
	}

	query := req.URL.Query()
	for key, values := range spec.Query {
		for _, value := range values {
			query.Add(key, value)
		}
	}

	req.URL.RawQuery = query.Encode()

	for key, values := range spec.Headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	req.Header = e2enginehttp.WithTestExecutionID(req.Header, testExecutionID)

	if spec.BodyJSON != "" &&
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
