package address

import (
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/ygrebnov/errorc"

	"github.com/e2engine/core/pkg/errors"
)

// Resolver substitutes explicitly mapped hostnames in outgoing addresses.
// Hostname mappings are restricted to DNS names and IPv4 addresses.
type Resolver struct {
	hosts map[string]string
}

func NewResolver(hosts map[string]string) (*Resolver, error) {
	copyHosts := make(map[string]string, len(hosts))

	for from, to := range hosts {
		if !validHostname(from) || !validHostname(to) {
			return nil, errorc.With(
				errors.ErrInvalidHostnameMapping,
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
		return "", errors.ErrInvalidHTTPURL
	}

	if (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return "", errors.ErrInvalidHTTPURL
	}

	if u.User != nil {
		return "", errors.ErrInvalidHTTPURL
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
		return "", errors.ErrInvalidGRPCTarget
	}

	if host == "" {
		return "", errors.ErrInvalidGRPCTarget
	}

	portNumber, err := strconv.ParseUint(port, 10, 16)
	if err != nil || portNumber == 0 {
		return "", errors.ErrInvalidGRPCTarget
	}

	if mapped, ok := r.hosts[host]; ok {
		return net.JoinHostPort(mapped, port), nil
	}

	return value, nil
}

func validHostname(value string) bool {
	if value == "" || len(value) > 253 {
		return false
	}

	if ip := net.ParseIP(value); ip != nil {
		return ip.To4() != nil
	}

	for _, label := range strings.Split(value, ".") {
		if label == "" || len(label) > 63 {
			return false
		}

		if label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}

		for _, c := range label {
			if (c >= 'a' && c <= 'z') ||
				(c >= 'A' && c <= 'Z') ||
				(c >= '0' && c <= '9') ||
				c == '-' {
				continue
			}

			return false
		}
	}

	return true
}
