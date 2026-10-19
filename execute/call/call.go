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
