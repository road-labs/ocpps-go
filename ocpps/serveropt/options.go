package serveropt

import (
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
