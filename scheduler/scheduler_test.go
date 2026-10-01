package scheduler

import (
	"context"
	"errors"
	"testing"

	"github.com/e2engine/core/execute"
)

func TestNewSchedulerNilTestJobsChannel(t *testing.T) {
	scheduler, err := NewScheduler(nil)

	if !errors.Is(err, ErrNilTestJobsChannel) {
		t.Fatalf(
			"expected %v, got %v",
			ErrNilTestJobsChannel,
			err,
		)
	}

	if scheduler != nil {
		t.Fatal("expected nil scheduler")
	}
}

func TestSchedulerPublishTestJob(t *testing.T) {
	jobs := make(chan execute.TestJob, 1)

	scheduler, err := NewScheduler(jobs)
	if err != nil {
		t.Fatal(err)
	}

	expected := execute.TestJob{
		ExecutionID: "execution-1",
	}

	if err := scheduler.PublishTestJob(
		context.Background(),
		expected,
	); err != nil {
		t.Fatal(err)
	}

	actual := <-jobs

	if actual.ExecutionID != expected.ExecutionID {
		t.Fatalf(
			"expected execution ID %q, got %q",
			expected.ExecutionID,
			actual.ExecutionID,
		)
	}
}

func TestSchedulerPublishTestJobCancelledContext(t *testing.T) {
	jobs := make(chan execute.TestJob)

	scheduler, err := NewScheduler(jobs)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = scheduler.PublishTestJob(
		ctx,
		execute.TestJob{},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"expected %v, got %v",
			context.Canceled,
			err,
		)
	}
}
