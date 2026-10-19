package call

import (
	"net/http"
	"testing"
)

func TestCallSize(t *testing.T) {
	tests := []struct {
		name string
		call Call
		want int64
	}{
		{
			name: "empty call",
			call: Call{},
			want: 0,
		},
		{
			name: "identifiers only",
			call: Call{
				TestExecutionID: "exec",
				ServiceID:       "service",
			},
			want: 11,
		},
		{
			name: "empty HTTP call",
			call: Call{
				HTTP: &HTTPCall{},
			},
			want: 0,
		},
		{
			name: "HTTP request and response",
			call: Call{
				TestExecutionID: "e",
				ServiceID:       "s",
				HTTP: &HTTPCall{
					Request: HTTPRequest{
						Method: "GET",
						Path:   "/p",
						Query: map[string][]string{
							"q": {"ab", "c"},
						},
						Headers: http.Header{
							"X-Test": {"v"},
						},
						Body:         []byte("abc"),
						BodyComplete: true,
					},
					Response: HTTPResponse{
						StatusCode: http.StatusOK,
						Headers: http.Header{
							"X-Reply": {"ok"},
						},
						Body: []byte("done"),
					},
				},
			},
			want: 34,
		},
		{
			name: "empty gRPC call",
			call: Call{
				GRPC: &GRPCCall{},
			},
			want: 0,
		},
		{
			name: "gRPC request and response",
			call: Call{
				GRPC: &GRPCCall{
					Request: GRPCRequest{
						Service: "svc",
						Method:  "Do",
						Metadata: map[string][]string{
							"a": {"one", "two"},
						},
						Message: []byte{0, 1, 2},
					},
					Response: GRPCResponse{
						Status: "OK",
						Metadata: map[string][]string{
							"b": {"x"},
						},
						Message: []byte{3, 4},
					},
				},
			},
			want: 21,
		},
		{
			name: "HTTP and gRPC populated",
			call: Call{
				HTTP: &HTTPCall{
					Request: HTTPRequest{
						Body: []byte("abc"),
					},
				},
				GRPC: &GRPCCall{
					Response: GRPCResponse{
						Message: []byte{1, 2},
					},
				},
			},
			want: 5,
		},
		{
			name: "metadata with empty values",
			call: Call{
				GRPC: &GRPCCall{
					Request: GRPCRequest{
						Metadata: map[string][]string{
							"key":   {},
							"empty": {""},
						},
					},
				},
			},
			want: 8,
		},
		{
			name: "truncation flags do not affect size",
			call: Call{
				HTTP: &HTTPCall{
					Request: HTTPRequest{
						Body:          []byte("abc"),
						BodyTruncated: true,
						BodyComplete:  false,
					},
					Response: HTTPResponse{
						Body:          []byte("12345"),
						BodyTruncated: true,
					},
				},
			},
			want: 8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.call.Size(); got != tt.want {
				t.Errorf(
					"Call.Size() = %d, want %d",
					got,
					tt.want,
				)
			}
		})
	}
}
