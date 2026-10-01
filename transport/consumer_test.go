package transport

import (
	"context"
	"errors"
	"testing"

	"github.com/e2engine/core/execute"
	"github.com/e2engine/core/model"
)

var errReceive = errors.New("receive error")

type testReceiver struct {
	messages []Message
	err      error
	index    int
}

func (r *testReceiver) Receive(
	_ context.Context,
) (Message, error) {
	if r.err != nil {
		return Message{}, r.err
	}

	message := r.messages[r.index]
	r.index++

	return message, nil
}

func TestConsumerRunReceivesTestJob(t *testing.T) {
	jobs := make(chan execute.TestJob, 1)

	expected := &execute.TestJob{
		ExecutionID:          "execution-1",
		TestSuiteExecutionID: "suite-execution-1",
		Environment: model.Environment{
			ID: "environment-1",
		},
		Test: model.Test{
			ID: "test-1",
		},
	}

	testJob := NewTestJob(expected)

	receiver := &testReceiver{
		messages: []Message{
			NewTestJobMessage(&testJob),
			NewEndMessage(),
		},
	}

	consumer := NewConsumer(jobs, receiver)

	if err := consumer.Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	actual, ok := <-jobs
	if !ok {
		t.Fatal("expected test job")
	}

	if actual.ExecutionID != expected.ExecutionID {
		t.Fatalf(
			"expected execution ID %q, got %q",
			expected.ExecutionID,
			actual.ExecutionID,
		)
	}

	if actual.TestSuiteExecutionID != expected.TestSuiteExecutionID {
		t.Fatalf(
			"expected test suite execution ID %q, got %q",
			expected.TestSuiteExecutionID,
			actual.TestSuiteExecutionID,
		)
	}

	if actual.Environment.ID != expected.Environment.ID {
		t.Fatalf(
			"expected environment ID %q, got %q",
			expected.Environment.ID,
			actual.Environment.ID,
		)
	}

	if actual.Test.ID != expected.Test.ID {
		t.Fatalf(
			"expected test ID %q, got %q",
			expected.Test.ID,
			actual.Test.ID,
		)
	}

	if _, ok := <-jobs; ok {
		t.Fatal("expected jobs channel to be closed")
	}
}

func TestConsumerRunEndClosesJobs(t *testing.T) {
	jobs := make(chan execute.TestJob)

	receiver := &testReceiver{
		messages: []Message{
			NewEndMessage(),
		},
	}

	consumer := NewConsumer(jobs, receiver)

	if err := consumer.Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	if _, ok := <-jobs; ok {
		t.Fatal("expected jobs channel to be closed")
	}
}

func TestConsumerRunReceiveError(t *testing.T) {
	jobs := make(chan execute.TestJob, 1)

	receiver := &testReceiver{
		err: errReceive,
	}

	consumer := NewConsumer(jobs, receiver)

	err := consumer.Run(context.Background())
	if !errors.Is(err, errReceive) {
		t.Fatalf("expected %v, got %v", errReceive, err)
	}
}

func TestConsumerRunInvalidMessage(t *testing.T) {
	jobs := make(chan execute.TestJob, 1)

	receiver := &testReceiver{
		messages: []Message{
			{
				Type: MessageTypeTestJob,
			},
		},
	}

	consumer := NewConsumer(jobs, receiver)

	err := consumer.Run(context.Background())
	if !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("expected %v, got %v", ErrInvalidMessage, err)
	}
}

func TestConsumerRunCancelledContext(t *testing.T) {
	jobs := make(chan execute.TestJob)

	testJob := NewTestJob(&execute.TestJob{})

	receiver := &testReceiver{
		messages: []Message{
			NewTestJobMessage(&testJob),
		},
	}

	consumer := NewConsumer(jobs, receiver)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := consumer.Run(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"expected %v, got %v",
			context.Canceled,
			err,
		)
	}
}
