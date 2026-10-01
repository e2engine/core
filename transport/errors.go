package transport

import "github.com/ygrebnov/errorc"

var (
	ErrInvalidConfig  = errorc.New("invalid transport configuration")
	ErrInvalidMessage = errorc.New("invalid transport message")
)
