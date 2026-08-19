package ocpps

import (
	"log/slog"
	"time"
)

type ClientConfig struct {
	Timeout time.Duration
	Logger  *slog.Logger
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
	if c.Logger == nil {
		c.Logger = slog.New(slog.DiscardHandler)
	}
	return c
}
