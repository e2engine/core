package call

import (
	"net/http"
	"reflect"
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

func TestCallClone(t *testing.T) {
	tests := []struct {
		name string
		call Call
	}{
		{
			name: "empty call",
			call: Call{},
		},
		{
			name: "identifiers and flags",
			call: Call{
				TestExecutionID:    "execution",
				ServiceID:          "service",
				MockFixtureMatched: true,
			},
		},
		{
			name: "HTTP call",
			call: Call{
				HTTP: &HTTPCall{
					Request: HTTPRequest{
						Method: "POST",
						Path:   "/test",
						Query: map[string][]string{
							"q": {"one", "two"},
						},
						Headers: http.Header{
							"X-Request": {"value"},
						},
						Body:          []byte("request"),
						BodyTruncated: true,
						BodyComplete:  true,
					},
					Response: HTTPResponse{
						StatusCode: http.StatusCreated,
						Headers: http.Header{
							"X-Response": {"value"},
						},
						Body:          []byte("response"),
						BodyTruncated: true,
					},
				},
			},
		},
		{
			name: "gRPC call",
			call: Call{
				GRPC: &GRPCCall{
					Request: GRPCRequest{
						Service: "service",
						Method:  "Method",
						Metadata: map[string][]string{
							"request": {"value"},
						},
						Message: []byte("request"),
					},
					Response: GRPCResponse{
						Status: "OK",
						Metadata: map[string][]string{
							"response": {"value"},
						},
						Message: []byte("response"),
					},
				},
			},
		},
		{
			name: "both protocols",
			call: Call{
				HTTP: &HTTPCall{
					Request: HTTPRequest{
						Body: []byte("http"),
					},
				},
				GRPC: &GRPCCall{
					Request: GRPCRequest{
						Message: []byte("grpc"),
					},
				},
			},
		},
		{
			name: "nil and empty collections",
			call: Call{
				HTTP: &HTTPCall{
					Request: HTTPRequest{
						Query: map[string][]string{
							"nil":   nil,
							"empty": {},
						},
						Headers: http.Header{
							"Nil":   nil,
							"Empty": {},
						},
						Body: []byte{},
					},
					Response: HTTPResponse{
						Headers: http.Header{},
						Body:    []byte{},
					},
				},
				GRPC: &GRPCCall{
					Request: GRPCRequest{
						Metadata: map[string][]string{
							"empty": {},
						},
						Message: []byte{},
					},
					Response: GRPCResponse{
						Metadata: map[string][]string{},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clone := tt.call.Clone()

			if !reflect.DeepEqual(clone, tt.call) {
				t.Fatalf(
					"Clone() = %#v, want %#v",
					clone,
					tt.call,
				)
			}

			if clone.Size() != tt.call.Size() {
				t.Errorf(
					"cloned size = %d, want %d",
					clone.Size(),
					tt.call.Size(),
				)
			}

			if tt.call.HTTP != nil {
				if clone.HTTP == tt.call.HTTP {
					t.Error("HTTPCall pointer was not cloned")
				}

				if len(clone.HTTP.Request.Body) > 0 {
					clone.HTTP.Request.Body[0] ^= 0xff

					if reflect.DeepEqual(
						clone.HTTP.Request.Body,
						tt.call.HTTP.Request.Body,
					) {
						t.Error("HTTP request body shares storage")
					}
				}

				if len(clone.HTTP.Response.Body) > 0 {
					clone.HTTP.Response.Body[0] ^= 0xff

					if reflect.DeepEqual(
						clone.HTTP.Response.Body,
						tt.call.HTTP.Response.Body,
					) {
						t.Error("HTTP response body shares storage")
					}
				}

				if clone.HTTP.Request.Query != nil {
					clone.HTTP.Request.Query["new"] = []string{"value"}

					if _, exists := tt.call.HTTP.Request.Query["new"]; exists {
						t.Error("HTTP query map shares storage")
					}

					for key, values := range clone.HTTP.Request.Query {
						if key == "new" || len(values) == 0 {
							continue
						}

						values[0] = "changed"

						if tt.call.HTTP.Request.Query[key][0] == "changed" {
							t.Error("HTTP query value slice shares storage")
						}
						break
					}
				}

				if clone.HTTP.Request.Headers != nil {
					clone.HTTP.Request.Headers.Set("X-Added", "new")

					if tt.call.HTTP.Request.Headers.Get("X-Added") != "" {
						t.Error("HTTP request headers share storage")
					}

					values := clone.HTTP.Request.Headers["X-Request"]
					if len(values) > 0 {
						values[0] = "changed"

						if tt.call.HTTP.Request.Headers["X-Request"][0] == "changed" {
							t.Error("HTTP request header values share storage")
						}
					}
				}

				if clone.HTTP.Response.Headers != nil {
					clone.HTTP.Response.Headers.Set("X-Added", "new")

					if tt.call.HTTP.Response.Headers.Get("X-Added") != "" {
						t.Error("HTTP response headers share storage")
					}
				}
			}

			if tt.call.GRPC != nil {
				if clone.GRPC == tt.call.GRPC {
					t.Error("GRPCCall pointer was not cloned")
				}

				if len(clone.GRPC.Request.Message) > 0 {
					clone.GRPC.Request.Message[0] ^= 0xff

					if reflect.DeepEqual(
						clone.GRPC.Request.Message,
						tt.call.GRPC.Request.Message,
					) {
						t.Error("gRPC request message shares storage")
					}
				}

				if len(clone.GRPC.Response.Message) > 0 {
					clone.GRPC.Response.Message[0] ^= 0xff

					if reflect.DeepEqual(
						clone.GRPC.Response.Message,
						tt.call.GRPC.Response.Message,
					) {
						t.Error("gRPC response message shares storage")
					}
				}

				for _, pair := range []struct {
					name     string
					clone    map[string][]string
					original map[string][]string
				}{
					{
						name:     "request",
						clone:    clone.GRPC.Request.Metadata,
						original: tt.call.GRPC.Request.Metadata,
					},
					{
						name:     "response",
						clone:    clone.GRPC.Response.Metadata,
						original: tt.call.GRPC.Response.Metadata,
					},
				} {
					if pair.clone == nil {
						continue
					}

					pair.clone["new"] = []string{"value"}

					if _, exists := pair.original["new"]; exists {
						t.Errorf(
							"gRPC %s metadata map shares storage",
							pair.name,
						)
					}

					for key, values := range pair.clone {
						if key == "new" || len(values) == 0 {
							continue
						}

						values[0] = "changed"

						if pair.original[key][0] == "changed" {
							t.Errorf(
								"gRPC %s metadata values share storage",
								pair.name,
							)
						}
						break
					}
				}
			}
		})
	}
}
