package ocpps

type ServerConfig struct {
	// Path is the HTTP path the server accepts SOAP-wrapped OCPP calls on.
	Path string
	// HealthPath is the HTTP path the server exposes a health endpoint on, which responds with a 200 and "OK".
	HealthPath string
}

type ServerOption func(*ServerConfig)

const (
	defaultServerPath = "/"
	defaultHealthPath = "/health"
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
	return c
}
