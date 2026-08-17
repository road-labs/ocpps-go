package serveropt

import (
	"crypto/tls"
	"log/slog"
	"time"

	"github.com/road-labs/ocpps-go/ocpps"
)

// WithPath specifies the HTTP path the server accepts SOAP-wrapped OCPP calls on.
func WithPath(path string) ocpps.ServerOption {
	return func(c *ocpps.ServerConfig) {
		c.Path = path
	}
}

// WithHealthEndpoint specifies the HTTP path a health endpoint is exposed on. This currently just exposes a single
// endpoint which returns a 200 with "OK" in the response body. The default is "/health".
func WithHealthEndpoint(path string) ocpps.ServerOption {
	return func(c *ocpps.ServerConfig) {
		c.HealthPath = path
	}
}

// WithTLSConfig configures TLS for the HTTP server. When set, the server listens for HTTPS connections; otherwise
// it listens in plaintext.
func WithTLSConfig(conf *tls.Config) ocpps.ServerOption {
	return func(c *ocpps.ServerConfig) {
		c.TLSConfig = conf
	}
}

// WithReadHeaderTimeout specifies the amount of time the server waits for a client to finish sending request
// headers before abandoning the connection. This is a mitigation against slowloris-style attacks. The default is
// 30 seconds.
func WithReadHeaderTimeout(d time.Duration) ocpps.ServerOption {
	return func(c *ocpps.ServerConfig) {
		c.ReadHeaderTimeout = d
	}
}

// WithLogger configures the logger the server uses for its own output. If not set, log output is discarded.
func WithLogger(logger *slog.Logger) ocpps.ServerOption {
	return func(c *ocpps.ServerConfig) {
		c.Logger = logger
	}
}
