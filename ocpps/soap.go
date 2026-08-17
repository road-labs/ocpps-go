package ocpps

import (
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/road-labs/ocpps-go/ocpps/message"
)

const wsaAnonymousURI = "http://www.w3.org/2005/08/addressing/anonymous"

type soapEnvelope struct {
	XMLName xml.Name `xml:"http://www.w3.org/2003/05/soap-envelope Envelope"`

	Header *soapHeader
	Body   soapBody
}

type soapHeader struct {
	XMLName                 xml.Name `xml:"http://www.w3.org/2003/05/soap-envelope Header"`
	ChargeBoxIdentityHeader chargeBoxIdentityHeader
	ActionHeader            actionHeader
	MessageIDHeader         messageIDHeader
	FromHeader              fromHeader
	ReplyToHeader           *replyToHeader
	ToHeader                toHeader
	RelatesTo               *relatesTo
}

type chargeBoxIdentityHeader struct {
	XMLName        xml.Name `xml:"chargeBoxIdentity"`
	XMLNS          string   `xml:"xmlns,attr"`
	MustUnderstand bool     `xml:"http://www.w3.org/2003/05/soap-envelope mustUnderstand,attr"`
	Identity       string   `xml:",chardata"`
}

type actionHeader struct {
	XMLName        xml.Name `xml:"http://www.w3.org/2005/08/addressing Action"`
	MustUnderstand bool     `xml:"http://www.w3.org/2003/05/soap-envelope mustUnderstand,attr"`
	Action         string   `xml:",chardata"`
}

type messageIDHeader struct {
	XMLName   xml.Name `xml:"http://www.w3.org/2005/08/addressing MessageID"`
	MessageID string   `xml:",chardata"`
}

type fromHeader struct {
	XMLName xml.Name `xml:"http://www.w3.org/2005/08/addressing From"`
	From    address
}

type replyToHeader struct {
	XMLName xml.Name `xml:"http://www.w3.org/2005/08/addressing ReplyTo"`
	ReplyTo address
}

type toHeader struct {
	XMLName xml.Name `xml:"http://www.w3.org/2005/08/addressing To"`
	To      string   `xml:",chardata"`
}

type relatesTo struct {
	XMLName   xml.Name `xml:"http://www.w3.org/2005/08/addressing RelatesTo"`
	RelatesTo string   `xml:",chardata"`
}

type address struct {
	XMLName xml.Name `xml:"http://www.w3.org/2005/08/addressing Address"`
	Address string   `xml:",chardata"`
}

type soapBody struct {
	XMLName xml.Name `xml:"http://www.w3.org/2003/05/soap-envelope Body"`
	Payload []byte   `xml:",innerxml"`
}

type soapFault struct {
	XMLName xml.Name `xml:"http://www.w3.org/2003/05/soap-envelope Fault"`
	Code    soapFaultCode
	Reason  soapFaultReason
	Node    string `xml:"Node,omitempty"`
	Role    string `xml:"Role,omitempty"`
	Detail  any    `xml:"Detail,omitempty"`
}

type soapFaultReason struct {
	XMLName xml.Name `xml:"http://www.w3.org/2003/05/soap-envelope Reason"`
	Text    soapFaultReasonText
}

type soapFaultReasonText struct {
	XMLName xml.Name `xml:"http://www.w3.org/2003/05/soap-envelope Text"`
	Lang    string   `xml:"http://www.w3.org/XML/1998/namespace lang,attr"`
	Value   string   `xml:",chardata"`
}

type soapFaultCode struct {
	XMLName xml.Name `xml:"http://www.w3.org/2003/05/soap-envelope Code"`
	Value   string   `xml:"http://www.w3.org/2003/05/soap-envelope Value"`
	Subcode *soapFaultSubCode
}

type soapFaultSubCode struct {
	XMLName xml.Name `xml:"http://www.w3.org/2003/05/soap-envelope Subcode"`
	Value   soapFaultSubCodeValue
	Subcode *soapFaultSubCode
}

type soapFaultSubCodeValue struct {
	XMLName xml.Name `xml:"http://www.w3.org/2003/05/soap-envelope Value"`
	OcppNS  string   `xml:"xmlns:ocpp,attr,omitempty"`
	Content string   `xml:",chardata"`
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
	return xml.Marshal(soapEnvelope{
		Header: &soapHeader{
			ActionHeader: actionHeader{
				MustUnderstand: true,
				Action:         fmt.Sprintf("/%s", env.Call.Action),
			},
			MessageIDHeader: messageIDHeader{
				MessageID: env.Call.MessageID,
			},
			ReplyToHeader: &replyToHeader{
				ReplyTo: address{
					Address: wsaAnonymousURI,
				},
			},
			ChargeBoxIdentityHeader: chargeBoxIdentityHeader{
				XMLNS:          env.SOAPURN,
				MustUnderstand: true,
				Identity:       env.OCPPIdentity,
			},
			FromHeader: fromHeader{
				From: address{
					Address: env.From,
				},
			},
			ToHeader: toHeader{
				To: env.To,
			},
		},
		Body: soapBody{
			Payload: env.Call.Payload,
		},
	})
}

func unmarshalCall(b []byte) (callEnvelope, error) {
	var env soapEnvelope
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
	return xml.Marshal(soapEnvelope{
		Header: &soapHeader{
			ActionHeader: actionHeader{
				MustUnderstand: true,
				Action:         fmt.Sprintf("/%s", env.Action),
			},
			MessageIDHeader: messageIDHeader{
				MessageID: env.MessageID,
			},
			ChargeBoxIdentityHeader: chargeBoxIdentityHeader{
				XMLNS:          env.SOAPURN,
				MustUnderstand: true,
				Identity:       env.OCPPIdentity,
			},
			FromHeader: fromHeader{
				From: address{
					Address: env.From,
				},
			},
			ToHeader: toHeader{
				To: env.To,
			},
			RelatesTo: &relatesTo{
				RelatesTo: env.CallResult.MessageID,
			},
		},
		Body: soapBody{
			Payload: env.CallResult.Payload,
		},
	})
}

func unmarshalCallResult(b []byte) (callResultEnvelope, error) {
	var env soapEnvelope
	if err := xml.Unmarshal(b, &env); err != nil {
		return callResultEnvelope{}, err
	}

	var relatesToID string
	if env.Header.RelatesTo != nil {
		relatesToID = env.Header.RelatesTo.RelatesTo
	}

	if !strings.HasPrefix(env.Header.ActionHeader.Action, "/") {
		return callResultEnvelope{}, fmt.Errorf("action should have a leading slash, received %q", env.Header.ActionHeader.Action)
	}

	return callResultEnvelope{
		CallResult: message.CallResult{
			Payload:   env.Body.Payload,
			MessageID: relatesToID,
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

// faultCodeFor looks up the SOAP fault class for an OCPP error code. Unknown codes default to Receiver.
func faultCodeFor(errorCode string) faultCode {
	if code, ok := errorCodes[message.Error(errorCode)]; ok {
		return code
	}
	return faultCodeReceiver
}

func marshalCallError(env callErrorEnvelope) ([]byte, error) {
	header := &soapHeader{
		MessageIDHeader: messageIDHeader{
			MessageID: env.MessageID,
		},
		ChargeBoxIdentityHeader: chargeBoxIdentityHeader{
			XMLNS:          env.SOAPURN,
			MustUnderstand: true,
			Identity:       env.OCPPIdentity,
		},
		FromHeader: fromHeader{
			From: address{
				Address: env.From,
			},
		},
		ToHeader: toHeader{
			To: env.To,
		},
	}
	if env.Action != "" {
		header.ActionHeader = actionHeader{
			MustUnderstand: true,
			Action:         fmt.Sprintf("/%s", env.Action),
		}
	}
	if env.CallError.MessageID != "" {
		header.RelatesTo = &relatesTo{
			RelatesTo: env.CallError.MessageID,
		}
	}

	payload, err := xml.Marshal(soapFault{
		Code: soapFaultCode{
			Value: string(faultCodeFor(env.CallError.ErrorCode)),
			Subcode: &soapFaultSubCode{
				Value: soapFaultSubCodeValue{
					OcppNS:  env.SOAPURN,
					Content: fmt.Sprintf("ocpp:%s", env.CallError.ErrorCode),
				},
			},
		},
		Reason: soapFaultReason{
			Text: soapFaultReasonText{
				Lang:  "en",
				Value: env.CallError.ErrorDescription,
			},
		},
	})
	if err != nil {
		return nil, err
	}

	return xml.Marshal(soapEnvelope{
		Header: header,
		Body: soapBody{
			Payload: payload,
		},
	})
}
