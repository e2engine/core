package scheduler

import (
	"context"

	"github.com/ygrebnov/errorc"

	"github.com/e2engine/core/execute"
)

var ErrNilTestJobsChannel = errorc.New("test jobs channel is nil")

type Scheduler struct {
	tests chan<- execute.TestJob
}

func NewScheduler(
	tests chan<- execute.TestJob,
) (*Scheduler, error) {
	if tests == nil {
		return nil, ErrNilTestJobsChannel
	}

	return &Scheduler{
		tests: tests,
	}, nil
}

func (s *Scheduler) PublishTestJob(
	ctx context.Context,
	job execute.TestJob,
) error {
	// TODO: Investigate passing TestJob by pointer through the execution pipeline
	// to avoid copying this relatively large value without complicating ownership.
	select {
	case <-ctx.Done():
		return ctx.Err()

	case s.tests <- job:
		return nil
	}
}
