package model

import "github.com/e2engine/core/model/matcher"

type GRPCRequest struct {
	// Target is the gRPC server address to connect to.
	Target string `json:"target" yaml:"target" validate:"networkaddress"`

	// Proto is the gRPC proto specification used to encode and decode messages.
	Proto *GRPCProtoSpec `json:"proto,omitempty" yaml:"proto,omitempty" validateElem:"omitempty,dive"`

	Service  string              `json:"service" yaml:"service" validate:"min(1)"`
	Method   string              `json:"method" yaml:"method" validate:"min(1)"`
	Metadata map[string][]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Message  map[string]any      `json:"message,omitempty" yaml:"message,omitempty"`
}

type GRPCRequestPattern struct {
	Service  string                      `json:"service" yaml:"service" validate:"min(1)"`
	Method   string                      `json:"method" yaml:"method" validate:"min(1)"`
	Metadata map[string][]matcher.String `json:"metadata,omitempty" yaml:"metadata,omitempty" validateElem:"stringmatchers"`

	// Message is nil when message matching should be skipped.
	// A non-nil matcher defines how the protobuf message is matched.
	// For equals, Value contains the protobuf message encoded as JSON.
	Message *matcher.GRPCMessage `json:"message,omitempty" yaml:"message,omitempty" validate:"grpcmessagematcher"`
}

// GRPCResponse describes observed gRPC response or a Fixture Then response.
// Validation is needed for Fixture.Then validation.
type GRPCResponse struct {
	Status   string              `json:"status" yaml:"status" validate:"oneof(OK,Canceled,Unknown,InvalidArgument,DeadlineExceeded,NotFound,AlreadyExists,PermissionDenied,ResourceExhausted,FailedPrecondition,Aborted,OutOfRange,Unimplemented,Internal,Unavailable,DataLoss,Unauthenticated)"`
	Metadata map[string][]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Message  map[string]any      `json:"message,omitempty" yaml:"message,omitempty"`
}

type GRPCResponsePattern struct {
	Status   string                      `json:"status" yaml:"status" validate:"oneof(OK,Canceled,Unknown,InvalidArgument,DeadlineExceeded,NotFound,AlreadyExists,PermissionDenied,ResourceExhausted,FailedPrecondition,Aborted,OutOfRange,Unimplemented,Internal,Unavailable,DataLoss,Unauthenticated)"`
	Metadata map[string][]matcher.String `json:"metadata,omitempty" yaml:"metadata,omitempty" validateElem:"stringmatchers"`

	// Message is nil when message matching should be skipped.
	// A non-nil matcher defines how the protobuf message is matched.
	// For equals, Value contains the protobuf message encoded as JSON.
	Message *matcher.GRPCMessage `json:"message,omitempty" yaml:"message,omitempty" validate:"grpcmessagematcher"`
}
