package transport

import (
	"github.com/ygrebnov/errorc"

	"github.com/e2engine/core/pkg/keys"
)

type Config struct {
	Kind    Kind   `json:"kind" yaml:"kind" default:"direct" validate:"oneof(direct,socket)"`
	Address string `json:"address,omitempty" yaml:"address,omitempty" validate:"omitempty,networkaddress"`
}

func (c *Config) Validate() error {
	if c.Kind == KindSocket && c.Address == "" {
		return errorc.With(
			ErrInvalidConfig,
			errorc.String(keys.Validation, "transport address must be provided for socket transport"),
		)
	}

	if c.Kind == KindDirect && c.Address != "" {
		return errorc.With(
			ErrInvalidConfig,
			errorc.String(keys.Validation, "transport address is not allowed for direct transport"),
		)
	}

	return nil
}

type Kind string

const (
	KindDirect Kind = "direct"
	KindSocket Kind = "socket"
)
