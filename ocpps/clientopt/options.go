package clientopt

import (
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
