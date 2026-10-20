package grpc

import (
	"context"
	nativeerrors "errors"
	"net"
	"strings"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
	ggrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/log"
)

func TestRawCodecName(t *testing.T) {
	codec := rawCodec{}

	if actual := codec.Name(); actual != "proto" {
		t.Errorf(
			"expected name %q, got %q",
			"proto",
			actual,
		)
	}
}

func TestRawCodecMarshal(t *testing.T) {
	codec := rawCodec{}

	tests := []struct {
		name     string
		value    any
		expected []byte
	}{
		{
			name: "value",
			value: rawMessage{
				1,
				2,
				3,
			},
			expected: []byte{
				1,
				2,
				3,
			},
		},
		{
			name: "pointer",
			value: func() *rawMessage {
				message := rawMessage{
					4,
					5,
					6,
				}

				return &message
			}(),
			expected: []byte{
				4,
				5,
				6,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := codec.Marshal(tt.value)
			if err != nil {
				t.Fatalf(
					"Marshal() error = %v",
					err,
				)
			}

			if string(actual) != string(tt.expected) {
				t.Errorf(
					"expected %v, got %v",
					tt.expected,
					actual,
				)
			}
		})
	}
}

func TestRawCodecMarshalUnsupportedType(t *testing.T) {
	codec := rawCodec{}

	_, err := codec.Marshal("message")
	if err == nil {
		t.Fatal("expected error")
	}

	if !nativeerrors.Is(
		err,
		errors.ErrUnsupportedGRPCMessageType,
	) {
		t.Errorf(
			"expected error %v, got %v",
			errors.ErrUnsupportedGRPCMessageType,
			err,
		)
	}
}

func TestRawCodecUnmarshal(t *testing.T) {
	codec := rawCodec{}

	message := rawMessage{
		9,
		9,
		9,
		9,
	}

	data := []byte{
		1,
		2,
		3,
	}

	if err := codec.Unmarshal(
		data,
		&message,
	); err != nil {
		t.Fatalf(
			"Unmarshal() error = %v",
			err,
		)
	}

	if string(message) != string(data) {
		t.Errorf(
			"expected %v, got %v",
			data,
			message,
		)
	}

	// rawCodec must not retain gRPC's input buffer.
	data[0] = 100

	if message[0] != 1 {
		t.Errorf(
			"expected copied message, got %v",
			message,
		)
	}
}

func TestRawCodecUnmarshalUnsupportedType(t *testing.T) {
	codec := rawCodec{}

	err := codec.Unmarshal(
		[]byte{1, 2, 3},
		new(string),
	)
	if err == nil {
		t.Fatal("expected error")
	}

	if !nativeerrors.Is(
		err,
		errors.ErrUnsupportedGRPCMessageType,
	) {
		t.Errorf(
			"expected error %v, got %v",
			errors.ErrUnsupportedGRPCMessageType,
			err,
		)
	}
}

func TestRealGRPCServiceMountCanceledContext(t *testing.T) {
	service := &RealGRPCService{}

	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	err := service.Mount(ctx)

	if !nativeerrors.Is(err, context.Canceled) {
		t.Errorf(
			"expected %v, got %v",
			context.Canceled,
			err,
		)
	}

	if service.server != nil {
		t.Error("expected server not to be created")
	}

	if service.connection != nil {
		t.Error("expected connection not to be created")
	}
}

func TestRealGRPCServiceMountAddressResolutionError(t *testing.T) {
	ctrl := gomock.NewController(t)

	addressResolver := NewMockaddressResolver(ctrl)

	resolveErr := nativeerrors.New("cannot resolve target")

	addressResolver.EXPECT().
		ResolveGRPCTarget("gateway-host:9000").
		Return("", resolveErr)

	service := &RealGRPCService{
		spec: &model.ServiceSpec{
			Target: "gateway-host:9000",
		},
		addressResolver: addressResolver,
	}

	err := service.Mount(context.Background())

	if !nativeerrors.Is(err, resolveErr) {
		t.Errorf(
			"expected %v, got %v",
			resolveErr,
			err,
		)
	}

	if service.server != nil {
		t.Error("expected server not to be created")
	}

	if service.connection != nil {
		t.Error("expected connection not to be created")
	}
}

func TestRealGRPCServiceUnmountBeforeMount(t *testing.T) {
	service := &RealGRPCService{}

	if err := service.Unmount(
		context.Background(),
	); err != nil {
		t.Errorf(
			"Unmount() error = %v",
			err,
		)
	}
}

func TestRealGRPCServiceProxy(t *testing.T) {
	listenerConfig := &net.ListenConfig{}
	upstreamListener, err := listenerConfig.Listen(
		t.Context(),
		"tcp",
		"127.0.0.1:0",
	)
	if err != nil {
		t.Fatalf(
			"net.Listen() error = %v",
			err,
		)
	}

	upstreamRequest := make(
		chan rawMessage,
		1,
	)
	upstreamMetadata := make(
		chan metadata.MD,
		1,
	)

	upstream := ggrpc.NewServer(
		ggrpc.ForceServerCodec(rawCodec{}),
		ggrpc.UnknownServiceHandler(
			func(
				_ any,
				stream ggrpc.ServerStream,
			) error {
				var request rawMessage

				if err := stream.RecvMsg(
					&request,
				); err != nil {
					return err
				}

				upstreamRequest <- append(
					rawMessage(nil),
					request...,
				)

				md, _ := metadata.FromIncomingContext(
					stream.Context(),
				)

				upstreamMetadata <- md.Copy()

				if err := stream.SendHeader(
					metadata.Pairs(
						"x-upstream-header",
						"header-value",
					),
				); err != nil {
					return err
				}

				stream.SetTrailer(
					metadata.Pairs(
						"x-upstream-trailer",
						"trailer-value",
					),
				)

				response := rawMessage(
					"response payload",
				)

				return stream.SendMsg(
					&response,
				)
			},
		),
	)

	go func() {
		_ = upstream.Serve(
			upstreamListener,
		)
	}()

	defer upstream.Stop()

	ctrl := gomock.NewController(t)

	resolver := NewMockaddressResolver(ctrl)

	resolver.EXPECT().
		ResolveGRPCTarget("gateway-host:9000").
		Return(upstreamListener.Addr().String(), nil)

	logger, err := log.NewSilentLogger()
	if err != nil {
		t.Fatalf(
			"log.NewSilentLogger() error = %v",
			err,
		)
	}

	service, err := NewRealGRPCService(
		&RealGRPCServiceConfig{
			MaxMessageSize:  4 << 20,
			MaxMetadataSize: 64 << 10,
		},
		logger,
		&model.ServiceSpec{
			ID:     "upstream",
			Target: "gateway-host:9000",
		},
		resolver,
	)
	if err != nil {
		t.Fatalf(
			"NewRealGRPCService() error = %v",
			err,
		)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	if err := service.Mount(ctx); err != nil {
		t.Fatalf(
			"Mount() error = %v",
			err,
		)
	}

	defer func() {
		ctx, cancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cancel()

		if err := service.Unmount(ctx); err != nil {
			t.Errorf(
				"Unmount() error = %v",
				err,
			)
		}
	}()

	connection, err := ggrpc.NewClient(
		service.GetRuntimeAddress(),
		ggrpc.WithTransportCredentials(
			insecure.NewCredentials(),
		),
		ggrpc.WithDefaultCallOptions(
			ggrpc.ForceCodec(rawCodec{}),
		),
	)
	if err != nil {
		t.Fatalf(
			"grpc.NewClient() error = %v",
			err,
		)
	}
	defer connection.Close()

	request := rawMessage(
		"request payload",
	)

	var response rawMessage
	var responseHeader metadata.MD
	var responseTrailer metadata.MD

	callContext := metadata.NewOutgoingContext(
		ctx,
		metadata.Pairs(
			"x-test-metadata",
			"metadata-value",
		),
	)

	if err := connection.Invoke(
		callContext,
		"/users.v1.UserService/GetUser",
		&request,
		&response,
		ggrpc.Header(&responseHeader),
		ggrpc.Trailer(&responseTrailer),
	); err != nil {
		t.Fatalf(
			"Invoke() error = %v",
			err,
		)
	}

	if actual := string(response); actual != "response payload" {
		t.Errorf(
			"expected response %q, got %q",
			"response payload",
			actual,
		)
	}

	select {
	case actual := <-upstreamRequest:
		if string(actual) != "request payload" {
			t.Errorf(
				"expected upstream request %q, got %q",
				"request payload",
				string(actual),
			)
		}

	case <-ctx.Done():
		t.Fatal(
			"timed out waiting for upstream request",
		)
	}

	select {
	case actual := <-upstreamMetadata:
		values := actual.Get(
			"x-test-metadata",
		)

		if len(values) != 1 ||
			values[0] != "metadata-value" {
			t.Errorf(
				"expected forwarded metadata %q, got %v",
				"metadata-value",
				values,
			)
		}

	case <-ctx.Done():
		t.Fatal(
			"timed out waiting for upstream metadata",
		)
	}

	header := responseHeader.Get(
		"x-upstream-header",
	)

	if len(header) != 1 ||
		header[0] != "header-value" {
		t.Errorf(
			"expected response header %q, got %v",
			"header-value",
			header,
		)
	}

	trailer := responseTrailer.Get(
		"x-upstream-trailer",
	)

	if len(trailer) != 1 ||
		trailer[0] != "trailer-value" {
		t.Errorf(
			"expected response trailer %q, got %v",
			"trailer-value",
			trailer,
		)
	}
}

func TestRealGRPCServiceProxyErrorMetadata(t *testing.T) {
	const (
		logicalTarget = "gateway-host:9000"
		fullMethod    = "/users.v1.UserService/GetUser"
	)

	listener, err := (&net.ListenConfig{}).Listen(
		t.Context(),
		"tcp",
		"127.0.0.1:0",
	)
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}

	upstream := ggrpc.NewServer(
		ggrpc.ForceServerCodec(rawCodec{}),
		ggrpc.UnknownServiceHandler(
			func(_ any, stream ggrpc.ServerStream) error {
				var request rawMessage

				if err := stream.RecvMsg(&request); err != nil {
					return err
				}

				if string(request) != "request payload" {
					t.Errorf(
						"upstream request = %q, want %q",
						request,
						"request payload",
					)
				}

				if err := stream.SendHeader(
					metadata.Pairs(
						"x-request-id",
						"abc123",
					),
				); err != nil {
					return err
				}

				stream.SetTrailer(
					metadata.Pairs(
						"x-error-code",
						"ACCESS_DENIED",
					),
				)

				return status.Error(
					codes.PermissionDenied,
					"access denied",
				)
			},
		),
	)

	go func() {
		_ = upstream.Serve(listener)
	}()

	defer upstream.Stop()

	ctrl := gomock.NewController(t)

	resolver := NewMockaddressResolver(ctrl)

	resolver.EXPECT().
		ResolveGRPCTarget(logicalTarget).
		Return(listener.Addr().String(), nil)

	logger, err := log.NewSilentLogger()
	if err != nil {
		t.Fatalf("NewSilentLogger() error = %v", err)
	}

	service, err := NewRealGRPCService(
		&RealGRPCServiceConfig{
			MaxMessageSize:  4 << 20,
			MaxMetadataSize: 64 << 10,
		},
		logger,
		&model.ServiceSpec{
			ID:     "upstream",
			Target: logicalTarget,
		},
		resolver,
	)
	if err != nil {
		t.Fatalf("NewRealGRPCService() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	if err := service.Mount(ctx); err != nil {
		t.Fatalf("Mount() error = %v", err)
	}

	defer func() {
		unmountCtx, unmountCancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer unmountCancel()

		if err := service.Unmount(unmountCtx); err != nil {
			t.Errorf("Unmount() error = %v", err)
		}
	}()

	connection, err := ggrpc.NewClient(
		service.GetRuntimeAddress(),
		ggrpc.WithTransportCredentials(
			insecure.NewCredentials(),
		),
		ggrpc.WithDefaultCallOptions(
			ggrpc.ForceCodec(rawCodec{}),
		),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer connection.Close()

	request := rawMessage("request payload")

	var response rawMessage
	var responseHeader metadata.MD
	var responseTrailer metadata.MD

	err = connection.Invoke(
		ctx,
		fullMethod,
		&request,
		&response,
		ggrpc.Header(&responseHeader),
		ggrpc.Trailer(&responseTrailer),
	)

	if code := status.Code(err); code != codes.PermissionDenied {
		t.Errorf(
			"status = %s, want %s (error: %v)",
			code,
			codes.PermissionDenied,
			err,
		)
	}

	if got := status.Convert(err).Message(); got != "access denied" {
		t.Errorf(
			"status message = %q, want %q",
			got,
			"access denied",
		)
	}

	if values := responseHeader.Get("x-request-id"); len(values) != 1 ||
		values[0] != "abc123" {
		t.Errorf(
			"response header = %v, want [abc123]",
			values,
		)
	}

	if values := responseTrailer.Get("x-error-code"); len(values) != 1 ||
		values[0] != "ACCESS_DENIED" {
		t.Errorf(
			"response trailer = %v, want [ACCESS_DENIED]",
			values,
		)
	}

	if len(response) != 0 {
		t.Errorf(
			"expected empty response message, got %v",
			response,
		)
	}
}

func TestRealGRPCServiceMessageSizeLimit(t *testing.T) {
	const limit = 5

	tests := []struct {
		name     string
		request  rawMessage
		response rawMessage
		wantCode codes.Code
	}{
		{
			name:     "exactly at limit",
			request:  rawMessage{1, 2, 3, 4, 5},
			response: rawMessage{6, 7, 8, 9, 10},
			wantCode: codes.OK,
		},
		{
			name:     "request exceeds limit",
			request:  rawMessage{1, 2, 3, 4, 5, 6},
			response: rawMessage{7},
			wantCode: codes.ResourceExhausted,
		},
		{
			name:     "response exceeds limit",
			request:  rawMessage{1},
			response: rawMessage{2, 3, 4, 5, 6, 7},
			wantCode: codes.ResourceExhausted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstreamListener, err := (&net.ListenConfig{}).Listen(
				t.Context(),
				"tcp",
				"127.0.0.1:0",
			)
			if err != nil {
				t.Fatalf("Listen() error = %v", err)
			}

			upstream := ggrpc.NewServer(
				ggrpc.ForceServerCodec(rawCodec{}),
				ggrpc.UnknownServiceHandler(
					func(_ any, stream ggrpc.ServerStream) error {
						var request rawMessage

						if err := stream.RecvMsg(&request); err != nil {
							return err
						}

						if string(request) != string(tt.request) {
							t.Errorf(
								"upstream request = %v, want %v",
								request,
								tt.request,
							)
						}

						return stream.SendMsg(&tt.response)
					},
				),
			)

			go func() {
				_ = upstream.Serve(upstreamListener)
			}()
			defer upstream.Stop()

			ctrl := gomock.NewController(t)
			resolver := NewMockaddressResolver(ctrl)

			const logicalTarget = "gateway-host:9000"

			resolver.EXPECT().
				ResolveGRPCTarget(logicalTarget).
				Return(upstreamListener.Addr().String(), nil)

			logger, err := log.NewSilentLogger()
			if err != nil {
				t.Fatalf("NewSilentLogger() error = %v", err)
			}

			service, err := NewRealGRPCService(
				&RealGRPCServiceConfig{
					MaxMessageSize:  limit,
					MaxMetadataSize: 64 << 10,
				},
				logger,
				&model.ServiceSpec{
					ID:     "upstream",
					Target: logicalTarget,
				},
				resolver,
			)
			if err != nil {
				t.Fatalf("NewRealGRPCService() error = %v", err)
			}

			if err := service.Mount(t.Context()); err != nil {
				t.Fatalf("Mount() error = %v", err)
			}

			defer func() {
				ctx, cancel := context.WithTimeout(
					context.Background(),
					5*time.Second,
				)
				defer cancel()

				if err := service.Unmount(ctx); err != nil {
					t.Errorf("Unmount() error = %v", err)
				}
			}()

			connection, err := ggrpc.NewClient(
				service.GetRuntimeAddress(),
				ggrpc.WithTransportCredentials(
					insecure.NewCredentials(),
				),
				ggrpc.WithDefaultCallOptions(
					ggrpc.ForceCodec(rawCodec{}),
				),
			)
			if err != nil {
				t.Fatalf("NewClient() error = %v", err)
			}
			defer connection.Close()

			var response rawMessage

			err = connection.Invoke(
				t.Context(),
				"/users.v1.UserService/GetUser",
				&tt.request,
				&response,
			)

			if code := status.Code(err); code != tt.wantCode {
				t.Fatalf(
					"status = %s, want %s (error: %v)",
					code,
					tt.wantCode,
					err,
				)
			}

			if tt.wantCode == codes.OK {
				if string(response) != string(tt.response) {
					t.Errorf(
						"response = %v, want %v",
						response,
						tt.response,
					)
				}
			}
		})
	}
}

func TestRealGRPCServiceMetadataSizeLimit(t *testing.T) {
	const (
		limit         uint32 = 1024
		logicalTarget        = "gateway-host:9000"
		fullMethod           = "/users.v1.UserService/GetUser"
	)

	tests := []struct {
		name             string
		requestMetadata  string
		responseMetadata string
		wantSuccess      bool
		wantUpstream     bool
	}{
		{
			name:             "within limit",
			requestMetadata:  "small",
			responseMetadata: "small",
			wantSuccess:      true,
			wantUpstream:     true,
		},
		{
			name:            "request exceeds limit",
			requestMetadata: strings.Repeat("a", 4096),
		},
		{
			name:             "response exceeds limit",
			requestMetadata:  "small",
			responseMetadata: strings.Repeat("b", 4096),
			wantUpstream:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listener, err := (&net.ListenConfig{}).Listen(
				t.Context(),
				"tcp",
				"127.0.0.1:0",
			)
			if err != nil {
				t.Fatalf("Listen() error = %v", err)
			}

			upstreamCalled := make(chan struct{}, 1)

			upstream := ggrpc.NewServer(
				ggrpc.ForceServerCodec(rawCodec{}),
				ggrpc.UnknownServiceHandler(
					func(_ any, stream ggrpc.ServerStream) error {
						var request rawMessage

						if err := stream.RecvMsg(&request); err != nil {
							return err
						}

						upstreamCalled <- struct{}{}

						if err := stream.SendHeader(
							metadata.Pairs(
								"x-response",
								tt.responseMetadata,
							),
						); err != nil {
							return err
						}

						response := rawMessage("response payload")
						return stream.SendMsg(&response)
					},
				),
			)

			go func() {
				_ = upstream.Serve(listener)
			}()
			defer upstream.Stop()

			ctrl := gomock.NewController(t)
			resolver := NewMockaddressResolver(ctrl)

			resolver.EXPECT().
				ResolveGRPCTarget(logicalTarget).
				Return(listener.Addr().String(), nil)

			logger, err := log.NewSilentLogger()
			if err != nil {
				t.Fatalf("NewSilentLogger() error = %v", err)
			}

			service, err := NewRealGRPCService(
				&RealGRPCServiceConfig{
					MaxMessageSize:  4 << 20,
					MaxMetadataSize: limit,
				},
				logger,
				&model.ServiceSpec{
					ID:     "upstream",
					Target: logicalTarget,
				},
				resolver,
			)
			if err != nil {
				t.Fatalf("NewRealGRPCService() error = %v", err)
			}

			if err := service.Mount(t.Context()); err != nil {
				t.Fatalf("Mount() error = %v", err)
			}

			defer func() {
				ctx, cancel := context.WithTimeout(
					context.Background(),
					5*time.Second,
				)
				defer cancel()

				if err := service.Unmount(ctx); err != nil {
					t.Errorf("Unmount() error = %v", err)
				}
			}()

			connection, err := ggrpc.NewClient(
				service.GetRuntimeAddress(),
				ggrpc.WithTransportCredentials(
					insecure.NewCredentials(),
				),
				ggrpc.WithDefaultCallOptions(
					ggrpc.ForceCodec(rawCodec{}),
				),
			)
			if err != nil {
				t.Fatalf("NewClient() error = %v", err)
			}
			defer connection.Close()

			ctx, cancel := context.WithTimeout(
				t.Context(),
				5*time.Second,
			)
			defer cancel()

			ctx = metadata.NewOutgoingContext(
				ctx,
				metadata.Pairs(
					"x-request",
					tt.requestMetadata,
				),
			)

			request := rawMessage("request payload")
			var response rawMessage
			var header metadata.MD

			err = connection.Invoke(
				ctx,
				fullMethod,
				&request,
				&response,
				ggrpc.Header(&header),
			)

			if tt.wantSuccess {
				if err != nil {
					t.Fatalf("Invoke() error = %v", err)
				}

				if string(response) != "response payload" {
					t.Errorf(
						"response = %q, want %q",
						response,
						"response payload",
					)
				}

				values := header.Get("x-response")
				if len(values) != 1 || values[0] != tt.responseMetadata {
					t.Errorf(
						"response metadata = %v, want [%s]",
						values,
						tt.responseMetadata,
					)
				}
			} else if err == nil {
				t.Fatal("expected metadata size limit error")
			}

			select {
			case <-upstreamCalled:
				if !tt.wantUpstream {
					t.Error("upstream unexpectedly invoked")
				}

			default:
				if tt.wantUpstream {
					t.Error("upstream not invoked")
				}
			}
		})
	}
}
