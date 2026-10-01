package config

import (
	"time"
)

type Config struct {
	ServiceProvider           *ProviderConfig `json:"service_provider" yaml:"service_provider" env:"SERVICE_PROVIDER" default:"dive" validateElem:"omitempty"`
	EnvironmentCleanupTimeout time.Duration   `json:"environment_cleanup_timeout" yaml:"environment_cleanup_timeout" env:"ENVIRONMENT_CLEANUP_TIMEOUT" default:"30s" validate:"min(1s)"`
	JobPublishTimeout         time.Duration   `json:"job_publish_timeout" yaml:"job_publish_timeout" env:"JOB_PUBLISH_TIMEOUT" default:"30s" validate:"min(1s)"`
	ExecutionCheckTimeout     time.Duration   `json:"execution_check_timeout" yaml:"execution_check_timeout" env:"EXECUTION_CHECK_TIMEOUT" default:"30s" validate:"min(1s)"`
	MaxResolvedTests          int             `json:"max_resolved_tests" yaml:"max_resolved_tests" env:"MAX_RESOLVED_TESTS" default:"1000" validate:"min(1)"`
}

type ProviderConfig struct {
	BufDescriptorLoaderToken string `json:"buf_descriptor_loader_token,omitempty" yaml:"buf_descriptor_loader_token,omitempty" env:"BUF_DESCRIPTOR_LOADER_TOKEN"` // TODO: add validation.
}
