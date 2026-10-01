package grpc

import (
	"context"
	nativeerrors "errors"
	"net"
	"testing"
	"time"

	ggrpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

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

	logger, err := log.NewSilentLogger()
	if err != nil {
		t.Fatalf(
			"log.NewSilentLogger() error = %v",
			err,
		)
	}

	service, err := NewRealGRPCService(
		logger,
		&model.ServiceSpec{
			ID:         "upstream",
			GRPCTarget: upstreamListener.Addr().String(),
		},
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
