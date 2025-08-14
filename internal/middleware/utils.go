package middleware

import (
	"net"
	"net/http"
	"strings"
)

// GetClientIP extracts the client IP address from the request.
// It checks various headers set by proxies and load-balancers before
// falling back to the remote address.
func GetClientIP(r *http.Request) string {
	xForwardedFor := r.Header.Get("X-Forwarded-For")

	if xForwardedFor != "" {
		ips := strings.Split(xForwardedFor, ",")

		if len(ips) > 0 {
			ip := strings.TrimSpace(ips[0])

			if net.ParseIP(ip) != nil {
				return ip
			}
		}
	}

	xRealIP := r.Header.Get("X-Real-IP")

	if xRealIP != "" {
		if net.ParseIP(xRealIP) != nil {
			return xRealIP
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)

	if err != nil {
		return r.RemoteAddr
	}

	return host
}
