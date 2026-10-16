package grpc

type addressResolver interface {
	ResolveGRPCTarget(value string) (string, error)
}

//go:generate go run -mod=mod go.uber.org/mock/mockgen -source=address_resolver.go -destination=mock_address_resolver_test.go -package=grpc
