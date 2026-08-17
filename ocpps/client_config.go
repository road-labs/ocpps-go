package ocpps

import "time"

type ClientConfig struct {
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
