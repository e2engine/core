package address_test

import (
	"testing"

	"github.com/e2engine/core/execute/address"
)

func TestNewResolver(t *testing.T) {
	tests := []struct {
		name    string
		hosts   map[string]string
		wantErr bool
	}{
		{
			name: "nil mappings",
		},
		{
			name:  "empty mappings",
			hosts: map[string]string{},
		},
		{
			name: "valid mappings",
			hosts: map[string]string{
				"telemetry-service": "127.0.0.1",
				"dynamodb":          "localhost",
			},
		},
		{
			name: "empty source hostname",
			hosts: map[string]string{
				"": "127.0.0.1",
			},
			wantErr: true,
		},
		{
			name: "empty destination hostname",
			hosts: map[string]string{
				"dynamodb": "",
			},
			wantErr: true,
		},
		{
			name: "source contains port",
			hosts: map[string]string{
				"dynamodb:8000": "127.0.0.1",
			},
			wantErr: true,
		},
		{
			name: "destination contains port",
			hosts: map[string]string{
				"dynamodb": "127.0.0.1:8000",
			},
			wantErr: true,
		},
		{
			name: "source contains scheme",
			hosts: map[string]string{
				"http://dynamodb": "127.0.0.1",
			},
			wantErr: true,
		},
		{
			name: "destination contains scheme",
			hosts: map[string]string{
				"dynamodb": "http://127.0.0.1",
			},
			wantErr: true,
		},
		{
			name: "source contains whitespace",
			hosts: map[string]string{
				"my service": "127.0.0.1",
			},
			wantErr: true,
		},
		{
			name: "destination contains whitespace",
			hosts: map[string]string{
				"dynamodb": "my host",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver, err := address.NewResolver(tt.hosts)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if resolver == nil {
				t.Fatal("expected non-nil resolver")
			}
		})
	}
}

func TestNewResolver_DefensiveCopy(t *testing.T) {
	hosts := map[string]string{
		"dynamodb": "127.0.0.1",
	}

	resolver, err := address.NewResolver(hosts)
	if err != nil {
		t.Fatalf("NewResolver() error: %v", err)
	}

	// Mutating the original map must not affect the resolver.
	hosts["dynamodb"] = "192.168.1.10"
	hosts["telemetry-service"] = "127.0.0.1"

	got, err := resolver.ResolveHTTPURL("http://dynamodb:8000")
	if err != nil {
		t.Fatalf("ResolveHTTPURL() error: %v", err)
	}

	if want := "http://127.0.0.1:8000"; got != want {
		t.Errorf("ResolveHTTPURL() = %q, want %q", got, want)
	}

	got, err = resolver.ResolveHTTPURL("http://telemetry-service:3000")
	if err != nil {
		t.Fatalf("ResolveHTTPURL() error: %v", err)
	}

	if want := "http://telemetry-service:3000"; got != want {
		t.Errorf("ResolveHTTPURL() = %q, want %q", got, want)
	}
}

func TestResolver_ResolveHTTPURL(t *testing.T) {
	tests := []struct {
		name    string
		hosts   map[string]string
		input   string
		want    string
		wantErr bool
	}{
		{
			name: "replace hostname",
			hosts: map[string]string{
				"telemetry-service": "127.0.0.1",
			},
			input: "http://telemetry-service:3000/health",
			want:  "http://127.0.0.1:3000/health",
		},
		{
			name: "replace hostname with another hostname",
			hosts: map[string]string{
				"telemetry-service": "telemetry.internal",
			},
			input: "http://telemetry-service:3000/health",
			want:  "http://telemetry.internal:3000/health",
		},
		{
			name: "preserve HTTPS scheme",
			hosts: map[string]string{
				"api": "api.internal",
			},
			input: "https://api:8443/v1/events",
			want:  "https://api.internal:8443/v1/events",
		},
		{
			name: "preserve path and query",
			hosts: map[string]string{
				"api": "127.0.0.1",
			},
			input: "http://api:3000/v1/events?limit=10&offset=20",
			want:  "http://127.0.0.1:3000/v1/events?limit=10&offset=20",
		},
		{
			name: "preserve escaped path",
			hosts: map[string]string{
				"api": "127.0.0.1",
			},
			input: "http://api:3000/v1/items/a%2Fb",
			want:  "http://127.0.0.1:3000/v1/items/a%2Fb",
		},
		{
			name: "replace hostname without port",
			hosts: map[string]string{
				"api": "api.internal",
			},
			input: "https://api/health",
			want:  "https://api.internal/health",
		},
		{
			name: "unmapped hostname",
			hosts: map[string]string{
				"dynamodb": "127.0.0.1",
			},
			input: "http://telemetry-service:3000/health",
			want:  "http://telemetry-service:3000/health",
		},
		{
			name:  "no mappings",
			input: "http://telemetry-service:3000/health",
			want:  "http://telemetry-service:3000/health",
		},
		{
			name: "external hostname unchanged",
			hosts: map[string]string{
				"telemetry-service": "127.0.0.1",
			},
			input: "https://api.telemetry.com/health",
			want:  "https://api.telemetry.com/health",
		},
		{
			name: "explicit external hostname mapping",
			hosts: map[string]string{
				"api.telemetry.com": "staging.telemetry.internal",
			},
			input: "https://api.telemetry.com/v1/events",
			want:  "https://staging.telemetry.internal/v1/events",
		},
		{
			name: "IPv4 destination",
			hosts: map[string]string{
				"api": "192.168.1.10",
			},
			input: "http://api:3000/health",
			want:  "http://192.168.1.10:3000/health",
		},
		{
			name: "IPv6 source hostname unchanged",
			hosts: map[string]string{
				"api": "127.0.0.1",
			},
			input: "http://[::1]:3000/health",
			want:  "http://[::1]:3000/health",
		},
		{
			name:    "invalid URL",
			input:   "://invalid",
			wantErr: true,
		},
		{
			name:    "missing hostname",
			input:   "http:///health",
			wantErr: true,
		},
		{
			name:    "unsupported scheme",
			input:   "ftp://api:3000/health",
			wantErr: true,
		},
		{
			name:    "relative URL",
			input:   "/health",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver, err := address.NewResolver(tt.hosts)
			if err != nil {
				t.Fatalf("NewResolver() error: %v", err)
			}

			got, err := resolver.ResolveHTTPURL(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Fatalf(
						"ResolveHTTPURL(%q): expected error, got %q",
						tt.input,
						got,
					)
				}

				return
			}

			if err != nil {
				t.Fatalf(
					"ResolveHTTPURL(%q) error: %v",
					tt.input,
					err,
				)
			}

			if got != tt.want {
				t.Errorf(
					"ResolveHTTPURL(%q) = %q, want %q",
					tt.input,
					got,
					tt.want,
				)
			}
		})
	}
}

func TestResolver_ResolveGRPCTarget(t *testing.T) {
	tests := []struct {
		name    string
		hosts   map[string]string
		input   string
		want    string
		wantErr bool
	}{
		{
			name: "replace hostname",
			hosts: map[string]string{
				"account": "127.0.0.1",
			},
			input: "account:50051",
			want:  "127.0.0.1:50051",
		},
		{
			name: "replace hostname with another hostname",
			hosts: map[string]string{
				"account": "account.internal",
			},
			input: "account:50051",
			want:  "account.internal:50051",
		},
		{
			name: "preserve port",
			hosts: map[string]string{
				"account": "192.168.1.10",
			},
			input: "account:8443",
			want:  "192.168.1.10:8443",
		},
		{
			name: "unmapped hostname",
			hosts: map[string]string{
				"dynamodb": "127.0.0.1",
			},
			input: "account:50051",
			want:  "account:50051",
		},
		{
			name:  "no mappings",
			input: "account:50051",
			want:  "account:50051",
		},
		{
			name: "IPv4 target unchanged",
			hosts: map[string]string{
				"account": "127.0.0.1",
			},
			input: "192.168.1.20:50051",
			want:  "192.168.1.20:50051",
		},
		{
			name: "IPv6 target unchanged",
			hosts: map[string]string{
				"account": "127.0.0.1",
			},
			input: "[::1]:50051",
			want:  "[::1]:50051",
		},
		{
			name:    "missing port",
			input:   "account",
			wantErr: true,
		},
		{
			name:    "missing colon",
			input:   "account50051",
			wantErr: true,
		},
		{
			name:    "unbracketed IPv6",
			input:   "::1:50051",
			wantErr: true,
		},
		{
			name:    "empty target",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver, err := address.NewResolver(tt.hosts)
			if err != nil {
				t.Fatalf("NewResolver() error: %v", err)
			}

			got, err := resolver.ResolveGRPCTarget(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Fatalf(
						"ResolveGRPCTarget(%q): expected error, got %q",
						tt.input,
						got,
					)
				}

				return
			}

			if err != nil {
				t.Fatalf(
					"ResolveGRPCTarget(%q) error: %v",
					tt.input,
					err,
				)
			}

			if got != tt.want {
				t.Errorf(
					"ResolveGRPCTarget(%q) = %q, want %q",
					tt.input,
					got,
					tt.want,
				)
			}
		})
	}
}
