package grpc

import (
	"github.com/e2engine/core/model"
)

type executionReporter interface {
	AddDeviation(deviation *model.TestExecutionDeviation)
	AddPayloadDeviation(deviation *model.PayloadDeviation)
	AddMetadataDeviation(
		key string,
		deviation *model.TestExecutionDeviation,
	)
	AddGRPCRequest(req *model.GRPCRequest)
	AddGRPCResponseStatus(status string)
	AddGRPCResponseMetadata(metadata map[string][]string)
	AddGRPCResponseMessage(message map[string]any)
}

//go:generate go run -mod=mod go.uber.org/mock/mockgen -source=reporter.go -destination=mock_reporter_test.go -package=grpc
