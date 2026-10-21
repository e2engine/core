package socket

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"time"

	"github.com/ygrebnov/errorc"

	"github.com/e2engine/core/pkg/keys"
	"github.com/e2engine/core/transport"
)

type Receiver struct {
	cfg      *transport.SocketConfig
	address  string
	listener net.Listener
}

func NewReceiver(
	cfg *transport.SocketConfig,
	address string,
) *Receiver {
	// TODO: address local peer authentication
	return &Receiver{
		cfg:     cfg,
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
	var zero transport.Message

	if err := ctx.Err(); err != nil {
		return zero, err
	}

	// Cancellation terminates the receiver by closing
	// its listener and interrupting any blocked Accept().
	stopAccept := context.AfterFunc(ctx, func() {
		_ = t.listener.Close()
	})
	defer stopAccept()

	for {
		if err := ctx.Err(); err != nil {
			return zero, err
		}

		connection, err := t.listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return zero, ctx.Err()
			}

			return zero, errorc.With(
				ErrCannotAccept,
				errorc.String(keys.SocketAddress, t.address),
				errorc.Error(keys.Cause, err),
			)
		}

		message, empty, err := t.readMessage(
			ctx,
			connection,
		)
		if err != nil {
			return zero, err
		}

		if empty {
			// Empty connection, e.g. readiness probe.
			continue
		}

		return message, nil
	}
}

func (t *Receiver) readMessage(
	ctx context.Context,
	connection net.Conn,
) (transport.Message, bool, error) {
	var message transport.Message

	defer connection.Close()

	deadline := time.Now().Add(
		t.cfg.ReadTimeout,
	)

	if ctxDeadline, ok := ctx.Deadline(); ok &&
		ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}

	if err := connection.SetReadDeadline(deadline); err != nil {
		return message, false, errorc.With(
			ErrCannotSetReadDeadline,
			errorc.String(keys.SocketAddress, t.address),
			errorc.String(
				keys.SocketReadDeadline,
				deadline.String(),
			),
			errorc.Error(keys.Cause, err),
		)
	}

	// Interrupt an active read on context cancellation.
	stopRead := context.AfterFunc(ctx, func() {
		_ = connection.Close()
	})
	defer stopRead()

	// Read at most MaxMessageSize+1 bytes.
	// The extra byte allows us to distinguish a message
	// exactly at the limit from an oversized message.
	data, err := io.ReadAll(
		io.LimitReader(
			connection,
			t.cfg.MaxMessageSize+1,
		),
	)

	if ctx.Err() != nil {
		return message, false, ctx.Err()
	}

	if err != nil {
		return message, false, errorc.With(
			ErrCannotReceiveMessage,
			errorc.String(keys.SocketAddress, t.address),
			errorc.Error(keys.Cause, err),
		)
	}

	if int64(len(data)) > t.cfg.MaxMessageSize {
		return message, false, ErrMessageSizeLimitExceeded
	}

	if len(data) == 0 {
		return message, true, nil
	}

	if errUnmarshal := json.Unmarshal(data, &message); errUnmarshal != nil {
		return message, false, errorc.With(
			ErrCannotReceiveMessage,
			errorc.String(keys.SocketAddress, t.address),
			errorc.Error(keys.Cause, errUnmarshal),
		)
	}

	return message, false, nil
}

func (t *Receiver) Close() error {
	if t.listener == nil {
		return nil
	}

	return t.listener.Close()
}
