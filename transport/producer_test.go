package transport

import (
	"context"
	"errors"
	"testing"

	"github.com/e2engine/core/execute"
	"github.com/e2engine/core/model"
)

var errSend = errors.New("send error")

type testSender struct {
	messages []Message
	err      error
}

func (s *testSender) Send(
	_ context.Context,
	message Message,
) error {
	if s.err != nil {
		return s.err
	}

	s.messages = append(s.messages, message)

	return nil
}

func TestProducerRunSendsTestJob(t *testing.T) {
	jobs := make(chan execute.TestJob, 1)
	sender := &testSender{}

	job := execute.TestJob{
		ExecutionID:          "execution-1",
		TestSuiteExecutionID: "suite-execution-1",
		Environment: model.Environment{
			ID: "environment-1",
		},
		Test: model.Test{
			ID: "test-1",
		},
	}
	jobs <- job
	close(jobs)

	producer := NewProducer(jobs, sender)

	if err := producer.Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	if len(sender.messages) != 2 {
		t.Fatalf(
			"expected 2 messages, got %d",
			len(sender.messages),
		)
	}

	message := sender.messages[0]

	if message.Type != MessageTypeTestJob {
		t.Fatalf(
			"expected message type %q, got %q",
			MessageTypeTestJob,
			message.Type,
		)
	}

	if message.TestJob == nil {
		t.Fatal("expected test job")
	}

	if message.TestJob.ExecutionID != job.ExecutionID {
		t.Fatalf(
			"expected execution ID %q, got %q",
			job.ExecutionID,
			message.TestJob.ExecutionID,
		)
	}

	if message.TestJob.TestSuiteExecutionID != job.TestSuiteExecutionID {
		t.Fatalf(
			"expected test suite execution ID %q, got %q",
			job.TestSuiteExecutionID,
			message.TestJob.TestSuiteExecutionID,
		)
	}

	if message.TestJob.Environment.ID != job.Environment.ID {
		t.Fatalf(
			"expected environment ID %q, got %q",
			job.Environment.ID,
			message.TestJob.Environment.ID,
		)
	}

	if message.TestJob.Test.ID != job.Test.ID {
		t.Fatalf(
			"expected test ID %q, got %q",
			job.Test.ID,
			message.TestJob.Test.ID,
		)
	}
}

func TestProducerRunSendsEnd(t *testing.T) {
	jobs := make(chan execute.TestJob)
	close(jobs)

	sender := &testSender{}
	producer := NewProducer(jobs, sender)

	if err := producer.Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	if len(sender.messages) != 1 {
		t.Fatalf(
			"expected 1 message, got %d",
			len(sender.messages),
		)
	}

	if sender.messages[0].Type != MessageTypeEnd {
		t.Fatalf(
			"expected message type %q, got %q",
			MessageTypeEnd,
			sender.messages[0].Type,
		)
	}
}

func TestProducerRunTestJobSendError(t *testing.T) {
	jobs := make(chan execute.TestJob, 1)
	jobs <- execute.TestJob{}

	sender := &testSender{
		err: errSend,
	}

	producer := NewProducer(jobs, sender)

	err := producer.Run(context.Background())
	if !errors.Is(err, errSend) {
		t.Fatalf("expected %v, got %v", errSend, err)
	}
}

func TestProducerRunEndSendError(t *testing.T) {
	jobs := make(chan execute.TestJob)
	close(jobs)

	sender := &testSender{
		err: errSend,
	}

	producer := NewProducer(jobs, sender)

	err := producer.Run(context.Background())
	if !errors.Is(err, errSend) {
		t.Fatalf("expected %v, got %v", errSend, err)
	}
}

func TestProducerRunCancelledContext(t *testing.T) {
	jobs := make(chan execute.TestJob)

	sender := &testSender{}
	producer := NewProducer(jobs, sender)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := producer.Run(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"expected %v, got %v",
			context.Canceled,
			err,
		)
	}

	if len(sender.messages) != 0 {
		t.Fatalf(
			"expected no messages, got %d",
			len(sender.messages),
		)
	}
}
