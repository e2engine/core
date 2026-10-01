package transport

import (
	"context"

	"github.com/e2engine/core/execute"
)

type Receiver interface {
	Receive(ctx context.Context) (Message, error)
}

type Consumer struct {
	jobs     chan<- execute.TestJob
	receiver Receiver
}

func NewConsumer(
	jobs chan<- execute.TestJob,
	receiver Receiver,
) *Consumer {
	return &Consumer{
		jobs:     jobs,
		receiver: receiver,
	}
}

func (c *Consumer) Run(ctx context.Context) error {
	for {
		message, err := c.receiver.Receive(ctx)
		if err != nil {
			return err
		}

		if err := message.Validate(); err != nil {
			return err
		}

		if message.Type == MessageTypeEnd {
			close(c.jobs)
			return nil
		}

		tj := message.GetTestJob()

		select {
		case <-ctx.Done():
			return ctx.Err()

		case c.jobs <- tj.TestJob():
		}
	}
}
