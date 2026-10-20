package grpc

type MockGRPCServiceConfig struct {
	MaxMessageSize int `json:"max_message_size" yaml:"max_message_size" env:"MAX_MESSAGE_SIZE" default:"4194304" validate:"min(1),max(67108864)"`

	MaxMetadataSize uint32 `json:"max_metadata_size" yaml:"max_metadata_size" env:"MAX_METADATA_SIZE" default:"65536" validate:"min(1024),max(1048576)"`
}

type RealGRPCServiceConfig struct {
	MaxMessageSize int `json:"max_message_size" yaml:"max_message_size" env:"MAX_MESSAGE_SIZE" default:"4194304" validate:"min(1),max(67108864)"`

	MaxMetadataSize uint32 `json:"max_metadata_size" yaml:"max_metadata_size" env:"MAX_METADATA_SIZE" default:"65536" validate:"min(1024),max(1048576)"`
}
