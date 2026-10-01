package socket

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/e2engine/core/transport"
)

func TestReceiverReceive(t *testing.T) {
	ctx := t.Context()
	receiver := NewReceiver("127.0.0.1:0")

	if err := receiver.Listen(); err != nil {
		t.Fatal(err)
	}
	defer receiver.Close()

	address := receiver.listener.Addr().String()

	message := transport.NewEndMessage()

	go func() {
		dialer := net.Dialer{}

		connection, err := dialer.DialContext(ctx, "tcp", address)
		if err != nil {
			return
		}
		defer connection.Close()

		_ = json.NewEncoder(connection).Encode(message)
	}()

	actual, err := receiver.Receive(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if actual.Type != message.Type {
		t.Fatalf(
			"expected message type %q, got %q",
			message.Type,
			actual.Type,
		)
	}
}

func TestReceiverReceiveIgnoresEmptyConnection(t *testing.T) {
	ctx := t.Context()

	receiver := NewReceiver("127.0.0.1:0")

	if err := receiver.Listen(); err != nil {
		t.Fatal(err)
	}
	defer receiver.Close()

	address := receiver.listener.Addr().String()

	go func() {
		dialer := net.Dialer{}

		// Simulate readiness probe.
		connection, err := dialer.DialContext(ctx, "tcp", address)
		if err != nil {
			return
		}
		_ = connection.Close()

		connection, err = dialer.DialContext(ctx, "tcp", address)
		if err != nil {
			return
		}
		defer connection.Close()

		_ = json.NewEncoder(connection).Encode(
			transport.NewEndMessage(),
		)
	}()

	message, err := receiver.Receive(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if message.Type != transport.MessageTypeEnd {
		t.Fatalf(
			"expected message type %q, got %q",
			transport.MessageTypeEnd,
			message.Type,
		)
	}
}

func TestReceiverReceiveInvalidJSON(t *testing.T) {
	ctx := t.Context()

	receiver := NewReceiver("127.0.0.1:0")

	if err := receiver.Listen(); err != nil {
		t.Fatal(err)
	}
	defer receiver.Close()

	address := receiver.listener.Addr().String()

	go func() {
		dialer := net.Dialer{}

		connection, err := dialer.DialContext(ctx, "tcp", address)
		if err != nil {
			return
		}
		defer connection.Close()

		_, _ = connection.Write([]byte("{invalid"))
	}()

	_, err := receiver.Receive(ctx)
	if !errors.Is(err, ErrCannotReceiveMessage) {
		t.Fatalf(
			"expected %v, got %v",
			ErrCannotReceiveMessage,
			err,
		)
	}
}

func TestReceiverReceiveCannotAccept(t *testing.T) {
	receiver := NewReceiver("127.0.0.1:0")

	if err := receiver.Listen(); err != nil {
		t.Fatal(err)
	}

	if err := receiver.Close(); err != nil {
		t.Fatal(err)
	}

	_, err := receiver.Receive(context.Background())
	if !errors.Is(err, ErrCannotAccept) {
		t.Fatalf(
			"expected %v, got %v",
			ErrCannotAccept,
			err,
		)
	}
}

func TestReceiverReceiveCancelledContext(t *testing.T) {
	receiver := NewReceiver("127.0.0.1:0")

	if err := receiver.Listen(); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	errorsCh := make(chan error, 1)

	go func() {
		_, err := receiver.Receive(ctx)
		errorsCh <- err
	}()

	cancel()

	if err := receiver.Close(); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-errorsCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf(
				"expected %v, got %v",
				context.Canceled,
				err,
			)
		}

	case <-time.After(time.Second):
		t.Fatal("receiver did not stop after context cancellation")
	}
}
