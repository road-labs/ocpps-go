package ocpps

import (
	"context"

	"github.com/road-labs/ocpps-go/ocpps/message"
)

// ClientInfo describes the peer that initiated an incoming call, derived from headers on the inbound SOAP envelope.
type ClientInfo struct {
	// OCPPIdentity is the value of the chargeBoxIdentity SOAP header, uniquely identifying the peer charging
	// station.
	OCPPIdentity string
	// FromURL is the value of the WS-Addressing From header, i.e. the URL the peer advertises for requests.
	FromURL string
}

// ResponseWriter is used from within a server hook to reply to an incoming call. Exactly one of WriteCallResult or
// WriteCallError should be invoked for a given call; subsequent writes will return an error.
type ResponseWriter interface {
	// WriteCallResult replies to the call with a successful result payload.
	WriteCallResult(context.Context, message.CallResult) error
	// WriteCallError replies to the call with a SOAP fault carrying an OCPP-S error code and description.
	WriteCallError(context.Context, message.CallError) error
}

// ServerHooks defines the callbacks used by a Server to notify the caller about incoming calls.
type ServerHooks interface {
	// OnCallRead is invoked when the server receives a call from a peer. A reply must be sent by calling
	// WriteCallResult or WriteCallError on the supplied ResponseWriter; if the hook returns a non-nil error and no
	// reply has been written, the server will send a generic InternalError call error on the caller's behalf.
	OnCallRead(context.Context, *ClientInfo, message.Call, ResponseWriter) error
}
