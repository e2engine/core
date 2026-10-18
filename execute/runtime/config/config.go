package config

import (
	"time"

	httpservice "github.com/e2engine/core/execute/runtime/service/http"
)

type Config struct {
	ServiceProvider           *ProviderConfig `json:"service_provider" yaml:"service_provider" env:"SERVICE_PROVIDER" default:"dive" validateElem:"omitempty"`
	EnvironmentCleanupTimeout time.Duration   `json:"environment_cleanup_timeout" yaml:"environment_cleanup_timeout" env:"ENVIRONMENT_CLEANUP_TIMEOUT" default:"30s" validate:"min(1s)"`
	JobPublishTimeout         time.Duration   `json:"job_publish_timeout" yaml:"job_publish_timeout" env:"JOB_PUBLISH_TIMEOUT" default:"30s" validate:"min(1s)"`
	ExecutionCheckTimeout     time.Duration   `json:"execution_check_timeout" yaml:"execution_check_timeout" env:"EXECUTION_CHECK_TIMEOUT" default:"30s" validate:"min(1s)"`
	MaxResolvedTests          int             `json:"max_resolved_tests" yaml:"max_resolved_tests" env:"MAX_RESOLVED_TESTS" default:"1000" validate:"min(1)"`
	E2EngineListenInterface   string          `json:"e2engine_listen_interface" yaml:"e2engine_listen_interface" env:"E2ENGINE_LISTEN_INTERFACE" default:"127.0.0.1" validate:"ip"`
}

type ProviderConfig struct {
	MockHTTP                 *httpservice.MockHTTPServiceConfig `json:"mock_http" yaml:"mock_http" default:"dive"`
	BufDescriptorLoaderToken string                             `json:"buf_descriptor_loader_token,omitempty" yaml:"buf_descriptor_loader_token,omitempty" env:"BUF_DESCRIPTOR_LOADER_TOKEN"` // TODO: add validation.
}
