package call

type Config struct {
	MaxCalls int `json:"max_calls" yaml:"max_calls" default:"1000" validate:"min(1),max(100000)"`
}
