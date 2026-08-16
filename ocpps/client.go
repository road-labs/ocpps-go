package ocpps

import (
	"bytes"
	"context"
	"fmt"
	"io"
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
}

// NewClient instantiates and returns a Client instance. serverURL is the peer endpoint calls will be POSTed to,
// soapURN is the OCPP SOAP URN advertised in outbound envelopes (e.g. "urn://Ocpp/Cs/2015/10/"), ocppIdentity is
// the charge point identity carried in the chargeBoxIdentity SOAP header, and from is the URL that identifies the
// client in the WS-Addressing From header (typically only meaningful when the caller also runs its own inbound
// SOAP endpoint that the peer will POST to). Additional behaviour (e.g. the request timeout) can be configured via
// ClientOption helpers exposed by the clientopt sub-package.
func NewClient(serverURL, soapURN, ocppIdentity, from string, opts ...ClientOption) (*Client, error) {
	if serverURL == "" || soapURN == "" || ocppIdentity == "" || from == "" {
		return nil, fmt.Errorf("serverURL, soapURN, ocppIdentity and from are required")
	}

	conf := newClientConfig(opts)

	cp := &Client{
		client: &http.Client{
			Timeout: conf.Timeout,
		},
		serverURL:    serverURL,
		soapURN:      soapURN,
		ocppIdentity: ocppIdentity,
		from:         from,
	}

	return cp, nil
}

// WriteCall dispatches a call to the peer, wrapped in a SOAP envelope, and returns the peer's response. Unlike the
// OCPP-J binding this is a synchronous, request/response operation: the underlying HTTP POST blocks until the peer
// has replied (or the client's timeout has been reached).
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

	if resp.StatusCode != http.StatusOK {
		resBody, _ := io.ReadAll(resp.Body)
		return message.CallResult{}, fmt.Errorf("unexpected status code %d (%s)", resp.StatusCode, string(resBody))
	}

	resBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return message.CallResult{}, err
	}

	resEnv, err := unmarshalCallResult(resBody)
	if err != nil {
		return message.CallResult{}, err
	}

	if expectedAction := call.Action + "Response"; resEnv.Action != expectedAction {
		return message.CallResult{}, fmt.Errorf("invalid action, expected %s, got %s", expectedAction, resEnv.Action)
	}

	return resEnv.CallResult, nil
}
