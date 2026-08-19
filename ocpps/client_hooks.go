package ocpps

import (
	"context"

	"github.com/road-labs/ocpps-go/ocpps/message"
)

type ClientHooks interface {
	// OnCallWritten is invoked after a call has been dispatched to the peer and the HTTP round-trip has completed.
	OnCallWritten(context.Context, *ClientInfo, message.Call) error
	// OnCallResultRead is invoked when the peer replied with a successful call result.
	OnCallResultRead(context.Context, *ClientInfo, message.CallResult) error
	// OnCallErrorRead is invoked when the peer replied with a SOAP fault that mapped to an OCPP call error.
	OnCallErrorRead(context.Context, *ClientInfo, message.CallError) error
}
