package transport

import (
	"github.com/ygrebnov/errorc"

	"github.com/e2engine/core/execute"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/keys"
)

// TestJob represents a test job that can be sent between the producer and consumer.
type TestJob struct {
	ExecutionID          string            `json:"execution_id"`
	TestSuiteExecutionID string            `json:"test_suite_execution_id,omitempty"`
	Environment          model.Environment `json:"environment"`
	Test                 model.Test        `json:"test"`
}

// NewTestJob creates a new TestJob from an execute.TestJob.
func NewTestJob(job *execute.TestJob) TestJob {
	return TestJob{
		ExecutionID:          job.ExecutionID,
		TestSuiteExecutionID: job.TestSuiteExecutionID,
		Environment:          job.Environment,
		Test:                 job.Test,
	}
}

// TestJob converts transport.TestJob to execute.TestJob.
func (j *TestJob) TestJob() execute.TestJob {
	return execute.TestJob{
		ExecutionID:          j.ExecutionID,
		TestSuiteExecutionID: j.TestSuiteExecutionID,
		Environment:          j.Environment,
		Test:                 j.Test,
	}
}

// MessageType represents the type of message that can be sent between the producer and consumer.
// Available message types are MessageTypeTestJob and MessageTypeEnd.
type MessageType string

const (
	MessageTypeTestJob MessageType = "test_job"
	MessageTypeEnd     MessageType = "end"
)

// Message represents a message that can be sent between the producer and consumer.
// It can either be a TestJob message or an End message.
type Message struct {
	Type MessageType `json:"type" default:"test_job" validate:"oneof(test_job,end)"`

	TestJob *TestJob `json:"test_job,omitempty" validateElem:"dive"`
}

// Validate performs Message validation checks and returns an error if any such check fails.
func (m *Message) Validate() error {
	switch m.Type {
	case MessageTypeTestJob:
		if m.TestJob == nil {
			return errorc.With(
				ErrInvalidMessage,
				errorc.String(keys.MessageType, string(m.Type)),
			)
		}

	case MessageTypeEnd:
		if m.TestJob != nil {
			return errorc.With(
				ErrInvalidMessage,
				errorc.String(keys.MessageType, string(m.Type)),
			)
		}

	default:
		return errorc.With(
			ErrInvalidMessage,
			errorc.String(keys.MessageType, string(m.Type)),
		)
	}

	return nil
}

// NewTestJobMessage creates a new Message of type MessageTypeTestJob.
func NewTestJobMessage(j *TestJob) Message {
	return Message{
		Type:    MessageTypeTestJob,
		TestJob: j,
	}
}

// GetTestJob returns the TestJob contained in the Message.
func (m Message) GetTestJob() TestJob {
	return *m.TestJob
}

// NewEndMessage creates a new Message of type MessageTypeEnd.
func NewEndMessage() Message {
	return Message{Type: MessageTypeEnd}
}
