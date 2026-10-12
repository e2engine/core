package model

type GRPCRequest struct {
	Target   string              `json:"target" yaml:"target" validate:"networkaddress"`
	Service  string              `json:"service" yaml:"service" validate:"min(1)"`
	Method   string              `json:"method" yaml:"method" validate:"min(1)"`
	Metadata map[string][]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Message  map[string]any      `json:"message,omitempty" yaml:"message,omitempty"`
}

type GRPCRequestPattern struct {
	Service  string              `json:"service" yaml:"service" validate:"min(1)"`
	Method   string              `json:"method" yaml:"method" validate:"min(1)"`
	Metadata map[string][]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	// Message is a pointer to a map to allow distinguishing between an empty message and no message.
	// message: {} means "empty message", while no message field means "will be skipped in matching".
	Message *map[string]any `json:"message,omitempty" yaml:"message,omitempty"`
}

type GRPCResponse struct {
	Status   string              `json:"status" yaml:"status" validate:"oneof(OK,Canceled,Unknown,InvalidArgument,DeadlineExceeded,NotFound,AlreadyExists,PermissionDenied,ResourceExhausted,FailedPrecondition,Aborted,OutOfRange,Unimplemented,Internal,Unavailable,DataLoss,Unauthenticated)"`
	Metadata map[string][]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	// Message is a pointer to a map to allow distinguishing between an empty message and no message.
	// message: {} means "empty message", while no message field means "will be skipped in matching".
	Message *map[string]any `json:"message,omitempty" yaml:"message,omitempty"`
}
