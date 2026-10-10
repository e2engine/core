package worker

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/e2engine/core/model"
)

func TestNewSummaryHTTP(t *testing.T) {
	spec := &model.TestSpec{
		Request: model.RequestSpec{
			HTTP: &model.HTTPRequestSpec{
				Method: http.MethodPost,
				URL:    "https://example.com/users",
				Headers: http.Header{
					"Content-Type": {"application/json"},
				},
				Body: `{
					"id": 42
				}`,
			},
		},
		Expect: model.ExpectSpec{
			HTTP: &model.HTTPExpectSpec{
				Status: http.StatusCreated,
				Body: `{
					"status": "created"
				}`,
			},
		},
	}

	summary := newSummary(spec)

	if summary.Request == nil {
		t.Fatal("expected non-nil request summary")
	}

	if summary.Request.HTTP == nil {
		t.Fatal("expected HTTP request summary")
	}

	if summary.Request.GRPC != nil {
		t.Fatal("expected nil gRPC request summary")
	}

	if summary.ExpectedCalls != nil {
		t.Fatal("expected nil expected calls summary")
	}

	if summary.Request.HTTP.Method != http.MethodPost {
		t.Errorf(
			"expected method %q, got %q",
			http.MethodPost,
			summary.Request.HTTP.Method,
		)
	}

	if summary.Request.HTTP.URL != "https://example.com/users" {
		t.Errorf(
			"unexpected URL %q",
			summary.Request.HTTP.URL,
		)
	}

	if summary.Request.HTTP.BodyJSON != `{"id":42}` {
		t.Errorf(
			"expected compact body %q, got %q",
			`{"id":42}`,
			summary.Request.HTTP.BodyJSON,
		)
	}

	spec.Request.HTTP.Headers["Content-Type"][0] = "text/plain"

	if summary.Request.HTTP.Headers["Content-Type"][0] !=
		"application/json" {
		t.Error("expected request headers to be cloned")
	}
}

func TestNewSummaryGRPC(t *testing.T) {
	spec := &model.TestSpec{
		Request: model.RequestSpec{
			GRPC: &model.GRPCRequestSpec{
				Service: "users.v1.UserService",
				Method:  "GetUser",
				Metadata: map[string][]string{
					"x-request-id": {"123"},
				},
				Message: map[string]any{
					"id": "42",
				},
			},
		},
		Expect: model.ExpectSpec{
			GRPC: &model.GRPCExpectSpec{
				Status: "OK",
				Message: map[string]any{
					"name": "Yaroslav",
				},
			},
		},
	}

	summary := newSummary(spec)

	if summary.Request == nil {
		t.Fatal("expected non-nil request summary")
	}

	if summary.Request.GRPC == nil {
		t.Fatal("expected gRPC request summary")
	}

	if summary.Request.HTTP != nil {
		t.Fatal("expected nil HTTP request summary")
	}

	if summary.ExpectedCalls != nil {
		t.Fatal("expected nil expected calls summary")
	}

	if summary.Request.GRPC.Service != "users.v1.UserService" {
		t.Errorf(
			"unexpected service %q",
			summary.Request.GRPC.Service,
		)
	}

	if summary.Request.GRPC.Method != "GetUser" {
		t.Errorf(
			"unexpected method %q",
			summary.Request.GRPC.Method,
		)
	}

	spec.Request.GRPC.Metadata["x-request-id"][0] = "changed"
	spec.Request.GRPC.Message["id"] = "changed"

	if actual := summary.Request.GRPC.Metadata["x-request-id"][0]; actual != "123" {
		t.Error("expected request metadata to be cloned")
	}

	if actual := summary.Request.GRPC.Message["id"]; actual != "42" {
		t.Error("expected request message to be cloned")
	}
}

func TestNewExpectedCallsSummaryEmpty(t *testing.T) {
	if actual := newExpectedCallsSummary(nil); actual != nil {
		t.Errorf(
			"expected nil, got %#v",
			actual,
		)
	}
}

func TestNewExpectedCallsSummary(t *testing.T) {
	count := 2

	expected := []model.CallExpectation{
		{
			ServiceID: "http-service",
			Count:     &count,
			HTTP: &model.HTTPCallExpectation{
				Method: http.MethodPost,
				Path:   "/users",
				Query: url.Values{
					"active": {"true"},
				},
				Headers: http.Header{
					"Content-Type": {"application/json"},
				},
				Body: `{
					"id": 42
				}`,
			},
		},
		{
			ServiceID: "grpc-service",
			GRPC: &model.GRPCCallExpectation{
				Service: "users.v1.UserService",
				Method:  "GetUser",
				Metadata: map[string][]string{
					"x-request-id": {"123"},
				},
				Message: map[string]any{
					"id": "42",
				},
			},
		},
	}

	summary := newExpectedCallsSummary(expected)

	if len(summary) != 2 {
		t.Fatalf(
			"expected 2 summaries, got %d",
			len(summary),
		)
	}

	httpSummary := summary[0]

	if httpSummary.ServiceID != "http-service" {
		t.Errorf(
			"unexpected service ID %q",
			httpSummary.ServiceID,
		)
	}

	if httpSummary.Count != &count &&
		(httpSummary.Count == nil || *httpSummary.Count != count) {
		t.Errorf(
			"unexpected count %#v",
			httpSummary.Count,
		)
	}

	if httpSummary.HTTP == nil {
		t.Fatal("expected HTTP call expectation summary")
	}

	if httpSummary.HTTP.BodyJSON != `{"id":42}` {
		t.Errorf(
			"unexpected HTTP body %q",
			httpSummary.HTTP.BodyJSON,
		)
	}

	grpcSummary := summary[1]

	if grpcSummary.GRPC == nil {
		t.Fatal("expected gRPC call expectation summary")
	}

	if grpcSummary.GRPC.Service != "users.v1.UserService" {
		t.Errorf(
			"unexpected service %q",
			grpcSummary.GRPC.Service,
		)
	}

	expected[0].HTTP.Query["active"][0] = "false"
	expected[0].HTTP.Headers["Content-Type"][0] = "text/plain"
	expected[1].GRPC.Metadata["x-request-id"][0] = "changed"
	expected[1].GRPC.Message["id"] = "changed"

	if httpSummary.HTTP.Query["active"][0] != "true" {
		t.Error("expected query to be cloned")
	}

	if httpSummary.HTTP.Headers["Content-Type"][0] !=
		"application/json" {
		t.Error("expected headers to be cloned")
	}

	if grpcSummary.GRPC.Metadata["x-request-id"][0] != "123" {
		t.Error("expected metadata to be cloned")
	}

	if grpcSummary.GRPC.Message["id"] != "42" {
		t.Error("expected message to be cloned")
	}
}
