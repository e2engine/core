package scheduler

type Config struct {
	QueueSize int `json:"queue_size" yaml:"queue_size" env:"QUEUE_SIZE" default:"1024" validate:"min(1)"`
}
