package ocpps

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/road-labs/ocpps-go/ocpps/message"
)

// Client represents a client party in the OCPP-S context. It sends SOAP-wrapped OCPP calls to a peer's HTTP
// endpoint and returns the peer's reply. In the OCPP-S binding both parties (charging station and central system)
// expose a SOAP endpoint, so this type can be used on either side to dispatch calls to the other.
type Client struct {
	client       *http.Client
	serverURL    string
	soapURN      string
	ocppIdentity string
	from         string
	hooks        ClientHooks
	logger       *slog.Logger
}

// info returns the ClientInfo describing this Client, used as context for hook invocations.
func (c *Client) info() *ClientInfo {
	return &ClientInfo{
		OCPPIdentity: c.ocppIdentity,
		FromURL:      c.from,
	}
}

// CallError is a simple wrapper around message.CallError which implements the error interface.
type CallError struct {
	message.CallError
}

func (ce *CallError) Error() string {
	return fmt.Sprintf("call error: %v", ce.CallError)
}

// NewClient instantiates and returns a Client instance. serverURL is the peer endpoint calls will be POSTed to,
// soapURN is the OCPP SOAP URN advertised in outbound envelopes (e.g. "urn://Ocpp/Cs/2015/10/"), ocppIdentity is
// the charge point identity carried in the chargeBoxIdentity SOAP header, and from is the URL that identifies the
// client in the WS-Addressing From header (typically only meaningful when the caller also runs its own inbound
// SOAP endpoint that the peer will POST to). Hooks are invoked for each successful send and each reply.
// Additional behaviour (e.g. the request timeout) can be configured via ClientOption helpers exposed by the
// clientopt sub-package.
func NewClient(serverURL, soapURN, ocppIdentity, from string, hooks ClientHooks, opts ...ClientOption) (*Client, error) {
	if serverURL == "" || soapURN == "" || ocppIdentity == "" || from == "" {
		return nil, fmt.Errorf("serverURL, soapURN, ocppIdentity and from are required")
	}

	conf := newClientConfig(opts)

	return &Client{
		client: &http.Client{
			Timeout: conf.Timeout,
		},
		serverURL:    serverURL,
		soapURN:      soapURN,
		ocppIdentity: ocppIdentity,
		from:         from,
		hooks:        hooks,
		logger:       conf.Logger,
	}, nil
}

// WriteCall dispatches a call to the peer, wrapped in a SOAP envelope, and returns the peer's response. Unlike the
// OCPP-J binding this is a synchronous, request/response operation: the underlying HTTP POST blocks until the peer
// has replied (or the client's timeout has been reached).
//
// If the peer replies with a non-2xx HTTP status carrying a well-formed SOAP fault, the fault is parsed into a
// message.CallError and returned as a *CallError. Other non-2xx responses return a plain error carrying the status
// code and response body.
//
// The action on the returned CallResult must match the outbound call's action with a "Response" suffix (as per the
// OCPP-S action naming convention), otherwise an error is returned.
func (c *Client) WriteCall(ctx context.Context, call message.Call) (message.CallResult, error) {
	reqBody, err := marshalCall(callEnvelope{
		Call:         call,
		SOAPURN:      c.soapURN,
		OCPPIdentity: c.ocppIdentity,
		From:         c.from,
		To:           c.serverURL,
	})
	if err != nil {
		return message.CallResult{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.serverURL, bytes.NewReader(reqBody))
	if err != nil {
		return message.CallResult{}, err
	}

	req.Header.Set("Accept", "application/soap+xml")
	req.Header.Set("Content-Type", "application/soap+xml")

	resp, err := c.client.Do(req)
	if err != nil {
		return message.CallResult{}, err
	}
	defer resp.Body.Close()

	info := c.info()
	if err := c.hooks.OnCallWritten(ctx, info, call); err != nil {
		return message.CallResult{}, fmt.Errorf("failed to invoke call written hook: %w", err)
	}

	resBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return message.CallResult{}, err
	}

	if resp.StatusCode != http.StatusOK {
		if callErr, parseErr := unmarshalCallError(resBody, call.MessageID); parseErr == nil {
			if err := c.hooks.OnCallErrorRead(ctx, info, callErr); err != nil {
				c.logger.Error("failed to invoke call error read hook", slog.Any("error", err))
			}
			return message.CallResult{}, &CallError{CallError: callErr}
		}
		return message.CallResult{}, fmt.Errorf("unexpected status code %d (%s)", resp.StatusCode, string(resBody))
	}

	resEnv, err := unmarshalCallResult(resBody, call.MessageID)
	if err != nil {
		return message.CallResult{}, err
	}

	if expectedAction := call.Action + "Response"; resEnv.Action != expectedAction {
		return message.CallResult{}, fmt.Errorf("invalid action, expected %s, got %s", expectedAction, resEnv.Action)
	}

	if err := c.hooks.OnCallResultRead(ctx, info, resEnv.CallResult); err != nil {
		c.logger.Error("failed to invoke call result read hook", slog.Any("error", err))
	}

	return resEnv.CallResult, nil
}
