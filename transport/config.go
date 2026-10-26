package transport

import (
	"net"
	"time"

	"github.com/ygrebnov/errorc"

	"github.com/e2engine/core/pkg/keys"
)

type Config struct {
	Kind    Kind   `json:"kind" yaml:"kind" default:"direct" validate:"oneof(direct,socket)"`
	Address string `json:"address,omitempty" yaml:"address,omitempty" validate:"omitempty,networkaddress"`

	Socket *SocketConfig `json:"socket,omitempty" yaml:"socket,omitempty" default:"dive" validateElem:"omitempty,dive"`
}

func (c *Config) Validate() error {
	switch c.Kind {
	case KindDirect:
		if c.Address != "" {
			return errorc.With(
				ErrInvalidConfig,
				errorc.String(
					keys.Validation,
					"transport address is not allowed for direct transport",
				),
			)
		}

	case KindSocket:
		if c.Address == "" {
			return errorc.With(
				ErrInvalidConfig,
				errorc.String(
					keys.Validation,
					"transport address must be provided for socket transport",
				),
			)
		}

		if c.Socket == nil {
			return errorc.With(
				ErrInvalidConfig,
				errorc.String(
					keys.Validation,
					"socket configuration must be provided for socket transport",
				),
			)
		}

		host, _, err := net.SplitHostPort(c.Address)
		if err != nil {
			return errorc.With(
				ErrInvalidConfig,
				errorc.String(
					keys.Validation,
					"invalid socket transport address",
				),
			)
		}

		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return errorc.With(
				ErrInvalidConfig,
				errorc.String(
					keys.Validation,
					"socket transport address must use a loopback IP",
				),
			)
		}

	default:
		return errorc.With(
			ErrInvalidConfig,
			errorc.String(keys.Validation, "invalid transport kind"),
		)
	}

	return nil
}

type Kind string

const (
	KindDirect Kind = "direct"
	KindSocket Kind = "socket"
)

type SocketConfig struct {
	MaxMessageSize int64 `json:"max_message_size" yaml:"max_message_size" env:"MAX_MESSAGE_SIZE" default:"67108864" validate:"min(1024),max(268435456)"`

	ReadTimeout  time.Duration `json:"read_timeout" yaml:"read_timeout" env:"READ_TIMEOUT" default:"30s" validate:"min(1s)"`
	WriteTimeout time.Duration `json:"write_timeout" yaml:"write_timeout" env:"WRITE_TIMEOUT" default:"30s" validate:"min(1s)"`
}
