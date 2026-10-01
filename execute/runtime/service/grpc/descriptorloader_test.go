package grpc

import (
	"bytes"
	"context"
	nativeerrors "errors"
	"io"
	"net/http"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/e2engine/core/pkg/errors"
)

func TestDescriptorURL(t *testing.T) {
	tests := []struct {
		name      string
		bufModule string
		expected  string
		wantErr   bool
	}{
		{
			name:      "valid module",
			bufModule: "buf.build/acme/users",
			expected:  "https://buf.build/acme/users/descriptor/main",
		},
		{
			name:      "empty module",
			bufModule: "",
			wantErr:   true,
		},
		{
			name:      "missing prefix",
			bufModule: "acme/users",
			wantErr:   true,
		},
		{
			name:      "missing owner",
			bufModule: "buf.build//users",
			wantErr:   true,
		},
		{
			name:      "missing module",
			bufModule: "buf.build/acme/",
			wantErr:   true,
		},
		{
			name:      "owner only",
			bufModule: "buf.build/acme",
			wantErr:   true,
		},
		{
			name:      "too many path components",
			bufModule: "buf.build/acme/users/v1",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := descriptorURL(tt.bufModule)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}

				if !nativeerrors.Is(
					err,
					errors.ErrInvalidBufModule,
				) {
					t.Errorf(
						"expected error %v, got %v",
						errors.ErrInvalidBufModule,
						err,
					)
				}

				return
			}

			if err != nil {
				t.Fatalf(
					"descriptorURL() error = %v",
					err,
				)
			}

			if actual != tt.expected {
				t.Errorf(
					"expected URL %q, got %q",
					tt.expected,
					actual,
				)
			}
		})
	}
}

func TestBufDescriptorLoaderLoad(t *testing.T) {
	descriptorSet := &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{
			{
				Name:    proto.String("users.proto"),
				Package: proto.String("users.v1"),
			},
		},
	}

	descriptorData, err := proto.Marshal(descriptorSet)
	if err != nil {
		t.Fatalf(
			"cannot marshal descriptor set: %v",
			err,
		)
	}

	tests := []struct {
		name        string
		token       string
		status      int
		body        []byte
		check       func(t *testing.T, req *http.Request)
		expectedErr error
	}{
		{
			name:   "descriptor loaded",
			status: http.StatusOK,
			body:   descriptorData,
			check: func(
				t *testing.T,
				req *http.Request,
			) {
				t.Helper()

				if req.Method != http.MethodGet {
					t.Errorf(
						"expected method %q, got %q",
						http.MethodGet,
						req.Method,
					)
				}

				expectedURL :=
					"https://buf.build/acme/users/descriptor/main"

				if req.URL.String() != expectedURL {
					t.Errorf(
						"expected URL %q, got %q",
						expectedURL,
						req.URL.String(),
					)
				}

				if actual := req.Header.Get("Accept"); actual != "application/octet-stream" {
					t.Errorf(
						"expected Accept header %q, got %q",
						"application/octet-stream",
						actual,
					)
				}

				if actual := req.Header.Get("Authorization"); actual != "" {
					t.Errorf(
						"expected empty Authorization header, got %q",
						actual,
					)
				}
			},
		},
		{
			name:   "authorization token",
			token:  "test-token",
			status: http.StatusOK,
			body:   descriptorData,
			check: func(
				t *testing.T,
				req *http.Request,
			) {
				t.Helper()

				if actual := req.Header.Get("Authorization"); actual != "Bearer test-token" {
					t.Errorf(
						"expected Authorization header %q, got %q",
						"Bearer test-token",
						actual,
					)
				}
			},
		},
		{
			name:        "bad request",
			status:      http.StatusBadRequest,
			body:        []byte("invalid module"),
			expectedErr: errors.ErrCannotLoadGRPCDescriptor,
		},
		{
			name:        "unauthorized",
			status:      http.StatusUnauthorized,
			body:        []byte("unauthorized"),
			expectedErr: errors.ErrCannotLoadGRPCDescriptor,
		},
		{
			name:        "server error",
			status:      http.StatusInternalServerError,
			body:        []byte("internal error"),
			expectedErr: errors.ErrCannotLoadGRPCDescriptor,
		},
		{
			name:        "invalid descriptor",
			status:      http.StatusOK,
			body:        []byte("not a protobuf descriptor"),
			expectedErr: errors.ErrCannotLoadGRPCDescriptor,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &http.Client{
				Transport: roundTripperFunc(
					func(
						req *http.Request,
					) (*http.Response, error) {
						if tt.check != nil {
							tt.check(t, req)
						}

						return &http.Response{
							StatusCode: tt.status,
							Header:     make(http.Header),
							Body: io.NopCloser(
								bytes.NewReader(tt.body),
							),
							Request: req,
						}, nil
					},
				),
			}

			loader := NewBufDescriptorLoader(
				client,
				tt.token,
			)

			actual, err := loader.Load(
				context.Background(),
				"buf.build/acme/users",
			)

			if tt.expectedErr != nil {
				if err == nil {
					t.Fatal("expected error")
				}

				if !nativeerrors.Is(
					err,
					tt.expectedErr,
				) {
					t.Errorf(
						"expected error %v, got %v",
						tt.expectedErr,
						err,
					)
				}

				return
			}

			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			if !proto.Equal(actual, descriptorSet) {
				t.Errorf(
					"expected descriptor set %v, got %v",
					descriptorSet,
					actual,
				)
			}
		})
	}
}

func TestBufDescriptorLoaderLoadInvalidModule(t *testing.T) {
	loader := NewBufDescriptorLoader(
		&http.Client{
			Transport: roundTripperFunc(
				func(
					req *http.Request,
				) (*http.Response, error) {
					t.Fatal(
						"HTTP request must not be executed for invalid module",
					)

					return nil, nil
				},
			),
		},
		"",
	)

	_, err := loader.Load(
		context.Background(),
		"invalid",
	)
	if err == nil {
		t.Fatal("expected error")
	}

	if !nativeerrors.Is(
		err,
		errors.ErrInvalidBufModule,
	) {
		t.Errorf(
			"expected error %v, got %v",
			errors.ErrInvalidBufModule,
			err,
		)
	}
}

func TestBufDescriptorLoaderLoadTransportError(t *testing.T) {
	expectedErr := nativeerrors.New("transport error")

	client := &http.Client{
		Transport: roundTripperFunc(
			func(
				req *http.Request,
			) (*http.Response, error) {
				return nil, expectedErr
			},
		),
	}

	loader := NewBufDescriptorLoader(client, "")

	_, err := loader.Load(
		context.Background(),
		"buf.build/acme/users",
	)
	if err == nil {
		t.Fatal("expected error")
	}

	if !nativeerrors.Is(
		err,
		errors.ErrCannotLoadGRPCDescriptor,
	) {
		t.Errorf(
			"expected error %v, got %v",
			errors.ErrCannotLoadGRPCDescriptor,
			err,
		)
	}
}

func TestBufDescriptorLoaderLoadReadError(t *testing.T) {
	expectedErr := nativeerrors.New("read error")

	client := &http.Client{
		Transport: roundTripperFunc(
			func(
				req *http.Request,
			) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: &errorReader{
						err: expectedErr,
					},
					Request: req,
				}, nil
			},
		),
	}

	loader := NewBufDescriptorLoader(client, "")

	_, err := loader.Load(
		context.Background(),
		"buf.build/acme/users",
	)
	if err == nil {
		t.Fatal("expected error")
	}

	if !nativeerrors.Is(
		err,
		errors.ErrCannotLoadGRPCDescriptor,
	) {
		t.Errorf(
			"expected error %v, got %v",
			errors.ErrCannotLoadGRPCDescriptor,
			err,
		)
	}
}

func TestBufDescriptorLoaderLoadCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	loader := NewBufDescriptorLoader(
		&http.Client{
			Transport: roundTripperFunc(
				func(
					req *http.Request,
				) (*http.Response, error) {
					if err := req.Context().Err(); err != nil {
						return nil, err
					}

					t.Fatal("expected canceled request context")

					return nil, nil
				},
			),
		},
		"",
	)

	_, err := loader.Load(
		ctx,
		"buf.build/acme/users",
	)
	if err == nil {
		t.Fatal("expected error")
	}

	if !nativeerrors.Is(
		err,
		errors.ErrCannotLoadGRPCDescriptor,
	) {
		t.Errorf(
			"expected error %v, got %v",
			errors.ErrCannotLoadGRPCDescriptor,
			err,
		)
	}
}

func TestNewBufDescriptorLoaderNilClient(t *testing.T) {
	loader := NewBufDescriptorLoader(nil, "")

	if loader.client != http.DefaultClient {
		t.Error(
			"expected nil client to use http.DefaultClient",
		)
	}
}

type roundTripperFunc func(
	req *http.Request,
) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(
	req *http.Request,
) (*http.Response, error) {
	return f(req)
}

type errorReader struct {
	err error
}

func (r *errorReader) Read(
	_ []byte,
) (int, error) {
	return 0, r.err
}

func (r *errorReader) Close() error {
	return nil
}
