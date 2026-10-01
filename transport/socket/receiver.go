package socket

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"

	"github.com/ygrebnov/errorc"

	"github.com/e2engine/core/pkg/keys"
	"github.com/e2engine/core/transport"
)

type Receiver struct {
	address  string
	listener net.Listener
}

func NewReceiver(address string) *Receiver {
	return &Receiver{
		address: address,
	}
}

func (t *Receiver) Listen() error {
	// Listen only initializes the receiver; its lifecycle is controlled by
	// Receive context and Close.
	listener, err := net.Listen( //nolint:noctx // per comment above.
		"tcp",
		t.address,
	)
	if err != nil {
		return errorc.With(
			ErrCannotListen,
			errorc.String(keys.SocketAddress, t.address),
			errorc.Error(keys.Cause, err),
		)
	}

	t.listener = listener

	return nil
}

func (t *Receiver) Receive(
	ctx context.Context,
) (transport.Message, error) {
	for {
		var message transport.Message

		connection, err := t.listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return message, ctx.Err()
			}

			return message, errorc.With(
				ErrCannotAccept,
				errorc.String(keys.SocketAddress, t.address),
				errorc.Error(keys.Cause, err),
			)
		}

		if deadline, ok := ctx.Deadline(); ok {
			if errSetDeadline := connection.SetReadDeadline(deadline); errSetDeadline != nil {
				_ = connection.Close()

				return message, errorc.With(
					ErrCannotSetReadDeadline,
					errorc.String(keys.SocketAddress, t.address),
					errorc.String(keys.SocketReadDeadline, deadline.String()),
					errorc.Error(keys.Cause, errSetDeadline),
				)
			}
		}

		err = json.NewDecoder(connection).Decode(&message)
		_ = connection.Close()

		if errors.Is(err, io.EOF) {
			// Empty connection, e.g. readiness probe.
			continue
		}

		if err != nil {
			return message, errorc.With(
				ErrCannotReceiveMessage,
				errorc.String(keys.SocketAddress, t.address),
				errorc.Error(keys.Cause, err),
			)
		}

		return message, nil
	}
}

func (t *Receiver) Close() error {
	if t.listener == nil {
		return nil
	}

	return t.listener.Close()
}
