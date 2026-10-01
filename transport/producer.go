package transport

import (
	"context"

	"github.com/e2engine/core/execute"
)

type Sender interface {
	Send(
		ctx context.Context,
		message Message,
	) error
}

type Producer struct {
	jobs   <-chan execute.TestJob
	sender Sender
}

func NewProducer(
	jobs <-chan execute.TestJob,
	sender Sender,
) *Producer {
	return &Producer{
		jobs:   jobs,
		sender: sender,
	}
}

func (p *Producer) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case job, ok := <-p.jobs:
			if !ok {
				return p.sender.Send(
					ctx,
					NewEndMessage(),
				)
			}

			tj := NewTestJob(&job)

			if err := p.sender.Send(
				ctx,
				NewTestJobMessage(&tj),
			); err != nil {
				return err
			}
		}
	}
}
