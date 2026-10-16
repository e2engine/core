package address

import (
	"net"
	"net/url"
	"strings"

	"github.com/ygrebnov/errorc"

	"github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/keys"
)

// Resolver substitutes explicitly mapped hostnames in outgoing addresses.
// Hostname mappings are restricted to DNS names and IPv4 addresses.
type Resolver struct {
	hosts map[string]string
}

func NewResolver(hosts map[string]string) (*Resolver, error) {
	copyHosts := make(map[string]string, len(hosts))

	for from, to := range hosts {
		if from == "" || to == "" ||
			strings.ContainsAny(from, ":/ ") ||
			strings.ContainsAny(to, ":/ ") {
			return nil, errorc.With(
				errors.ErrInvalidHostnameMapping,
				errorc.String(keys.HostnameMappingFrom, from),
				errorc.String(keys.HostnameMappingTo, to),
			)
		}

		copyHosts[from] = to
	}

	return &Resolver{hosts: copyHosts}, nil
}

// ResolveHTTPURL replaces only the hostname of an absolute HTTP(S) URL.
func (r *Resolver) ResolveHTTPURL(value string) (string, error) {
	u, err := url.Parse(value)
	if err != nil {
		return "", errorc.With(
			errors.ErrInvalidHTTPURL,
			errorc.String(keys.URL, value),
			errorc.Error(keys.Cause, err),
		)
	}

	if (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return "", errorc.With(
			errors.ErrInvalidHTTPURL,
			errorc.String(keys.URL, value),
			errorc.Error(keys.Cause, err),
		)
	}

	if mapped, ok := r.hosts[u.Hostname()]; ok {
		if port := u.Port(); port != "" {
			u.Host = net.JoinHostPort(mapped, port)
		} else if strings.Contains(mapped, ":") {
			u.Host = "[" + strings.Trim(mapped, "[]") + "]"
		} else {
			u.Host = mapped
		}
	}

	return u.String(), nil
}

// ResolveGRPCTarget replaces only the hostname in a host:port target.
func (r *Resolver) ResolveGRPCTarget(value string) (string, error) {
	host, port, err := net.SplitHostPort(value)
	if err != nil {
		return "", errorc.With(
			errors.ErrInvalidGRPCTarget,
			errorc.String(keys.URL, value),
			errorc.Error(keys.Cause, err),
		)
	}

	if mapped, ok := r.hosts[host]; ok {
		return net.JoinHostPort(mapped, port), nil
	}

	return value, nil
}
