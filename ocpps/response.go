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
	written       bool
}

func (w *soapResponseWriter) WriteCallResult(_ context.Context, callResult message.CallResult) error {
	if w.req == nil {
		return errors.New("request must be set to write call result")
	}
	env := callResultEnvelope{
		CallResult:   callResult,
		SOAPURN:      w.soapURN,
		MessageID:    uuid.NewString(),
		Action:       w.req.Call.Action + "Response",
		OCPPIdentity: w.req.OCPPIdentity,
		From:         w.advertiseAddr,
		To:           w.req.From,
	}
	return w.writeCallResultEnvelope(env)
}

func (w *soapResponseWriter) WriteCallError(_ context.Context, callError message.CallError) error {
	env := callErrorEnvelope{
		CallError: callError,
		SOAPURN:   w.soapURN,
		MessageID: uuid.NewString(),
		From:      w.advertiseAddr,
	}
	if w.req != nil {
		env.To = w.req.From
		env.OCPPIdentity = w.req.OCPPIdentity
		env.Action = w.req.Call.Action + "Response"
	}
	return w.writeCallErrorEnvelope(env)
}

func (w *soapResponseWriter) writeCallResultEnvelope(env callResultEnvelope) error {
	resBody, err := marshalCallResult(env)
	if err != nil {
		http.Error(w.writer, "Internal Server Error", http.StatusInternalServerError)
		slog.Warn("failed to marshal call error", slog.Any("error", err))
		return err
	}
	return w.write(http.StatusOK, resBody)
}

func (w *soapResponseWriter) writeCallErrorEnvelope(env callErrorEnvelope) error {
	resBody, err := marshalCallError(env)
	if err != nil {
		slog.Warn("failed to marshal call error", slog.Any("error", err))
		return err
	}
	return w.write(http.StatusInternalServerError, resBody)
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
