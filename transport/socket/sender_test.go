package socket

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"testing"

	"github.com/e2engine/core/transport"
)

func TestSenderSend(t *testing.T) {
	listenerConfig := net.ListenConfig{}
	listener, err := listenerConfig.Listen(
		t.Context(),
		"tcp",
		"127.0.0.1:0",
	)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	message := transport.NewEndMessage()

	received := make(chan transport.Message, 1)
	receiveErrors := make(chan error, 1)

	go func() {
		connection, err := listener.Accept()
		if err != nil {
			receiveErrors <- err
			return
		}
		defer connection.Close()

		var message transport.Message
		if err := json.NewDecoder(connection).Decode(&message); err != nil {
			receiveErrors <- err
			return
		}

		received <- message
	}()

	sender := NewSender(listener.Addr().String())

	if err := sender.Send(context.Background(), message); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-receiveErrors:
		t.Fatal(err)

	case actual := <-received:
		if actual.Type != message.Type {
			t.Fatalf(
				"expected message type %q, got %q",
				message.Type,
				actual.Type,
			)
		}
	}
}

func TestSenderSendCannotConnect(t *testing.T) {
	listenerConfig := net.ListenConfig{}
	listener, err := listenerConfig.Listen(
		t.Context(),
		"tcp",
		"127.0.0.1:0",
	)
	if err != nil {
		t.Fatal(err)
	}

	address := listener.Addr().String()

	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}

	sender := NewSender(address)

	err = sender.Send(
		context.Background(),
		transport.NewEndMessage(),
	)
	if !errors.Is(err, ErrCannotConnect) {
		t.Fatalf("expected %v, got %v", ErrCannotConnect, err)
	}
}

func TestSenderSendCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	sender := NewSender("127.0.0.1:1")

	err := sender.Send(
		ctx,
		transport.NewEndMessage(),
	)
	if !errors.Is(err, ErrCannotConnect) {
		t.Fatalf("expected %v, got %v", ErrCannotConnect, err)
	}

	if ctx.Err() != context.Canceled {
		t.Fatalf("expected context to be canceled, got %v", ctx.Err())
	}
}
