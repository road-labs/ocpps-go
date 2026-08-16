package message

type Error string

const (
	SecurityError    Error = "SecurityError"
	IdentityMismatch Error = "IdentityMismatch"
	ProtocolError    Error = "ProtocolError"
	InternalError    Error = "InternalError"
	NotSupported     Error = "NotSupported"
)

type CallError struct {
	MessageID        string
	ErrorCode        string
	ErrorDescription string
}
