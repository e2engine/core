package reporter

type Config struct {
	// DumpPayload enables dumping of request and response payloads in the
	// test execution report. Applies to both HTTP and gRPC requests and responses.
	// When disabled, payloads are replaced with a placeholder string.
	DumpPayload bool `json:"dump_payload" yaml:"dump_payload" env:"DUMP_PAYLOAD"`

	// SkipRedactingHeadersMetadata disables redaction of builtin and
	// additional sensitive headers and metadata.
	SkipRedactingHeadersMetadata bool `json:"skip_redacting_headers_metadata" yaml:"skip_redacting_headers_metadata" env:"SKIP_REDACTING_HEADERS_METADATA"`
	// RedactHeaders contains additional sensitive headers to redact.
	RedactHeaders []string `json:"redact_headers" yaml:"redact_headers" env:"REDACT_HEADERS"`
	// RedactMetadata contains additional sensitive metadata to redact.
	RedactMetadata []string `json:"redact_metadata" yaml:"redact_metadata" env:"REDACT_METADATA"`

	// SkipRedactingQueryParams disables redaction of builtin and
	// additional request query parameters in the test execution report.
	SkipRedactingQueryParams bool `json:"skip_redacting_query_params" yaml:"skip_redacting_query_params" env:"SKIP_REDACTING_QUERY_PARAMS"`
	// RedactQueryParams contains additional sensitive request query parameters to redact.
	RedactQueryParams []string `json:"redact_query_params" yaml:"redact_query_params" env:"REDACT_QUERY_PARAMS"`
}
