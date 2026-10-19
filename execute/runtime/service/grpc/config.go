package grpc

type MockGRPCServiceConfig struct {
	MaxMessageSize int `json:"max_message_size" yaml:"max_message_size" default:"4194304" validate:"min(1),max(67108864)"`
}

type RealGRPCServiceConfig struct {
	MaxMessageSize int `json:"max_message_size" yaml:"max_message_size" default:"4194304" validate:"min(1),max(67108864)"`
}
