package http

type Config struct {
	MaxRecordedBodySize int64 `json:"max_recorded_body_size" yaml:"max_recorded_body_size" default:"1048576" validate:"min(1),max(67108864)"`
}
