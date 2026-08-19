package ocpps

import (
	"context"

	"github.com/road-labs/ocpps-go/ocpps/message"
)

// ClientInfo describes a client party in the OCPP-S context by its OCPP identity and advertised From URL. On the
// server side it describes the peer that initiated an incoming call (derived from the inbound SOAP envelope's
// chargeBoxIdentity and From headers). On the client side it describes the local client itself — the identity and
// From URL configured on the Client.
type ClientInfo struct {
	OCPPIdentity string
	FromURL      string
}

// ResponseWriter is used from within a server hook to reply to an incoming call. Exactly one of WriteCallResult or
// WriteCallError should be invoked for a given call; subsequent writes will return an error.
type ResponseWriter interface {
	// WriteCallResult replies to the call with a successful result payload.
	WriteCallResult(context.Context, message.CallResult) error
	// WriteCallError replies to the call with a SOAP fault carrying an OCPP-S error code and description.
	WriteCallError(context.Context, message.CallError) error
}

// ServerHooks defines the callbacks used by a Server to notify the caller about incoming calls and outgoing replies.
type ServerHooks interface {
	// OnCallRead is invoked when the server receives a call from a peer. A reply must be sent by calling
	// WriteCallResult or WriteCallError on the supplied ResponseWriter; if the hook returns a non-nil error and no
	// reply has been written, the server will send a generic InternalError call error on the caller's behalf.
	OnCallRead(context.Context, *ClientInfo, message.Call, ResponseWriter) error
	// OnCallResultWritten is invoked after a successful call result has been written to the peer.
	OnCallResultWritten(context.Context, *ClientInfo, message.CallResult) error
	// OnCallErrorWritten is invoked after a call error has been written to the peer.
	OnCallErrorWritten(context.Context, *ClientInfo, message.CallError) error
}
