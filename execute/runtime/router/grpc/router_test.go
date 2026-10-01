package grpc

import (
	"context"
	"errors"
	"net"
	"testing"

	grpcpkg "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/e2engine/core/execute/call"
	"github.com/e2engine/core/execute/runtime"
	"github.com/e2engine/core/pkg/keys"
	"github.com/e2engine/core/pkg/log"
)

func TestRawCodec(t *testing.T) {
	codec := rawCodec{}

	if codec.Name() != "proto" {
		t.Errorf(
			"expected codec name proto, got %s",
			codec.Name(),
		)
	}

	message := rawMessage{1, 2, 3}

	data, err := codec.Marshal(message)
	if err != nil {
		t.Fatalf("Marshal(rawMessage) error = %v", err)
	}

	if string(data) != string(message) {
		t.Errorf(
			"expected %v, got %v",
			message,
			data,
		)
	}

	data, err = codec.Marshal(&message)
	if err != nil {
		t.Fatalf("Marshal(*rawMessage) error = %v", err)
	}

	if string(data) != string(message) {
		t.Errorf(
			"expected %v, got %v",
			message,
			data,
		)
	}

	if _, err := codec.Marshal("invalid"); err == nil {
		t.Fatal("expected unsupported Marshal type error")
	}

	var decoded rawMessage

	if err := codec.Unmarshal(
		[]byte{4, 5, 6},
		&decoded,
	); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if string(decoded) != string([]byte{4, 5, 6}) {
		t.Errorf(
			"expected [4 5 6], got %v",
			decoded,
		)
	}

	if err := codec.Unmarshal(
		[]byte{1},
		new(string),
	); err == nil {
		t.Fatal("expected unsupported Unmarshal type error")
	}
}

func TestCloneMetadata(t *testing.T) {
	source := metadata.Pairs(
		"x-test", "first",
		"x-test", "second",
		string(keys.E2EngineTestExecutionID), "execution-1",
	)

	got := cloneMetadata(source)

	values := got["x-test"]
	if len(values) != 2 ||
		values[0] != "first" ||
		values[1] != "second" {
		t.Errorf(
			"unexpected cloned metadata: %#v",
			got,
		)
	}

	if _, ok := got[string(keys.E2EngineTestExecutionID)]; ok {
		t.Fatal(
			"expected execution ID metadata to be excluded",
		)
	}

	source["x-test"][0] = "changed"

	if got["x-test"][0] != "first" {
		t.Fatal("expected metadata values to be cloned")
	}
}

func TestMergeMetadata(t *testing.T) {
	header := metadata.Pairs(
		"x-shared", "header",
		"x-header", "value",
	)

	trailer := metadata.Pairs(
		"x-shared", "trailer",
		"x-trailer", "value",
	)

	got := mergeMetadata(header, trailer)

	shared := got["x-shared"]
	if len(shared) != 2 ||
		shared[0] != "header" ||
		shared[1] != "trailer" {
		t.Errorf(
			"unexpected shared metadata: %#v",
			shared,
		)
	}

	if got["x-header"][0] != "value" {
		t.Errorf(
			"unexpected header metadata: %#v",
			got,
		)
	}

	if got["x-trailer"][0] != "value" {
		t.Errorf(
			"unexpected trailer metadata: %#v",
			got,
		)
	}
}

func TestCloneBytes(t *testing.T) {
	source := []byte{1, 2, 3}

	got := cloneBytes(source)

	source[0] = 9

	if got[0] != 1 {
		t.Fatalf(
			"expected cloned bytes to be independent, got %v",
			got,
		)
	}
}

func TestRouter(t *testing.T) {
	var upstreamMetadata metadata.MD
	var upstreamRequest rawMessage

	upstream := newTestGRPCServer(
		t,
		func(
			_ any,
			stream grpcpkg.ServerStream,
		) error {
			var request rawMessage

			if err := stream.RecvMsg(&request); err != nil {
				return err
			}

			upstreamRequest = cloneBytes(request)

			md, _ := metadata.FromIncomingContext(
				stream.Context(),
			)
			upstreamMetadata = md.Copy()

			if err := stream.SendHeader(
				metadata.Pairs(
					"x-response-header",
					"header-value",
				),
			); err != nil {
				return err
			}

			stream.SetTrailer(
				metadata.Pairs(
					"x-response-trailer",
					"trailer-value",
				),
			)

			response := rawMessage{4, 5, 6}

			return stream.SendMsg(&response)
		},
	)
	defer upstream.stop()

	store := &call.Store{}
	router := newTestGRPCRouter(t, store)

	router.Register(&runtime.Route{
		ServiceID:      "service-1",
		ListenAddress:  "127.0.0.1:0",
		RuntimeAddress: upstream.address(),
	})

	if err := router.Mount(context.Background()); err != nil {
		t.Fatalf("Mount() error = %v", err)
	}

	defer func() {
		if err := router.Unmount(
			context.Background(),
		); err != nil {
			t.Errorf("Unmount() error = %v", err)
		}
	}()

	if len(router.mounted) != 1 {
		t.Fatalf(
			"expected one mounted route, got %d",
			len(router.mounted),
		)
	}

	connection, err := grpcpkg.NewClient(
		router.mounted[0].listener.Addr().String(),
		grpcpkg.WithTransportCredentials(
			insecure.NewCredentials(),
		),
		grpcpkg.WithDefaultCallOptions(
			grpcpkg.ForceCodec(rawCodec{}),
		),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer connection.Close()

	ctx := metadata.NewOutgoingContext(
		context.Background(),
		metadata.Pairs(
			"x-request", "request-value",
			string(keys.E2EngineTestExecutionID),
			"execution-1",
		),
	)

	request := rawMessage{1, 2, 3}
	var response rawMessage
	var responseHeader metadata.MD
	var responseTrailer metadata.MD

	err = connection.Invoke(
		ctx,
		"/test.Service/Method",
		&request,
		&response,
		grpcpkg.Header(&responseHeader),
		grpcpkg.Trailer(&responseTrailer),
	)
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}

	if string(response) != string([]byte{4, 5, 6}) {
		t.Errorf(
			"unexpected response: %v",
			response,
		)
	}

	if string(upstreamRequest) != string(request) {
		t.Errorf(
			"expected upstream request %v, got %v",
			request,
			upstreamRequest,
		)
	}

	if upstreamMetadata.Get("x-request")[0] != "request-value" {
		t.Errorf(
			"expected request metadata upstream, got %#v",
			upstreamMetadata,
		)
	}

	if len(
		upstreamMetadata.Get(
			string(keys.E2EngineTestExecutionID),
		),
	) != 0 {
		t.Fatal(
			"expected execution ID not to be forwarded upstream",
		)
	}

	if responseHeader.Get(
		"x-response-header",
	)[0] != "header-value" {
		t.Errorf(
			"unexpected response header: %#v",
			responseHeader,
		)
	}

	if responseTrailer.Get(
		"x-response-trailer",
	)[0] != "trailer-value" {
		t.Errorf(
			"unexpected response trailer: %#v",
			responseTrailer,
		)
	}

	calls := store.Get(call.Filter{
		TestExecutionID: "execution-1",
		ServiceID:       "service-1",
	})

	if len(calls) != 1 {
		t.Fatalf(
			"expected one recorded call, got %d",
			len(calls),
		)
	}

	got := calls[0]

	if got.GRPC == nil {
		t.Fatal("expected gRPC call")
	}

	if got.GRPC.Request.RPC != "/test.Service/Method" {
		t.Errorf(
			"unexpected RPC %s",
			got.GRPC.Request.RPC,
		)
	}

	if got.GRPC.Request.Metadata["x-request"][0] !=
		"request-value" {
		t.Errorf(
			"unexpected recorded request metadata: %#v",
			got.GRPC.Request.Metadata,
		)
	}

	if _, ok := got.GRPC.Request.Metadata[string(keys.E2EngineTestExecutionID)]; ok {
		t.Fatal(
			"expected execution ID to be excluded from recorded metadata",
		)
	}

	if string(got.GRPC.Request.Message) != string(request) {
		t.Errorf(
			"unexpected recorded request: %v",
			got.GRPC.Request.Message,
		)
	}

	if got.GRPC.Response.Status != codes.OK.String() {
		t.Errorf(
			"expected status OK, got %s",
			got.GRPC.Response.Status,
		)
	}

	if got.GRPC.Response.Metadata["x-response-header"][0] != "header-value" {
		t.Errorf(
			"unexpected recorded response metadata: %#v",
			got.GRPC.Response.Metadata,
		)
	}

	if got.GRPC.Response.Metadata["x-response-trailer"][0] != "trailer-value" {
		t.Errorf(
			"unexpected recorded response metadata: %#v",
			got.GRPC.Response.Metadata,
		)
	}

	if string(got.GRPC.Response.Message) !=
		string(response) {
		t.Errorf(
			"unexpected recorded response: %v",
			got.GRPC.Response.Message,
		)
	}
}

func TestRouterUpstreamError(t *testing.T) {
	upstream := newTestGRPCServer(
		t,
		func(
			_ any,
			stream grpcpkg.ServerStream,
		) error {
			var request rawMessage

			if err := stream.RecvMsg(&request); err != nil {
				return err
			}

			return status.Error(
				codes.InvalidArgument,
				"invalid request",
			)
		},
	)
	defer upstream.stop()

	store := &call.Store{}
	router := newTestGRPCRouter(t, store)

	router.Register(&runtime.Route{
		ServiceID:      "service-1",
		ListenAddress:  "127.0.0.1:0",
		RuntimeAddress: upstream.address(),
	})

	if err := router.Mount(context.Background()); err != nil {
		t.Fatalf("Mount() error = %v", err)
	}

	defer func() {
		if err := router.Unmount(
			context.Background(),
		); err != nil {
			t.Errorf("Unmount() error = %v", err)
		}
	}()

	connection, err := grpcpkg.NewClient(
		router.mounted[0].listener.Addr().String(),
		grpcpkg.WithTransportCredentials(
			insecure.NewCredentials(),
		),
		grpcpkg.WithDefaultCallOptions(
			grpcpkg.ForceCodec(rawCodec{}),
		),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer connection.Close()

	ctx := metadata.NewOutgoingContext(
		context.Background(),
		metadata.Pairs(
			string(keys.E2EngineTestExecutionID),
			"execution-1",
		),
	)

	request := rawMessage{1, 2, 3}
	var response rawMessage

	err = connection.Invoke(
		ctx,
		"/test.Service/Method",
		&request,
		&response,
	)

	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf(
			"expected InvalidArgument, got %v",
			err,
		)
	}

	calls := store.Get(call.Filter{
		TestExecutionID: "execution-1",
		ServiceID:       "service-1",
	})

	if len(calls) != 1 {
		t.Fatalf(
			"expected one recorded call, got %d",
			len(calls),
		)
	}

	if calls[0].GRPC == nil {
		t.Fatal("expected gRPC call")
	}

	if calls[0].GRPC.Response.Status !=
		codes.InvalidArgument.String() {
		t.Errorf(
			"expected recorded status %s, got %s",
			codes.InvalidArgument.String(),
			calls[0].GRPC.Response.Status,
		)
	}
}

func TestRouterMountCancelledContext(t *testing.T) {
	router := newTestGRPCRouter(
		t,
		&call.Store{},
	)

	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	err := router.Mount(ctx)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"expected context.Canceled, got %v",
			err,
		)
	}
}

func TestRouterUnmountCancelledContext(t *testing.T) {
	router := newTestGRPCRouter(
		t,
		&call.Store{},
	)

	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	err := router.Unmount(ctx)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"expected context.Canceled, got %v",
			err,
		)
	}
}

func TestRouterMultipleRoutes(t *testing.T) {
	first := newTestGRPCServer(
		t,
		func(
			_ any,
			stream grpcpkg.ServerStream,
		) error {
			return nil
		},
	)
	defer first.stop()

	second := newTestGRPCServer(
		t,
		func(
			_ any,
			stream grpcpkg.ServerStream,
		) error {
			return nil
		},
	)
	defer second.stop()

	router := newTestGRPCRouter(
		t,
		&call.Store{},
	)

	router.Register(&runtime.Route{
		ServiceID:      "service-1",
		ListenAddress:  "127.0.0.1:0",
		RuntimeAddress: first.address(),
	})

	router.Register(&runtime.Route{
		ServiceID:      "service-2",
		ListenAddress:  "127.0.0.1:0",
		RuntimeAddress: second.address(),
	})

	if err := router.Mount(context.Background()); err != nil {
		t.Fatalf("Mount() error = %v", err)
	}

	if len(router.mounted) != 2 {
		t.Fatalf(
			"expected two mounted routes, got %d",
			len(router.mounted),
		)
	}

	if err := router.Unmount(context.Background()); err != nil {
		t.Fatalf("Unmount() error = %v", err)
	}

	if router.mounted != nil {
		t.Fatal("expected mounted routes to be cleared")
	}
}

type testGRPCServer struct {
	listener net.Listener
	server   *grpcpkg.Server
}

func newTestGRPCServer(
	t *testing.T,
	handler grpcpkg.StreamHandler,
) *testGRPCServer {
	t.Helper()

	listenerConfig := net.ListenConfig{}
	listener, err := listenerConfig.Listen(
		t.Context(),
		"tcp",
		"127.0.0.1:0",
	)
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}

	server := grpcpkg.NewServer(
		grpcpkg.ForceServerCodec(rawCodec{}),
		grpcpkg.UnknownServiceHandler(handler),
	)

	go func() {
		if err := server.Serve(listener); err != nil {
			// Serve returns an error when the listener itself fails.
			// Normal Stop/GracefulStop does not need reporting here.
		}
	}()

	return &testGRPCServer{
		listener: listener,
		server:   server,
	}
}

func (s *testGRPCServer) address() string {
	return s.listener.Addr().String()
}

func (s *testGRPCServer) stop() {
	s.server.Stop()
}

func newTestGRPCRouter(
	t *testing.T,
	store *call.Store,
) *Router {
	t.Helper()

	logger, err := log.NewSilentLogger()
	if err != nil {
		t.Fatalf(
			"cannot create silent logger: %v",
			err,
		)
	}

	return NewRouter(
		logger,
		store,
	)
}
