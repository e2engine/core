package call

type Config struct {
	MaxCalls         int   `json:"max_calls" yaml:"max_calls" default:"1000" validate:"min(1),max(100000)"`
	MaxRecordedBytes int64 `json:"max_recorded_bytes" yaml:"max_recorded_bytes" default:"67108864" validate:"min(1),max(1073741824)"`
}
