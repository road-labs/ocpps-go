package ocpps

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/road-labs/ocpps-go/ocpps/message"
)

// soapResponseWriter is the concrete ResponseWriter implementation supplied to server hooks. It renders replies as
// SOAP envelopes onto the underlying http.ResponseWriter, and prevents more than one reply being written for a
// single request.
type soapResponseWriter struct {
	writer        http.ResponseWriter
	req           *callEnvelope
	soapURN       string
	advertiseAddr string
	hooks         ServerHooks
	logger        *slog.Logger
	written       bool
}

func (w *soapResponseWriter) WriteCallResult(ctx context.Context, callResult message.CallResult) error {
	if w.req == nil {
		return errors.New("request must be set to write call result")
	}
	env := callResultEnvelope{
		CallResult: callResult,
		SOAPURN:    w.soapURN,
		// The WS-Addressing MessageID on the response echoes the request's MessageID rather than being freshly
		// generated. OCPP-S 1.6 §3.5 treats MessageID as the correlation key between request and response.
		MessageID:    w.req.Call.MessageID,
		Action:       w.req.Call.Action + "Response",
		OCPPIdentity: w.req.OCPPIdentity,
		From:         w.advertiseAddr,
		To:           w.req.From,
	}
	if err := w.writeCallResultEnvelope(env); err != nil {
		return err
	}
	if err := w.hooks.OnCallResultWritten(ctx, w.clientInfo(), callResult); err != nil {
		w.logger.Error("failed to invoke call result written hook", slog.Any("error", err))
	}
	return nil
}

func (w *soapResponseWriter) WriteCallError(ctx context.Context, callError message.CallError) error {
	env := callErrorEnvelope{
		CallError: callError,
		SOAPURN:   w.soapURN,
		From:      w.advertiseAddr,
	}
	if w.req != nil {
		// Echo the request's MessageID; see WriteCallResult for the rationale.
		env.MessageID = w.req.Call.MessageID
		env.To = w.req.From
		env.OCPPIdentity = w.req.OCPPIdentity
		env.Action = w.req.Call.Action + "Response"
	} else {
		// Pre-parse errors (invalid HTTP method, malformed envelope) have no request MessageID to echo - fall back
		// to a fresh UUID so the WS-Addressing MessageID header is still populated.
		env.MessageID = uuid.NewString()
	}
	if err := w.writeCallErrorEnvelope(env); err != nil {
		return err
	}
	// Only fire the hook when we know who the peer is; pre-parse errors don't have a ClientInfo.
	if w.req != nil {
		if err := w.hooks.OnCallErrorWritten(ctx, w.clientInfo(), callError); err != nil {
			w.logger.Error("failed to invoke call error written hook", slog.Any("error", err))
		}
	}
	return nil
}

// clientInfo builds a ClientInfo from the parsed request envelope. Only safe to call after w.req has been set.
func (w *soapResponseWriter) clientInfo() *ClientInfo {
	return &ClientInfo{
		OCPPIdentity: w.req.OCPPIdentity,
		FromURL:      w.req.From,
	}
}

func (w *soapResponseWriter) writeCallResultEnvelope(env callResultEnvelope) error {
	resBody, err := marshalCallResult(env)
	if err != nil {
		http.Error(w.writer, "Internal Server Error", http.StatusInternalServerError)
		w.logger.Warn("failed to marshal call result", slog.Any("error", err))
		return err
	}
	return w.write(http.StatusOK, resBody)
}

func (w *soapResponseWriter) writeCallErrorEnvelope(env callErrorEnvelope) error {
	resBody, err := marshalCallError(env)
	if err != nil {
		w.logger.Warn("failed to marshal call error", slog.Any("error", err))
		return err
	}
	status := http.StatusInternalServerError
	if faultCodeFor(env.CallError.ErrorCode) == faultCodeSender {
		status = http.StatusBadRequest
	}
	return w.write(status, resBody)
}

func (w *soapResponseWriter) write(statusCode int, resBody []byte) error {
	if w.written {
		return errors.New("response has already been written")
	}

	w.written = true

	w.writer.Header().Set("Content-Type", "application/soap+xml")
	w.writer.WriteHeader(statusCode)

	_, err := w.writer.Write(resBody)
	return err
}
