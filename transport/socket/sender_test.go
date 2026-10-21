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

	sender := NewSender(
		newTestSenderConfig(),
		listener.Addr().String(),
	)

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

	sender := NewSender(
		newTestSenderConfig(),
		address,
	)

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

	sender := NewSender(
		newTestSenderConfig(),
		"127.0.0.1:1",
	)

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

func TestSenderSendMessageSizeLimit(t *testing.T) {
	message := transport.NewEndMessage()

	data, err := json.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		limit     int64
		wantError error
	}{
		{
			name:      "message exceeds limit",
			limit:     int64(len(data)),
			wantError: ErrMessageSizeLimitExceeded,
		},
		{
			name:  "message exactly at limit",
			limit: int64(len(data)) + 1,
		},
		{
			name:  "message below limit",
			limit: int64(len(data)) + 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

			sender := NewSender(
				&transport.SocketConfig{
					MaxMessageSize: tt.limit,
					WriteTimeout:   5 * time.Second,
				},
				listener.Addr().String(),
			)

			if tt.wantError != nil {
				err := sender.Send(t.Context(), message)

				if !errors.Is(err, tt.wantError) {
					t.Fatalf(
						"Send() error = %v, want %v",
						err,
						tt.wantError,
					)
				}

				return
			}

			received := make(chan transport.Message, 1)
			receiveErrors := make(chan error, 1)

			go func() {
				connection, err := listener.Accept()
				if err != nil {
					receiveErrors <- err
					return
				}
				defer connection.Close()

				var actual transport.Message

				if err := json.NewDecoder(connection).Decode(&actual); err != nil {
					receiveErrors <- err
					return
				}

				received <- actual
			}()

			if err := sender.Send(t.Context(), message); err != nil {
				t.Fatalf("Send() error = %v", err)
			}

			select {
			case err := <-receiveErrors:
				t.Fatal(err)

			case actual := <-received:
				if actual.Type != message.Type {
					t.Errorf(
						"message type = %q, want %q",
						actual.Type,
						message.Type,
					)
				}

			case <-time.After(5 * time.Second):
				t.Fatal("timed out waiting for message")
			}
		})
	}
}

func newTestSenderConfig() *transport.SocketConfig {
	return &transport.SocketConfig{
		MaxMessageSize: 64 << 20,
		WriteTimeout:   5 * time.Second,
	}
}
