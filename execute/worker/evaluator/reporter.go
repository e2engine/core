package evaluator

import (
	"github.com/e2engine/core/model"
)

type deviationReporter interface {
	AddDeviation(deviation *model.TestExecutionDeviation)
}

//go:generate go run -mod=mod go.uber.org/mock/mockgen -source=reporter.go -destination=mock_reporter_test.go -package=evaluator
