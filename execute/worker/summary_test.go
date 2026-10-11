package worker

import (
	"net/http"
	"testing"

	"github.com/e2engine/core/model"
)

func TestNewSummaryHTTPBody(t *testing.T) {
	spec := &model.TestSpec{
		Request: model.RequestSpec{
			HTTP: &model.HTTPRequest{
				Method: http.MethodPost,
				URL:    "https://example.com/users",
				Query: map[string][]string{
					"active": {"true"},
				},
				Headers: http.Header{
					"Content-Type": {
						"text/plain",
					},
				},
				Body: `{
					"id": 42
				}`,
			},
		},
	}

	summary := newSummary(spec)

	if summary.Request == nil {
		t.Fatal(
			"expected non-nil request summary",
		)
	}

	if summary.Request.HTTP == nil {
		t.Fatal(
			"expected HTTP request summary",
		)
	}

	if summary.Request.GRPC != nil {
		t.Fatal(
			"expected nil gRPC request summary",
		)
	}

	actual := summary.Request.HTTP

	if actual.Method != http.MethodPost {
		t.Errorf(
			"expected method %q, got %q",
			http.MethodPost,
			actual.Method,
		)
	}

	if actual.URL != "https://example.com/users" {
		t.Errorf(
			"unexpected URL %q",
			actual.URL,
		)
	}

	if actual.Query["active"][0] != "true" {
		t.Errorf(
			"unexpected query %#v",
			actual.Query,
		)
	}

	if actual.Headers["Content-Type"][0] !=
		"text/plain" {
		t.Errorf(
			"unexpected headers %#v",
			actual.Headers,
		)
	}

	if actual.Body != `{
					"id": 42
				}` {
		t.Errorf(
			"unexpected body %q",
			actual.Body,
		)
	}

	if actual.BodyJSON != "" {
		t.Errorf(
			"expected empty JSON body, got %q",
			actual.BodyJSON,
		)
	}

	spec.Request.HTTP.Query["active"][0] = "false"
	spec.Request.HTTP.Headers["Content-Type"][0] =
		"application/json"
	spec.Request.HTTP.Body = "changed"

	if actual.Query["active"][0] != "true" {
		t.Error(
			"expected request query to be cloned",
		)
	}

	if actual.Headers["Content-Type"][0] !=
		"text/plain" {
		t.Error(
			"expected request headers to be cloned",
		)
	}

	if actual.Body == "changed" {
		t.Error(
			"expected request body snapshot to be independent",
		)
	}
}

func TestNewSummaryHTTPBodyJSON(t *testing.T) {
	spec := &model.TestSpec{
		Request: model.RequestSpec{
			HTTP: &model.HTTPRequest{
				Method: http.MethodPost,
				URL:    "https://example.com/users",
				BodyJSON: `{
					"id": 42,
					"name": "Yaroslav"
				}`,
			},
		},
	}

	summary := newSummary(spec)

	if summary.Request == nil {
		t.Fatal(
			"expected non-nil request summary",
		)
	}

	if summary.Request.HTTP == nil {
		t.Fatal(
			"expected HTTP request summary",
		)
	}

	actual := summary.Request.HTTP

	if actual.Body != "" {
		t.Errorf(
			"expected empty body, got %q",
			actual.Body,
		)
	}

	if actual.BodyJSON !=
		`{"id":42,"name":"Yaroslav"}` {
		t.Errorf(
			"expected compact JSON body %q, got %q",
			`{"id":42,"name":"Yaroslav"}`,
			actual.BodyJSON,
		)
	}

	spec.Request.HTTP.BodyJSON = `{"changed":true}`

	if actual.BodyJSON !=
		`{"id":42,"name":"Yaroslav"}` {
		t.Error(
			"expected request JSON body snapshot to be independent",
		)
	}
}

func TestNewSummaryGRPC(t *testing.T) {
	spec := &model.TestSpec{
		Request: model.RequestSpec{
			GRPC: &model.GRPCRequest{
				Target:  "127.0.0.1:50051",
				Service: "users.v1.UserService",
				Method:  "GetUser",
				Metadata: map[string][]string{
					"x-request-id": {
						"123",
					},
				},
				Message: map[string]any{
					"id": "42",
				},
			},
		},
	}

	summary := newSummary(spec)

	if summary.Request == nil {
		t.Fatal(
			"expected non-nil request summary",
		)
	}

	if summary.Request.GRPC == nil {
		t.Fatal(
			"expected gRPC request summary",
		)
	}

	if summary.Request.HTTP != nil {
		t.Fatal(
			"expected nil HTTP request summary",
		)
	}

	actual := summary.Request.GRPC

	if actual.Target != "127.0.0.1:50051" {
		t.Errorf(
			"unexpected target %q",
			actual.Target,
		)
	}

	if actual.Service !=
		"users.v1.UserService" {
		t.Errorf(
			"unexpected service %q",
			actual.Service,
		)
	}

	if actual.Method != "GetUser" {
		t.Errorf(
			"unexpected method %q",
			actual.Method,
		)
	}

	if actual.Metadata["x-request-id"][0] !=
		"123" {
		t.Errorf(
			"unexpected metadata %#v",
			actual.Metadata,
		)
	}

	if actual.Message["id"] != "42" {
		t.Errorf(
			"unexpected message %#v",
			actual.Message,
		)
	}

	spec.Request.GRPC.Target = "changed:50051"
	spec.Request.GRPC.Service = "changed.Service"
	spec.Request.GRPC.Method = "Changed"
	spec.Request.GRPC.Metadata["x-request-id"][0] =
		"changed"
	spec.Request.GRPC.Message["id"] = "changed"

	if actual.Target != "127.0.0.1:50051" {
		t.Error(
			"expected target snapshot to be independent",
		)
	}

	if actual.Service !=
		"users.v1.UserService" {
		t.Error(
			"expected service snapshot to be independent",
		)
	}

	if actual.Method != "GetUser" {
		t.Error(
			"expected method snapshot to be independent",
		)
	}

	if actual.Metadata["x-request-id"][0] !=
		"123" {
		t.Error(
			"expected request metadata to be cloned",
		)
	}

	if actual.Message["id"] != "42" {
		t.Error(
			"expected request message to be cloned",
		)
	}
}
