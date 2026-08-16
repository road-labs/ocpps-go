package message

type Call struct {
	MessageID string
	Action    string
	Payload   []byte
}
