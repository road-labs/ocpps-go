package ocpps

import (
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/road-labs/ocpps-go/ocpps/message"
)

type SOAPEnvelope struct {
	XMLName xml.Name `xml:"http://www.w3.org/2003/05/soap-envelope Envelope"`

	Header *SOAPHeader
	Body   SOAPBody
}

type SOAPHeader struct {
	XMLName                 xml.Name `xml:"http://www.w3.org/2003/05/soap-envelope Header"`
	ChargeBoxIdentityHeader ChargeBoxIdentityHeader
	ActionHeader            ActionHeader
	MessageIDHeader         MessageIDHeader
	FromHeader              FromHeader
	ReplyToHeader           ReplyToHeader
	ToHeader                ToHeader
	RelatesTo               *RelatesTo
}

type ChargeBoxIdentityHeader struct {
	XMLName  xml.Name `xml:"chargeBoxIdentity"`
	XMLNS    string   `xml:"xmlns,attr"`
	Identity string   `xml:",chardata"`
}

type ActionHeader struct {
	XMLName        xml.Name `xml:"http://www.w3.org/2005/08/addressing Action"`
	MustUnderstand bool     `xml:"http://www.w3.org/2003/05/soap-envelope mustUnderstand,attr"`
	Action         string   `xml:",chardata"`
}

type MessageIDHeader struct {
	XMLName   xml.Name `xml:"http://www.w3.org/2005/08/addressing MessageID"`
	MessageID string   `xml:",chardata"`
}

type FromHeader struct {
	XMLName xml.Name `xml:"http://www.w3.org/2005/08/addressing From"`
	From    Address
}

type ReplyToHeader struct {
	XMLName xml.Name `xml:"http://www.w3.org/2005/08/addressing ReplyTo"`
	ReplyTo Address
}

type ToHeader struct {
	XMLName        xml.Name `xml:"http://www.w3.org/2005/08/addressing To"`
	MustUnderstand bool     `xml:"http://www.w3.org/2003/05/soap-envelope mustUnderstand,attr"`
	To             string   `xml:",chardata"`
}

type RelatesTo struct {
	XMLName   xml.Name `xml:"http://www.w3.org/2005/08/addressing RelatesTo"`
	RelatesTo string   `xml:",chardata"`
}

type Address struct {
	XMLName xml.Name `xml:"http://www.w3.org/2005/08/addressing Address"`
	Address string   `xml:",chardata"`
}

type SOAPBody struct {
	XMLName xml.Name `xml:"http://www.w3.org/2003/05/soap-envelope Body"`
	Payload []byte   `xml:",innerxml"`
}

type SOAPFault struct {
	XMLName xml.Name `xml:"http://www.w3.org/2003/05/soap-envelope Fault"`
	Code    SOAPFaultCode
	Reason  string `xml:"Reason"`
	Node    string `xml:"Node,omitempty"`
	Role    string `xml:"Role,omitempty"`
	Detail  any    `xml:"Detail,omitempty"`
}

type SOAPFaultCode struct {
	XMLName xml.Name `xml:"http://www.w3.org/2003/05/soap-envelope Code"`
	Value   string   `xml:"Value"`
	Subcode *SOAPFaultSubCode
}

type SOAPFaultSubCode struct {
	XMLName xml.Name `xml:"Subcode"`
	XMLNS   string   `xml:"xmlns,attr"`
	Value   string   `xml:"Value"`
	Subcode *SOAPFaultSubCode
}

type callEnvelope struct {
	Call         message.Call
	SOAPURN      string
	OCPPIdentity string
	From         string
	To           string
}

type callResultEnvelope struct {
	CallResult   message.CallResult
	SOAPURN      string
	MessageID    string
	Action       string
	OCPPIdentity string
	From         string
	To           string
}

type callErrorEnvelope struct {
	CallError    message.CallError
	SOAPURN      string
	MessageID    string
	Action       string
	OCPPIdentity string
	From         string
	To           string
}

func marshalCall(env callEnvelope) ([]byte, error) {
	return xml.Marshal(SOAPEnvelope{
		Header: &SOAPHeader{
			ActionHeader: ActionHeader{
				MustUnderstand: true,
				Action:         fmt.Sprintf("/%s", env.Call.Action),
			},
			MessageIDHeader: MessageIDHeader{
				MessageID: env.Call.MessageID,
			},
			ReplyToHeader: ReplyToHeader{
				ReplyTo: Address{
					Address: "http://www.w3.org/2005/08/addressing/anonymous",
				},
			},
			ChargeBoxIdentityHeader: ChargeBoxIdentityHeader{
				XMLNS:    env.SOAPURN,
				Identity: env.OCPPIdentity,
			},
			FromHeader: FromHeader{
				From: Address{
					Address: env.From,
				},
			},
			ToHeader: ToHeader{
				To: env.To,
			},
		},
		Body: SOAPBody{
			Payload: env.Call.Payload,
		},
	})
}

func unmarshalCall(b []byte) (callEnvelope, error) {
	var env SOAPEnvelope
	if err := xml.Unmarshal(b, &env); err != nil {
		return callEnvelope{}, err
	}

	return callEnvelope{
		Call: message.Call{
			MessageID: env.Header.MessageIDHeader.MessageID,
			Action:    strings.TrimPrefix(env.Header.ActionHeader.Action, "/"),
			Payload:   env.Body.Payload,
		},
		OCPPIdentity: env.Header.ChargeBoxIdentityHeader.Identity,
		From:         env.Header.FromHeader.From.Address,
		To:           env.Header.ToHeader.To,
	}, nil
}

func marshalCallResult(env callResultEnvelope) ([]byte, error) {
	return xml.Marshal(SOAPEnvelope{
		Header: &SOAPHeader{
			ActionHeader: ActionHeader{
				MustUnderstand: true,
				Action:         fmt.Sprintf("/%s", env.Action),
			},
			MessageIDHeader: MessageIDHeader{
				MessageID: env.MessageID,
			},
			ReplyToHeader: ReplyToHeader{
				ReplyTo: Address{
					Address: "http://www.w3.org/2005/08/addressing/anonymous",
				},
			},
			ChargeBoxIdentityHeader: ChargeBoxIdentityHeader{
				XMLNS:    env.SOAPURN,
				Identity: env.OCPPIdentity,
			},
			FromHeader: FromHeader{
				From: Address{
					Address: env.From,
				},
			},
			ToHeader: ToHeader{
				To: env.To,
			},
			RelatesTo: &RelatesTo{
				RelatesTo: env.CallResult.MessageID,
			},
		},
		Body: SOAPBody{
			Payload: env.CallResult.Payload,
		},
	})
}

func unmarshalCallResult(b []byte) (callResultEnvelope, error) {
	var env SOAPEnvelope
	if err := xml.Unmarshal(b, &env); err != nil {
		return callResultEnvelope{}, err
	}

	var relatesTo string
	if env.Header.RelatesTo != nil {
		relatesTo = env.Header.RelatesTo.RelatesTo
	}

	if !strings.HasPrefix(env.Header.ActionHeader.Action, "/") {
		return callResultEnvelope{}, fmt.Errorf("action should have a leading slash, received %q", env.Header.ActionHeader.Action)
	}

	return callResultEnvelope{
		CallResult: message.CallResult{
			Payload:   env.Body.Payload,
			MessageID: relatesTo,
		},
		MessageID:    env.Header.MessageIDHeader.MessageID,
		Action:       strings.TrimPrefix(env.Header.ActionHeader.Action, "/"),
		OCPPIdentity: env.Header.ChargeBoxIdentityHeader.Identity,
		From:         env.Header.FromHeader.From.Address,
		To:           env.Header.ToHeader.To,
	}, nil
}

type faultCode string

const (
	faultCodeSender   faultCode = "Sender"
	faultCodeReceiver faultCode = "Receiver"
)

var errorCodes = map[message.Error]faultCode{
	message.SecurityError:    faultCodeSender,
	message.IdentityMismatch: faultCodeSender,
	message.ProtocolError:    faultCodeSender,
	message.InternalError:    faultCodeReceiver,
	message.NotSupported:     faultCodeReceiver,
}

func marshalCallError(env callErrorEnvelope) ([]byte, error) {
	header := &SOAPHeader{
		MessageIDHeader: MessageIDHeader{
			MessageID: env.MessageID,
		},
		ReplyToHeader: ReplyToHeader{
			ReplyTo: Address{
				Address: "http://www.w3.org/2005/08/addressing/anonymous",
			},
		},
		ChargeBoxIdentityHeader: ChargeBoxIdentityHeader{
			XMLNS:    env.SOAPURN,
			Identity: env.OCPPIdentity,
		},
		FromHeader: FromHeader{
			From: Address{
				Address: env.From,
			},
		},
		ToHeader: ToHeader{
			To: env.To,
		},
	}
	if env.Action != "" {
		header.ActionHeader = ActionHeader{
			MustUnderstand: true,
			Action:         fmt.Sprintf("/%s", env.Action),
		}
	}
	if env.CallError.MessageID != "" {
		header.RelatesTo = &RelatesTo{
			RelatesTo: env.CallError.MessageID,
		}
	}

	payload, err := xml.Marshal(SOAPFault{
		Code: SOAPFaultCode{
			Value: string(errorCodes[message.Error(env.CallError.ErrorCode)]),
			Subcode: &SOAPFaultSubCode{
				XMLNS: env.SOAPURN,
				Value: env.CallError.ErrorCode,
			},
		},
		Reason: env.CallError.ErrorDescription,
	})
	if err != nil {
		return nil, err
	}

	return xml.Marshal(SOAPEnvelope{
		Header: header,
		Body: SOAPBody{
			Payload: payload,
		},
	})
}
