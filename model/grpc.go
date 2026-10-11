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
	Message  map[string]any      `json:"message,omitempty" yaml:"message,omitempty"`
}

type GRPCResponse struct {
	Status   string              `json:"status" yaml:"status" validate:"oneof(OK,Canceled,Unknown,InvalidArgument,DeadlineExceeded,NotFound,AlreadyExists,PermissionDenied,ResourceExhausted,FailedPrecondition,Aborted,OutOfRange,Unimplemented,Internal,Unavailable,DataLoss,Unauthenticated)"`
	Metadata map[string][]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Message  map[string]any      `json:"message,omitempty" yaml:"message,omitempty"`
}
