package ocpps

import (
	"crypto/tls"
	"log/slog"
	"time"
)

type ServerConfig struct {
	Path              string
	HealthPath        string
	TLSConfig         *tls.Config
	ReadHeaderTimeout time.Duration
	Logger            *slog.Logger
}

type ServerOption func(*ServerConfig)

const (
	defaultServerPath        = "/"
	defaultHealthPath        = "/health"
	defaultReadHeaderTimeout = 30 * time.Second
)

func newServerConfig(opts []ServerOption) *ServerConfig {
	c := &ServerConfig{}
	for _, opt := range opts {
		opt(c)
	}
	if c.Path == "" {
		c.Path = defaultServerPath
	}
	if c.HealthPath == "" {
		c.HealthPath = defaultHealthPath
	}
	if c.ReadHeaderTimeout == 0 {
		c.ReadHeaderTimeout = defaultReadHeaderTimeout
	}
	if c.Logger == nil {
		c.Logger = slog.New(slog.DiscardHandler)
	}
	return c
}
