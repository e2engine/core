package reporter

import (
	"testing"

	"github.com/e2engine/core/execute/call"
	"github.com/e2engine/core/model"
)

func TestSummarizeCalls(t *testing.T) {
	tests := []struct {
		name string

		calls []call.Call

		expected []model.ServiceObservedCalls
	}{
		{
			name: "no calls",
		},
		{
			name: "aggregates identical HTTP calls",
			calls: []call.Call{
				{
					TestExecutionID: "execution-1",
					ServiceID:       "users",
					HTTP: &call.HTTPCall{
						Request: call.HTTPRequest{
							Method: "GET",
							Path:   "/users",
						},
						Response: call.HTTPResponse{
							StatusCode: 200,
						},
					},
				},
				{
					TestExecutionID: "execution-1",
					ServiceID:       "users",
					HTTP: &call.HTTPCall{
						Request: call.HTTPRequest{
							Method: "GET",
							Path:   "/users",
						},
						Response: call.HTTPResponse{
							StatusCode: 200,
						},
					},
				},
			},
			expected: []model.ServiceObservedCalls{
				{
					ServiceID: "users",
					Count:     2,
					HTTP: &model.HTTPServiceObservedCall{
						Method:             "GET",
						Path:               "/users",
						ResponseStatusCode: intPtrObserved(200),
					},
				},
			},
		},
		{
			name: "separates HTTP calls by response status",
			calls: []call.Call{
				{
					TestExecutionID: "execution-1",
					ServiceID:       "users",
					HTTP: &call.HTTPCall{
						Request: call.HTTPRequest{
							Method: "GET",
							Path:   "/users",
						},
						Response: call.HTTPResponse{
							StatusCode: 200,
						},
					},
				},
				{
					TestExecutionID: "execution-1",
					ServiceID:       "users",
					HTTP: &call.HTTPCall{
						Request: call.HTTPRequest{
							Method: "GET",
							Path:   "/users",
						},
						Response: call.HTTPResponse{
							StatusCode: 500,
						},
					},
				},
			},
			expected: []model.ServiceObservedCalls{
				{
					ServiceID: "users",
					Count:     1,
					HTTP: &model.HTTPServiceObservedCall{
						Method:             "GET",
						Path:               "/users",
						ResponseStatusCode: intPtrObserved(200),
					},
				},
				{
					ServiceID: "users",
					Count:     1,
					HTTP: &model.HTTPServiceObservedCall{
						Method:             "GET",
						Path:               "/users",
						ResponseStatusCode: intPtrObserved(500),
					},
				},
			},
		},
		{
			name: "separates HTTP calls by request identity",
			calls: []call.Call{
				{
					TestExecutionID: "execution-1",
					ServiceID:       "users",
					HTTP: &call.HTTPCall{
						Request: call.HTTPRequest{
							Method: "GET",
							Path:   "/users",
						},
						Response: call.HTTPResponse{
							StatusCode: 200,
						},
					},
				},
				{
					TestExecutionID: "execution-1",
					ServiceID:       "users",
					HTTP: &call.HTTPCall{
						Request: call.HTTPRequest{
							Method: "POST",
							Path:   "/users",
						},
						Response: call.HTTPResponse{
							StatusCode: 200,
						},
					},
				},
				{
					TestExecutionID: "execution-1",
					ServiceID:       "users",
					HTTP: &call.HTTPCall{
						Request: call.HTTPRequest{
							Method: "GET",
							Path:   "/health",
						},
						Response: call.HTTPResponse{
							StatusCode: 200,
						},
					},
				},
			},
			expected: []model.ServiceObservedCalls{
				{
					ServiceID: "users",
					Count:     1,
					HTTP: &model.HTTPServiceObservedCall{
						Method:             "GET",
						Path:               "/health",
						ResponseStatusCode: intPtrObserved(200),
					},
				},
				{
					ServiceID: "users",
					Count:     1,
					HTTP: &model.HTTPServiceObservedCall{
						Method:             "GET",
						Path:               "/users",
						ResponseStatusCode: intPtrObserved(200),
					},
				},
				{
					ServiceID: "users",
					Count:     1,
					HTTP: &model.HTTPServiceObservedCall{
						Method:             "POST",
						Path:               "/users",
						ResponseStatusCode: intPtrObserved(200),
					},
				},
			},
		},
		{
			name: "aggregates identical gRPC calls",
			calls: []call.Call{
				{
					TestExecutionID: "execution-1",
					ServiceID:       "users",
					GRPC: &call.GRPCCall{
						Request: call.GRPCRequest{
							Service: "users.UserService",
							Method:  "Create",
						},
						Response: call.GRPCResponse{
							Status: "OK",
						},
					},
				},
				{
					TestExecutionID: "execution-1",
					ServiceID:       "users",
					GRPC: &call.GRPCCall{
						Request: call.GRPCRequest{
							Service: "users.UserService",
							Method:  "Create",
						},
						Response: call.GRPCResponse{
							Status: "OK",
						},
					},
				},
			},
			expected: []model.ServiceObservedCalls{
				{
					ServiceID: "users",
					Count:     2,
					GRPC: &model.GRPCServiceObservedCall{
						Service:        "users.UserService",
						Method:         "Create",
						ResponseStatus: "OK",
					},
				},
			},
		},
		{
			name: "separates gRPC calls by response status",
			calls: []call.Call{
				{
					TestExecutionID: "execution-1",
					ServiceID:       "users",
					GRPC: &call.GRPCCall{
						Request: call.GRPCRequest{
							Service: "users.UserService",
							Method:  "Create",
						},
						Response: call.GRPCResponse{
							Status: "Internal",
						},
					},
				},
				{
					TestExecutionID: "execution-1",
					ServiceID:       "users",
					GRPC: &call.GRPCCall{
						Request: call.GRPCRequest{
							Service: "users.UserService",
							Method:  "Create",
						},
						Response: call.GRPCResponse{
							Status: "OK",
						},
					},
				},
			},
			expected: []model.ServiceObservedCalls{
				{
					ServiceID: "users",
					Count:     1,
					GRPC: &model.GRPCServiceObservedCall{
						Service:        "users.UserService",
						Method:         "Create",
						ResponseStatus: "Internal",
					},
				},
				{
					ServiceID: "users",
					Count:     1,
					GRPC: &model.GRPCServiceObservedCall{
						Service:        "users.UserService",
						Method:         "Create",
						ResponseStatus: "OK",
					},
				},
			},
		},
		{
			name: "separates gRPC calls by request identity",
			calls: []call.Call{
				{
					TestExecutionID: "execution-1",
					ServiceID:       "users",
					GRPC: &call.GRPCCall{
						Request: call.GRPCRequest{
							Service: "users.UserService",
							Method:  "Get",
						},
						Response: call.GRPCResponse{
							Status: "OK",
						},
					},
				},
				{
					TestExecutionID: "execution-1",
					ServiceID:       "users",
					GRPC: &call.GRPCCall{
						Request: call.GRPCRequest{
							Service: "users.UserService",
							Method:  "Create",
						},
						Response: call.GRPCResponse{
							Status: "OK",
						},
					},
				},
				{
					TestExecutionID: "execution-1",
					ServiceID:       "users",
					GRPC: &call.GRPCCall{
						Request: call.GRPCRequest{
							Service: "users.AdminService",
							Method:  "Create",
						},
						Response: call.GRPCResponse{
							Status: "OK",
						},
					},
				},
			},
			expected: []model.ServiceObservedCalls{
				{
					ServiceID: "users",
					Count:     1,
					GRPC: &model.GRPCServiceObservedCall{
						Service:        "users.AdminService",
						Method:         "Create",
						ResponseStatus: "OK",
					},
				},
				{
					ServiceID: "users",
					Count:     1,
					GRPC: &model.GRPCServiceObservedCall{
						Service:        "users.UserService",
						Method:         "Create",
						ResponseStatus: "OK",
					},
				},
				{
					ServiceID: "users",
					Count:     1,
					GRPC: &model.GRPCServiceObservedCall{
						Service:        "users.UserService",
						Method:         "Get",
						ResponseStatus: "OK",
					},
				},
			},
		},
		{
			name: "separates calls by service ID",
			calls: []call.Call{
				{
					TestExecutionID: "execution-1",
					ServiceID:       "service-b",
					HTTP: &call.HTTPCall{
						Request: call.HTTPRequest{
							Method: "GET",
							Path:   "/health",
						},
						Response: call.HTTPResponse{
							StatusCode: 200,
						},
					},
				},
				{
					TestExecutionID: "execution-1",
					ServiceID:       "service-a",
					HTTP: &call.HTTPCall{
						Request: call.HTTPRequest{
							Method: "GET",
							Path:   "/health",
						},
						Response: call.HTTPResponse{
							StatusCode: 200,
						},
					},
				},
			},
			expected: []model.ServiceObservedCalls{
				{
					ServiceID: "service-a",
					Count:     1,
					HTTP: &model.HTTPServiceObservedCall{
						Method:             "GET",
						Path:               "/health",
						ResponseStatusCode: intPtrObserved(200),
					},
				},
				{
					ServiceID: "service-b",
					Count:     1,
					HTTP: &model.HTTPServiceObservedCall{
						Method:             "GET",
						Path:               "/health",
						ResponseStatusCode: intPtrObserved(200),
					},
				},
			},
		},
		{
			name: "HTTP sorts before gRPC for same service",
			calls: []call.Call{
				{
					TestExecutionID: "execution-1",
					ServiceID:       "service-1",
					GRPC: &call.GRPCCall{
						Request: call.GRPCRequest{
							Service: "users.UserService",
							Method:  "Create",
						},
						Response: call.GRPCResponse{
							Status: "OK",
						},
					},
				},
				{
					TestExecutionID: "execution-1",
					ServiceID:       "service-1",
					HTTP: &call.HTTPCall{
						Request: call.HTTPRequest{
							Method: "GET",
							Path:   "/health",
						},
						Response: call.HTTPResponse{
							StatusCode: 200,
						},
					},
				},
			},
			expected: []model.ServiceObservedCalls{
				{
					ServiceID: "service-1",
					Count:     1,
					HTTP: &model.HTTPServiceObservedCall{
						Method:             "GET",
						Path:               "/health",
						ResponseStatusCode: intPtrObserved(200),
					},
				},
				{
					ServiceID: "service-1",
					Count:     1,
					GRPC: &model.GRPCServiceObservedCall{
						Service:        "users.UserService",
						Method:         "Create",
						ResponseStatus: "OK",
					},
				},
			},
		},
		{
			name: "ignores calls from another execution",
			calls: []call.Call{
				{
					TestExecutionID: "execution-2",
					ServiceID:       "users",
					HTTP: &call.HTTPCall{
						Request: call.HTTPRequest{
							Method: "GET",
							Path:   "/users",
						},
						Response: call.HTTPResponse{
							StatusCode: 200,
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &call.Store{}

			for _, actual := range tt.calls {
				store.Record(actual)
			}

			actual := summarizeCalls(
				"execution-1",
				store,
			)

			assertObservedCalls(
				t,
				actual,
				tt.expected,
			)
		})
	}
}

func TestSummarizeCallsDoesNotExposeCapturedData(
	t *testing.T,
) {
	store := &call.Store{}

	store.Record(call.Call{
		TestExecutionID: "execution-1",
		ServiceID:       "service-1",
		HTTP: &call.HTTPCall{
			Request: call.HTTPRequest{
				Method: "POST",
				Path:   "/events",
				Query: map[string][]string{
					"token": {"secret-query"},
				},
				Headers: map[string][]string{
					"Authorization": {
						"Bearer secret-token",
					},
				},
				Body: []byte(`{"secret":"request"}`),
			},
			Response: call.HTTPResponse{
				StatusCode: 204,
				Headers: map[string][]string{
					"Set-Cookie": {
						"secret-cookie",
					},
				},
				Body: []byte(`{"secret":"response"}`),
			},
		},
	})

	store.Record(call.Call{
		TestExecutionID: "execution-1",
		ServiceID:       "grpc-service",
		GRPC: &call.GRPCCall{
			Request: call.GRPCRequest{
				Service: "users.UserService",
				Method:  "Create",
				Metadata: map[string][]string{
					"authorization": {
						"Bearer secret-token",
					},
				},
				Message: []byte(`{"secret": "request"}`),
			},
			Response: call.GRPCResponse{
				Status: "OK",
				Metadata: map[string][]string{
					"set-cookie": {
						"secret-cookie",
					},
				},
				Message: []byte(`{"secret": "response"}`),
			},
		},
	})

	actual := summarizeCalls(
		"execution-1",
		store,
	)

	expected := []model.ServiceObservedCalls{
		{
			ServiceID: "grpc-service",
			Count:     1,
			GRPC: &model.GRPCServiceObservedCall{
				Service:        "users.UserService",
				Method:         "Create",
				ResponseStatus: "OK",
			},
		},
		{
			ServiceID: "service-1",
			Count:     1,
			HTTP: &model.HTTPServiceObservedCall{
				Method:             "POST",
				Path:               "/events",
				ResponseStatusCode: intPtrObserved(204),
			},
		},
	}

	assertObservedCalls(
		t,
		actual,
		expected,
	)
}

func TestObservedHTTPStatus(t *testing.T) {
	status := 404

	tests := []struct {
		name string

		call *model.HTTPServiceObservedCall

		expected int
	}{
		{
			name: "status",
			call: &model.HTTPServiceObservedCall{
				ResponseStatusCode: &status,
			},
			expected: 404,
		},
		{
			name:     "nil status",
			call:     &model.HTTPServiceObservedCall{},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := observedHTTPStatus(
				tt.call,
			)

			if actual != tt.expected {
				t.Errorf(
					"expected status %d, got %d",
					tt.expected,
					actual,
				)
			}
		})
	}
}

func assertObservedCalls(
	t *testing.T,
	actual []model.ServiceObservedCalls,
	expected []model.ServiceObservedCalls,
) {
	t.Helper()

	if len(actual) != len(expected) {
		t.Fatalf(
			"expected %d observed calls, got %d: %+v",
			len(expected),
			len(actual),
			actual,
		)
	}

	for i := range expected {
		if actual[i].ServiceID != expected[i].ServiceID {
			t.Errorf(
				"call %d: expected service ID %q, got %q",
				i,
				expected[i].ServiceID,
				actual[i].ServiceID,
			)
		}

		if actual[i].Count != expected[i].Count {
			t.Errorf(
				"call %d: expected count %d, got %d",
				i,
				expected[i].Count,
				actual[i].Count,
			)
		}

		assertObservedHTTPCall(
			t,
			i,
			actual[i].HTTP,
			expected[i].HTTP,
		)

		assertObservedGRPCCall(
			t,
			i,
			actual[i].GRPC,
			expected[i].GRPC,
		)
	}
}

func assertObservedHTTPCall(
	t *testing.T,
	index int,
	actual *model.HTTPServiceObservedCall,
	expected *model.HTTPServiceObservedCall,
) {
	t.Helper()

	if expected == nil {
		if actual != nil {
			t.Errorf(
				"call %d: expected no HTTP call, got %+v",
				index,
				actual,
			)
		}

		return
	}

	if actual == nil {
		t.Errorf(
			"call %d: expected HTTP call %+v, got nil",
			index,
			expected,
		)

		return
	}

	if actual.Method != expected.Method {
		t.Errorf(
			"call %d: expected HTTP method %q, got %q",
			index,
			expected.Method,
			actual.Method,
		)
	}

	if actual.Path != expected.Path {
		t.Errorf(
			"call %d: expected HTTP path %q, got %q",
			index,
			expected.Path,
			actual.Path,
		)
	}

	actualStatus := observedHTTPStatus(actual)
	expectedStatus := observedHTTPStatus(expected)

	if actualStatus != expectedStatus {
		t.Errorf(
			"call %d: expected HTTP status %d, got %d",
			index,
			expectedStatus,
			actualStatus,
		)
	}
}

func assertObservedGRPCCall(
	t *testing.T,
	index int,
	actual *model.GRPCServiceObservedCall,
	expected *model.GRPCServiceObservedCall,
) {
	t.Helper()

	if expected == nil {
		if actual != nil {
			t.Errorf(
				"call %d: expected no gRPC call, got %+v",
				index,
				actual,
			)
		}

		return
	}

	if actual == nil {
		t.Errorf(
			"call %d: expected gRPC call %+v, got nil",
			index,
			expected,
		)

		return
	}

	if actual.Service != expected.Service {
		t.Errorf(
			"call %d: expected gRPC service %q, got %q",
			index,
			expected.Service,
			actual.Service,
		)
	}

	if actual.Method != expected.Method {
		t.Errorf(
			"call %d: expected gRPC method %q, got %q",
			index,
			expected.Method,
			actual.Method,
		)
	}

	if actual.ResponseStatus != expected.ResponseStatus {
		t.Errorf(
			"call %d: expected gRPC status %q, got %q",
			index,
			expected.ResponseStatus,
			actual.ResponseStatus,
		)
	}
}

func intPtrObserved(value int) *int {
	return &value
}
