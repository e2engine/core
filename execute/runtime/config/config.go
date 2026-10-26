package config

import (
	"time"

	grpcservice "github.com/e2engine/core/execute/runtime/service/grpc"
	httpservice "github.com/e2engine/core/execute/runtime/service/http"
)

type Config struct {
	Service                   *ServiceConfig `json:"service" yaml:"service" env:"SERVICE" default:"dive" validateElem:"omitempty"`
	Router                    *RouterConfig  `json:"router" yaml:"router" env:"ROUTER" default:"dive"`
	EnvironmentCleanupTimeout time.Duration  `json:"environment_cleanup_timeout" yaml:"environment_cleanup_timeout" env:"ENVIRONMENT_CLEANUP_TIMEOUT" default:"30s" validate:"min(1s),max(10m)"`
	JobPublishTimeout         time.Duration  `json:"job_publish_timeout" yaml:"job_publish_timeout" env:"JOB_PUBLISH_TIMEOUT" default:"30s" validate:"min(1s),max(5m)"`
	ExecutionCheckTimeout     time.Duration  `json:"execution_check_timeout" yaml:"execution_check_timeout" env:"EXECUTION_CHECK_TIMEOUT" default:"30s" validate:"min(1s),max(5m)"`
	MaxResolvedTests          int            `json:"max_resolved_tests" yaml:"max_resolved_tests" env:"MAX_RESOLVED_TESTS" default:"1000" validate:"min(1),max(10000)"`
	E2EngineListenInterface   string         `json:"e2engine_listen_interface" yaml:"e2engine_listen_interface" env:"E2ENGINE_LISTEN_INTERFACE" default:"127.0.0.1" validate:"ip"`
}

type ServiceConfig struct {
	MockHTTP *httpservice.MockHTTPServiceConfig `json:"mock_http" yaml:"mock_http" default:"dive"`
	MockGRPC *grpcservice.MockGRPCServiceConfig `json:"mock_grpc" yaml:"mock_grpc" default:"dive"`
	RealHTTP *httpservice.RealHTTPServiceConfig `json:"real_http" yaml:"real_http" default:"dive"`
	RealGRPC *grpcservice.RealGRPCServiceConfig `json:"real_grpc" yaml:"real_grpc" default:"dive"`
	// BufDescriptorLoaderToken is a token used to authenticate with the Buf Descriptor Loader service.
	// It can be provided via environment variable.
	BufDescriptorLoaderToken string `json:"-" yaml:"-" env:"BUF_DESCRIPTOR_LOADER_TOKEN"`
}

type RouterConfig struct {
	HTTP *HTTPRouterConfig `json:"http" yaml:"http" default:"dive"`
	GRPC *GRPCRouterConfig `json:"grpc" yaml:"grpc" default:"dive"`
}

type HTTPRouterConfig struct {
	MaxRecordedBodySize int64 `json:"max_recorded_body_size" yaml:"max_recorded_body_size" env:"MAX_RECORDED_BODY_SIZE" default:"1048576" validate:"min(1),max(67108864)"`

	ReadHeaderTimeout time.Duration `json:"read_header_timeout" yaml:"read_header_timeout" env:"READ_HEADER_TIMEOUT" default:"5s" validate:"min(1s),max(1m)"`
	IdleTimeout       time.Duration `json:"idle_timeout" yaml:"idle_timeout" env:"IDLE_TIMEOUT" default:"60s" validate:"min(1s),max(10m)"`

	ResponseHeaderTimeout time.Duration `json:"response_header_timeout" yaml:"response_header_timeout" env:"RESPONSE_HEADER_TIMEOUT" default:"30s" validate:"min(1s),max(5m)"`

	MaxResponseHeaderBytes int64 `json:"max_response_header_bytes" yaml:"max_response_header_bytes" env:"MAX_RESPONSE_HEADER_BYTES" default:"1048576" validate:"min(1024),max(16777216)"`
}

type GRPCRouterConfig struct {
	MaxMessageSize int `json:"max_message_size" yaml:"max_message_size" env:"MAX_MESSAGE_SIZE" default:"4194304" validate:"min(1),max(67108864)"`

	MaxMetadataSize uint32 `json:"max_metadata_size" yaml:"max_metadata_size" env:"MAX_METADATA_SIZE" default:"65536" validate:"min(1024),max(1048576)"`
}
