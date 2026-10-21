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
	receiver := newTestReceiver(t, "127.0.0.1:0")

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

	receiver := newTestReceiver(t, "127.0.0.1:0")

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

	receiver := newTestReceiver(t, "127.0.0.1:0")

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
	receiver := newTestReceiver(t, "127.0.0.1:0")

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
	receiver := newTestReceiver(t, "127.0.0.1:0")

	if err := receiver.Listen(); err != nil {
		t.Fatal(err)
	}
	defer receiver.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errorsCh := make(chan error, 1)

	go func() {
		_, err := receiver.Receive(ctx)
		errorsCh <- err
	}()

	cancel()

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

func TestReceiverReceiveMessageSizeLimit(t *testing.T) {
	message := transport.NewEndMessage()

	data, err := json.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}

	// Match the sender's newline-terminated JSON format.
	data = append(data, '\n')

	tests := []struct {
		name      string
		limit     int64
		wantError error
	}{
		{
			name:  "exactly at limit",
			limit: int64(len(data)),
		},
		{
			name:      "exceeds limit",
			limit:     int64(len(data)) - 1,
			wantError: ErrMessageSizeLimitExceeded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			receiver := newTestReceiver(t, "127.0.0.1:0")
			receiver.cfg.MaxMessageSize = tt.limit

			if err := receiver.Listen(); err != nil {
				t.Fatal(err)
			}
			defer receiver.Close()

			conn, err := net.Dial(
				"tcp",
				receiver.listener.Addr().String(),
			)
			if err != nil {
				t.Fatal(err)
			}

			if _, err := conn.Write(data); err != nil {
				_ = conn.Close()
				t.Fatal(err)
			}

			if err := conn.Close(); err != nil {
				t.Fatal(err)
			}

			actual, err := receiver.Receive(t.Context())

			if tt.wantError != nil {
				if !errors.Is(err, tt.wantError) {
					t.Fatalf(
						"Receive() error = %v, want %v",
						err,
						tt.wantError,
					)
				}
				return
			}

			if err != nil {
				t.Fatalf("Receive() error = %v", err)
			}

			if actual.Type != message.Type {
				t.Errorf(
					"message type = %q, want %q",
					actual.Type,
					message.Type,
				)
			}
		})
	}
}

func TestReceiverReceiveCancelledDuringRead(t *testing.T) {
	receiver := newTestReceiver(t, "127.0.0.1:0")

	if err := receiver.Listen(); err != nil {
		t.Fatal(err)
	}
	defer receiver.Close()

	conn, err := net.Dial(
		"tcp",
		receiver.listener.Addr().String(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// Send an incomplete JSON message.
	if _, err := conn.Write([]byte(`{"type":`)); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errorsCh := make(chan error, 1)

	go func() {
		_, err := receiver.Receive(ctx)
		errorsCh <- err
	}()

	cancel()

	select {
	case err := <-errorsCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf(
				"Receive() error = %v, want context.Canceled",
				err,
			)
		}

	case <-time.After(2 * time.Second):
		t.Fatal("Receive() did not stop after cancellation")
	}
}

func TestReceiverReceiveReadTimeout(t *testing.T) {
	receiver := newTestReceiver(t, "127.0.0.1:0")
	receiver.cfg.ReadTimeout = 50 * time.Millisecond

	if err := receiver.Listen(); err != nil {
		t.Fatal(err)
	}
	defer receiver.Close()

	conn, err := net.Dial(
		"tcp",
		receiver.listener.Addr().String(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// The client sends incomplete JSON and keeps
	// the connection open.
	if _, err := conn.Write([]byte(`{"type":`)); err != nil {
		t.Fatal(err)
	}

	_, err = receiver.Receive(t.Context())

	if !errors.Is(err, ErrCannotReceiveMessage) {
		t.Fatalf(
			"Receive() error = %v, want %v",
			err,
			ErrCannotReceiveMessage,
		)
	}
}

func TestReceiverReceiveTrailingData(t *testing.T) {
	receiver := newTestReceiver(t, "127.0.0.1:0")

	if err := receiver.Listen(); err != nil {
		t.Fatal(err)
	}
	defer receiver.Close()

	conn, err := net.Dial(
		"tcp",
		receiver.listener.Addr().String(),
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = conn.Write(
		[]byte(`{"type":"end"} {"type":"end"}`),
	)
	if err != nil {
		_ = conn.Close()
		t.Fatal(err)
	}

	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}

	_, err = receiver.Receive(t.Context())

	if !errors.Is(err, ErrCannotReceiveMessage) {
		t.Fatalf(
			"Receive() error = %v, want %v",
			err,
			ErrCannotReceiveMessage,
		)
	}
}

func newTestReceiver(t *testing.T, address string) *Receiver {
	t.Helper()
	receiver := NewReceiver(
		&transport.SocketConfig{
			MaxMessageSize: 1 << 20,
			ReadTimeout:    5 * time.Second,
		},
		address,
	)
	return receiver
}
