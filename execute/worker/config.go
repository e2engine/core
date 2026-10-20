package worker

import (
	"time"

	grpcworker "github.com/e2engine/core/execute/worker/grpc"
	httpworker "github.com/e2engine/core/execute/worker/http"
	reporterPkg "github.com/e2engine/core/execute/worker/reporter"
)

type Config struct {
	TestTimeout time.Duration `json:"test_timeout" yaml:"test_timeout" env:"TEST_TIMEOUT" default:"30s" validate:"min(1s)"`

	HTTPMaxRedirects int `json:"http_max_redirects" yaml:"http_max_redirects" env:"HTTP_MAX_REDIRECTS" default:"0" validate:"min(0),max(10)"`

	HTTPMaxResponseHeaderBytes int64 `json:"http_max_response_header_bytes" yaml:"http_max_response_header_bytes" env:"HTTP_MAX_RESPONSE_HEADER_BYTES" default:"1048576" validate:"min(1024),max(16777216)"`

	HTTPEvaluator *httpworker.EvaluatorConfig `json:"http_evaluator" yaml:"http_evaluator" env:"HTTP_EVALUATOR" default:"dive"`
	GRPCExecutor  *grpcworker.ExecutorConfig  `json:"grpc_executor" yaml:"grpc_executor" env:"GRPC_EXECUTOR" default:"dive"`
	Reporter      *reporterPkg.Config         `json:"reporter" yaml:"reporter" env:"REPORTER"`
}
