package socket

import (
	"context"
	"encoding/json"
	"net"

	"github.com/ygrebnov/errorc"

	"github.com/e2engine/core/pkg/keys"
	"github.com/e2engine/core/transport"
)

type Sender struct {
	address string
}

func NewSender(address string) *Sender {
	return &Sender{
		address: address,
	}
}

func (s *Sender) Send(
	ctx context.Context,
	message transport.Message,
) error {
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

	if deadline, ok := ctx.Deadline(); ok {
		if err := connection.SetWriteDeadline(deadline); err != nil {
			return errorc.With(
				ErrCannotSetWriteDeadline,
				errorc.String(keys.SocketAddress, s.address),
				errorc.String(keys.SocketWriteDeadline, deadline.String()),
				errorc.Error(keys.Cause, err),
			)
		}
	}

	if err := json.NewEncoder(connection).Encode(message); err != nil {
		return errorc.With(
			ErrCannotSendMessage,
			errorc.String(keys.SocketAddress, s.address),
			errorc.Error(keys.Cause, err),
		)
	}

	return nil
}
