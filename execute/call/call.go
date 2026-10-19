package call

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
