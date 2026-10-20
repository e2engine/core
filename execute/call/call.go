package call

import (
	"slices"

	"github.com/e2engine/core/internal/util"
)

type Recorder interface {
	Record(call Call) error
	Err() error
}

type Reader interface {
	Get(f Filter) []Call
}

type Call struct {
	TestExecutionID string
	ServiceID       string

	HTTP *HTTPCall
	GRPC *GRPCCall

	MockFixtureMatched bool
}

// Size returns the size of variable-length data retained by the call.
// It excludes Go object, map, slice, and allocator overhead.
func (c Call) Size() int64 {
	size := int64(
		len(c.TestExecutionID) +
			len(c.ServiceID),
	)

	if c.HTTP != nil {
		size += int64(
			len(c.HTTP.Request.Method) +
				len(c.HTTP.Request.Path),
		)

		size += stringValuesSize(c.HTTP.Request.Query)
		size += stringValuesSize(c.HTTP.Request.Headers)
		size += int64(len(c.HTTP.Request.Body))

		size += stringValuesSize(c.HTTP.Response.Headers)
		size += int64(len(c.HTTP.Response.Body))
	}

	if c.GRPC != nil {
		size += int64(
			len(c.GRPC.Request.Service) +
				len(c.GRPC.Request.Method),
		)

		size += stringValuesSize(c.GRPC.Request.Metadata)
		size += int64(len(c.GRPC.Request.Message))

		size += int64(len(c.GRPC.Response.Status))
		size += stringValuesSize(c.GRPC.Response.Metadata)
		size += int64(len(c.GRPC.Response.Message))
	}

	return size
}

// Clone returns an independent copy of the call and its mutable data.
func (c Call) Clone() Call {
	clone := c

	if c.HTTP != nil {
		httpCall := *c.HTTP

		httpCall.Request.Query = util.CloneStringSliceMap(c.HTTP.Request.Query)
		httpCall.Request.Headers = c.HTTP.Request.Headers.Clone()
		httpCall.Request.Body = slices.Clone(c.HTTP.Request.Body)

		httpCall.Response.Headers = c.HTTP.Response.Headers.Clone()
		httpCall.Response.Body = slices.Clone(c.HTTP.Response.Body)

		clone.HTTP = &httpCall
	}

	if c.GRPC != nil {
		grpcCall := *c.GRPC

		grpcCall.Request.Metadata = util.CloneStringSliceMap(c.GRPC.Request.Metadata)
		grpcCall.Request.Message = slices.Clone(c.GRPC.Request.Message)

		grpcCall.Response.Metadata = util.CloneStringSliceMap(c.GRPC.Response.Metadata)
		grpcCall.Response.Message = slices.Clone(c.GRPC.Response.Message)

		clone.GRPC = &grpcCall
	}

	return clone
}

func stringValuesSize(values map[string][]string) int64 {
	var size int64

	for key, items := range values {
		size += int64(len(key))

		for _, item := range items {
			size += int64(len(item))
		}
	}

	return size
}
