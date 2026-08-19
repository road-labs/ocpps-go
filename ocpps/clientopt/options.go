package clientopt

import (
	"log/slog"
	"time"

	"github.com/road-labs/ocpps-go/ocpps"
)

// WithTimeout specifies the total time an outbound call is allowed to take before the underlying HTTP request is
// abandoned. The default is 15 seconds.
func WithTimeout(timeout time.Duration) ocpps.ClientOption {
	return func(c *ocpps.ClientConfig) {
		c.Timeout = timeout
	}
}

// WithLogger configures the logger the client uses for its own output. If not set, log output is discarded.
func WithLogger(logger *slog.Logger) ocpps.ClientOption {
	return func(c *ocpps.ClientConfig) {
		c.Logger = logger
	}
}
