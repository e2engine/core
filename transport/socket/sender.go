package socket

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"time"

	"github.com/ygrebnov/errorc"

	"github.com/e2engine/core/pkg/keys"
	"github.com/e2engine/core/transport"
)

type Sender struct {
	cfg     *transport.SocketConfig
	address string
}

func NewSender(
	cfg *transport.SocketConfig,
	address string,
) *Sender {
	return &Sender{
		cfg:     cfg,
		address: address,
	}
}

func (s *Sender) Send(
	ctx context.Context,
	message transport.Message,
) error {
	// Serialize before establishing the connection.
	data, err := json.Marshal(message)
	if err != nil {
		return errorc.With(
			ErrCannotSendMessage,
			errorc.Error(keys.Cause, err),
		)
	}

	// Account for the newline appended by json.Encoder.Encode.
	if int64(len(data)) >= s.cfg.MaxMessageSize {
		return ErrMessageSizeLimitExceeded
	}

	data = append(data, '\n')

	connection, err := (&net.Dialer{}).DialContext(
		ctx,
		"tcp",
		s.address,
	)
	if err != nil {
		return errorc.With(
			ErrCannotConnect,
			errorc.String(keys.SocketAddress, s.address),
			errorc.Error(keys.Cause, err),
		)
	}
	defer connection.Close()

	// Apply an independent write timeout, respecting an
	// earlier deadline supplied by the caller.
	deadline := time.Now().Add(s.cfg.WriteTimeout)

	if ctxDeadline, ok := ctx.Deadline(); ok &&
		ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}

	if err := connection.SetWriteDeadline(deadline); err != nil {
		return errorc.With(
			ErrCannotSetWriteDeadline,
			errorc.String(keys.SocketAddress, s.address),
			errorc.String(
				keys.SocketWriteDeadline,
				deadline.String(),
			),
			errorc.Error(keys.Cause, err),
		)
	}

	// Context cancellation must interrupt an active write,
	// even when the caller has no deadline.
	stop := context.AfterFunc(ctx, func() {
		_ = connection.Close()
	})
	defer stop()

	if _, err := io.Copy(
		connection,
		bytes.NewReader(data),
	); err != nil {
		return errorc.With(
			ErrCannotSendMessage,
			errorc.String(keys.SocketAddress, s.address),
			errorc.Error(keys.Cause, err),
		)
	}

	return nil
}
