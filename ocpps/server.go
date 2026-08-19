package ocpps

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/road-labs/ocpps-go/ocpps/message"
)

// Server represents a server party in the OCPP-S context. It runs an HTTP server that accepts SOAP-wrapped OCPP
// calls on a configured path, and invokes the supplied hooks for each call received. In the OCPP-S binding both
// parties (charging station and central system) expose a SOAP endpoint, so this type can be used on either side.
type Server struct {
	listenAddr        string
	soapURN           string
	advertiseAddr     string
	path              string
	healthPath        string
	tlsConfig         *tls.Config
	readHeaderTimeout time.Duration
	hooks             ServerHooks
	logger            *slog.Logger
}

// NewServer creates a new server instance. listenAddr is the address the HTTP server will bind to (e.g. ":8080"),
// soapURN is the OCPP SOAP URN advertised in outbound envelopes (e.g. "urn://Ocpp/Cs/2015/10/"), advertiseAddr is
// the URL of this server as it should appear in the WS-Addressing From header of outbound envelopes, and hooks
// receive each incoming call. Additional behaviour (e.g. the request path) can be configured via ServerOption
// helpers exposed by the serveropt sub-package.
func NewServer(listenAddr string, soapURN string, advertiseAddr string, hooks ServerHooks, opts ...ServerOption) (*Server, error) {
	if listenAddr == "" || soapURN == "" || advertiseAddr == "" {
		return nil, errors.New("listenAddr, soapURN and advertiseAddr are required")
	}

	conf := newServerConfig(opts)

	return &Server{
		listenAddr:        listenAddr,
		soapURN:           soapURN,
		advertiseAddr:     advertiseAddr,
		path:              conf.Path,
		healthPath:        conf.HealthPath,
		tlsConfig:         conf.TLSConfig,
		readHeaderTimeout: conf.ReadHeaderTimeout,
		hooks:             hooks,
		logger:            conf.Logger,
	}, nil
}

// Start runs the server. This is a blocking operation, and will not return until the server is interrupted in some
// way (e.g. the supplied context is cancelled). Two endpoints are registered: the OCPP-S handler on the configured
// path (defaulting to "/", which matches all paths), and a health endpoint at the configured health path
// (defaulting to "/health"). When a TLS config is set the server listens for HTTPS connections; otherwise it
// listens in plaintext.
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc(s.healthPath, s.handleHealth)
	mux.HandleFunc(s.path, s.handleSoap)

	server := http.Server{
		Addr:              s.listenAddr,
		Handler:           mux,
		TLSConfig:         s.tlsConfig,
		ReadHeaderTimeout: s.readHeaderTimeout,
	}

	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		s.logger.Info(
			"ocpp-s server listening",
			slog.String("address", s.listenAddr),
			slog.Bool("tls", s.tlsConfig != nil),
		)
		var err error
		if s.tlsConfig != nil {
			err = server.ListenAndServeTLS("", "")
		} else {
			err = server.ListenAndServe()
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})
	eg.Go(func() error {
		<-ctx.Done()
		sCtx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		return server.Shutdown(sCtx) //nolint:contextcheck
	})
	return eg.Wait()
}

// handleHealth responds with a 200 and "OK" body. It's intentionally minimal and does not attempt to probe any
// dependencies; callers wanting a richer readiness check should run their own health endpoint on a separate port
// or in front of this server.
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

// handleSoap processes a single incoming HTTP request. It unmarshals the SOAP envelope into a Call, invokes the
// OnCallRead hook, and lets the hook write its reply via the supplied ResponseWriter.
func (s *Server) handleSoap(w http.ResponseWriter, req *http.Request) {
	resWriter := &soapResponseWriter{
		writer:        w,
		soapURN:       s.soapURN,
		advertiseAddr: s.advertiseAddr,
		hooks:         s.hooks,
		logger:        s.logger,
	}

	if req.Method != http.MethodPost {
		_ = resWriter.WriteCallError(req.Context(), message.CallError{
			ErrorCode:        string(message.ProtocolError),
			ErrorDescription: "Invalid HTTP Method",
		})
		return
	}

	reqBody, err := io.ReadAll(req.Body)
	if err != nil {
		_ = resWriter.WriteCallError(req.Context(), message.CallError{
			ErrorCode:        string(message.InternalError),
			ErrorDescription: err.Error(),
		})
		return
	}

	soapReq, err := unmarshalCall(reqBody)
	if err != nil {
		_ = resWriter.WriteCallError(req.Context(), message.CallError{
			ErrorCode:        string(message.ProtocolError),
			ErrorDescription: err.Error(),
		})
		return
	}

	resWriter.req = &soapReq

	clientInfo := &ClientInfo{
		OCPPIdentity: soapReq.OCPPIdentity,
		FromURL:      soapReq.From,
	}

	if err = s.hooks.OnCallRead(req.Context(), clientInfo, soapReq.Call, resWriter); err != nil {
		_ = resWriter.WriteCallError(req.Context(), message.CallError{
			MessageID:        soapReq.Call.MessageID,
			ErrorCode:        string(message.InternalError),
			ErrorDescription: "Internal Server Error",
		})
		return
	}
}
