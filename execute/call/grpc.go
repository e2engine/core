package call

type GRPCCall struct {
	Request  GRPCRequest
	Response GRPCResponse
}

type GRPCRequest struct {
	RPC      string
	Metadata map[string][]string
	Message  []byte
}

type GRPCResponse struct {
	Status   string
	Metadata map[string][]string
	Message  []byte
}
