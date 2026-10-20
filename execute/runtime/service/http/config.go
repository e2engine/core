package http

import "time"

type MockHTTPServiceConfig struct {
	MaxRequestBodySize int64 `json:"max_request_body_size" yaml:"max_request_body_size" env:"MAX_REQUEST_BODY_SIZE" default:"1048576" validate:"min(1),max(67108864)"`

	ReadHeaderTimeout time.Duration `json:"read_header_timeout" yaml:"read_header_timeout" env:"READ_HEADER_TIMEOUT" default:"5s" validate:"min(1s)"`
	IdleTimeout       time.Duration `json:"idle_timeout" yaml:"idle_timeout" env:"IDLE_TIMEOUT" default:"60s" validate:"min(1s)"`

	MaxRequestHeaderBytes int `json:"max_request_header_bytes" yaml:"max_request_header_bytes" env:"MAX_REQUEST_HEADER_BYTES" default:"1048576" validate:"min(1024),max(16777216)"`
}

type RealHTTPServiceConfig struct {
	MaxRequestBodySize int64 `json:"max_request_body_size" yaml:"max_request_body_size" env:"MAX_REQUEST_BODY_SIZE" default:"1048576" validate:"min(1),max(67108864)"`

	ReadHeaderTimeout time.Duration `json:"read_header_timeout" yaml:"read_header_timeout" env:"READ_HEADER_TIMEOUT" default:"5s" validate:"min(1s)"`
	IdleTimeout       time.Duration `json:"idle_timeout" yaml:"idle_timeout" env:"IDLE_TIMEOUT" default:"60s" validate:"min(1s)"`

	MaxRequestHeaderBytes int `json:"max_request_header_bytes" yaml:"max_request_header_bytes" env:"MAX_REQUEST_HEADER_BYTES" default:"1048576" validate:"min(1024),max(16777216)"`

	ResponseHeaderTimeout time.Duration `json:"response_header_timeout" yaml:"response_header_timeout" env:"RESPONSE_HEADER_TIMEOUT" default:"30s" validate:"min(1s)"`

	MaxResponseHeaderBytes int64 `json:"max_response_header_bytes" yaml:"max_response_header_bytes" env:"MAX_RESPONSE_HEADER_BYTES" default:"1048576" validate:"min(1024),max(16777216)"`
}
