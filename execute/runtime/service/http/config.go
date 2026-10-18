package http

type MockHTTPServiceConfig struct {
	MaxRequestBodySize int64 `json:"max_request_body_size" yaml:"max_request_body_size" default:"1048576" validate:"min(1),max(67108864)"`
}
