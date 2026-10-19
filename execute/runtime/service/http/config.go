package http

type MockHTTPServiceConfig struct {
	MaxRequestBodySize int64 `json:"max_request_body_size" yaml:"max_request_body_size" env:"MAX_REQUEST_BODY_SIZE" default:"1048576" validate:"min(1),max(67108864)"`
}
