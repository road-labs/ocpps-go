package ocpps

import (
	"context"
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
	listenAddr    string
	soapURN       string
	advertiseAddr string
	hooks         ServerHooks
	conf          *ServerConfig
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

	return &Server{
		listenAddr:    listenAddr,
		soapURN:       soapURN,
		advertiseAddr: advertiseAddr,
		hooks:         hooks,
		conf:          newServerConfig(opts),
	}, nil
}

// Start runs the server. This is a blocking operation, and will not return until the server is interrupted in some
// way (e.g. the supplied context is cancelled). Two endpoints are registered: the OCPP-S handler on the configured
// path (defaulting to "/", which matches all paths), and a health endpoint at the configured health path
// (defaulting to "/health").
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc(s.conf.HealthPath, s.handleHealth)
	mux.HandleFunc(s.conf.Path, s.handleSoap)

	server := http.Server{
		Addr:    s.listenAddr,
		Handler: mux,
	}

	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		slog.Info(
			"ocpp-s server listening",
			slog.String("addr", s.listenAddr),
			slog.String("path", s.conf.Path),
		)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
// dependencies; callers wanting a richer readiness check should embed the server via Handler and layer their own
// health endpoint alongside it.
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
