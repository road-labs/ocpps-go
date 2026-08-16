package ocpps

import "time"

type ClientConfig struct {
	// Timeout is the total time an outbound call is allowed to take before the underlying HTTP request is abandoned. It
	//covers connection, request writing, response reading, and any redirects.
	Timeout time.Duration
}

type ClientOption func(*ClientConfig)

const defaultClientTimeout = 15 * time.Second

func newClientConfig(opts []ClientOption) *ClientConfig {
	c := &ClientConfig{}
	for _, opt := range opts {
		opt(c)
	}
	if c.Timeout == 0 {
		c.Timeout = defaultClientTimeout
	}
	return c
}
