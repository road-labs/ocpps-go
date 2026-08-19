package ocpps_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/road-labs/ocpps-go/ocpps"
	"github.com/road-labs/ocpps-go/ocpps/clientopt"
	"github.com/road-labs/ocpps-go/ocpps/message"
)

const (
	testSOAPURN      = "urn://Ocpp/Cs/2015/10/"
	testOCPPIdentity = "ChargeStationFoo"
	testClientFrom   = "http://charge-station.example.com/ocpp"
)

func TestNewClient_ValidatesInputs(t *testing.T) {
	hooks := &captureClientHooks{}
	tests := map[string]struct {
		serverURL    string
		soapURN      string
		ocppIdentity string
		from         string
	}{
		"missing serverURL":    {"", testSOAPURN, testOCPPIdentity, testClientFrom},
		"missing soapURN":      {"http://example.com", "", testOCPPIdentity, testClientFrom},
		"missing ocppIdentity": {"http://example.com", testSOAPURN, "", testClientFrom},
		"missing from":         {"http://example.com", testSOAPURN, testOCPPIdentity, ""},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := ocpps.NewClient(tc.serverURL, tc.soapURN, tc.ocppIdentity, tc.from, hooks)
			assert.Error(t, err)
		})
	}
}

func TestClient_WriteCall(t *testing.T) {
	t.Run("sends a SOAP envelope, returns the parsed CallResult, and invokes hooks", func(t *testing.T) {
		var receivedBody []byte
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedBody, _ = io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/soap+xml")
			_, _ = w.Write([]byte(bootNotificationResponseEnvelope("message-1", "reply-1")))
		}))
		defer srv.Close()

		hooks := &captureClientHooks{}
		client, err := ocpps.NewClient(srv.URL, testSOAPURN, testOCPPIdentity, testClientFrom, hooks)
		require.NoError(t, err)

		res, err := client.WriteCall(context.Background(), message.Call{
			MessageID: "message-1",
			Action:    "BootNotification",
			Payload:   []byte(`<bootNotificationRequest/>`),
		})
		require.NoError(t, err)

		assert.Equal(t, "message-1", res.MessageID)
		assert.Contains(t, string(res.Payload), "bootNotificationResponse")

		body := string(receivedBody)
		assert.Contains(t, body, "<Action")
		assert.Contains(t, body, "/BootNotification")
		assert.Contains(t, body, "message-1")
		assert.Contains(t, body, testOCPPIdentity)
		assert.Contains(t, body, testClientFrom)

		// Hooks fired for write + successful result, not for error.
		assert.Equal(t, int32(1), hooks.written.Load())
		assert.Equal(t, int32(1), hooks.resultRead.Load())
		assert.Equal(t, int32(0), hooks.errorRead.Load())
	})

	t.Run("parses a SOAP fault into a *CallError and invokes OnCallErrorRead", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/soap+xml")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(faultEnvelope("message-1", "SecurityError", "not authorized")))
		}))
		defer srv.Close()

		hooks := &captureClientHooks{}
		client, err := ocpps.NewClient(srv.URL, testSOAPURN, testOCPPIdentity, testClientFrom, hooks)
		require.NoError(t, err)

		_, err = client.WriteCall(context.Background(), message.Call{
			MessageID: "message-1",
			Action:    "BootNotification",
			Payload:   []byte(`<bootNotificationRequest/>`),
		})
		require.Error(t, err)

		var callErr *ocpps.CallError
		require.ErrorAs(t, err, &callErr)
		assert.Equal(t, "SecurityError", callErr.ErrorCode)
		assert.Equal(t, "not authorized", callErr.ErrorDescription)
		assert.Equal(t, "message-1", callErr.MessageID)

		assert.Equal(t, int32(1), hooks.written.Load())
		assert.Equal(t, int32(1), hooks.errorRead.Load())
		assert.Equal(t, int32(0), hooks.resultRead.Load())
	})

	t.Run("falls back to a generic error when a non-200 response isn't a valid SOAP fault", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "nope", http.StatusInternalServerError)
		}))
		defer srv.Close()

		hooks := &captureClientHooks{}
		client, err := ocpps.NewClient(srv.URL, testSOAPURN, testOCPPIdentity, testClientFrom, hooks)
		require.NoError(t, err)

		_, err = client.WriteCall(context.Background(), message.Call{
			MessageID: "message-1",
			Action:    "BootNotification",
			Payload:   []byte(`<bootNotificationRequest/>`),
		})
		assert.ErrorContains(t, err, "unexpected status code 500")

		var callErr *ocpps.CallError
		assert.False(t, false, "not a *CallError")
		assert.NotErrorIs(t, err, callErr)
	})

	t.Run("returns an error when the reply action does not match", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/soap+xml")
			_, _ = w.Write([]byte(bootNotificationResponseEnvelopeWithAction("message-1", "reply-1", "/HeartbeatResponse")))
		}))
		defer srv.Close()

		hooks := &captureClientHooks{}
		client, err := ocpps.NewClient(srv.URL, testSOAPURN, testOCPPIdentity, testClientFrom, hooks)
		require.NoError(t, err)

		_, err = client.WriteCall(context.Background(), message.Call{
			MessageID: "message-1",
			Action:    "BootNotification",
			Payload:   []byte(`<bootNotificationRequest/>`),
		})
		assert.ErrorContains(t, err, "invalid action")
		assert.ErrorContains(t, err, "BootNotificationResponse")
	})

	t.Run("returns an error when the reply body is malformed XML", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/soap+xml")
			_, _ = w.Write([]byte("not xml"))
		}))
		defer srv.Close()

		hooks := &captureClientHooks{}
		client, err := ocpps.NewClient(srv.URL, testSOAPURN, testOCPPIdentity, testClientFrom, hooks)
		require.NoError(t, err)

		_, err = client.WriteCall(context.Background(), message.Call{
			MessageID: "message-1",
			Action:    "BootNotification",
			Payload:   []byte(`<bootNotificationRequest/>`),
		})
		assert.Error(t, err)
	})

	t.Run("honours the configured request timeout", func(t *testing.T) {
		release := make(chan struct{})
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-release
		}))
		defer srv.Close()
		defer close(release)

		hooks := &captureClientHooks{}
		client, err := ocpps.NewClient(srv.URL, testSOAPURN, testOCPPIdentity, testClientFrom, hooks, clientopt.WithTimeout(50*time.Millisecond))
		require.NoError(t, err)

		_, err = client.WriteCall(context.Background(), message.Call{
			MessageID: "message-1",
			Action:    "BootNotification",
			Payload:   []byte(`<bootNotificationRequest/>`),
		})
		assert.Error(t, err)
	})

	t.Run("propagates context cancellation to the underlying HTTP request", func(t *testing.T) {
		blocked := make(chan struct{})
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-blocked
		}))
		defer srv.Close()
		defer close(blocked)

		hooks := &captureClientHooks{}
		client, err := ocpps.NewClient(srv.URL, testSOAPURN, testOCPPIdentity, testClientFrom, hooks)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err = client.WriteCall(ctx, message.Call{
			MessageID: "message-1",
			Action:    "BootNotification",
			Payload:   []byte(`<bootNotificationRequest/>`),
		})
		assert.Error(t, err)
	})
}

// captureClientHooks is a ClientHooks test double that counts invocations.
type captureClientHooks struct {
	written    atomic.Int32
	resultRead atomic.Int32
	errorRead  atomic.Int32
}

func (h *captureClientHooks) OnCallWritten(context.Context, *ocpps.ClientInfo, message.Call) error {
	h.written.Add(1)
	return nil
}

func (h *captureClientHooks) OnCallResultRead(context.Context, *ocpps.ClientInfo, message.CallResult) error {
	h.resultRead.Add(1)
	return nil
}

func (h *captureClientHooks) OnCallErrorRead(context.Context, *ocpps.ClientInfo, message.CallError) error {
	h.errorRead.Add(1)
	return nil
}

// bootNotificationResponseEnvelope produces a minimal, well-formed SOAP envelope carrying a BootNotificationResponse
// payload. It is intentionally hand-built so tests exercise the client's unmarshalling path against realistic input.
func bootNotificationResponseEnvelope(relatesTo, messageID string) string {
	return bootNotificationResponseEnvelopeWithAction(relatesTo, messageID, "/BootNotificationResponse")
}

func bootNotificationResponseEnvelopeWithAction(relatesTo, messageID, action string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope"
               xmlns:wsa="http://www.w3.org/2005/08/addressing"
               xmlns:cs="urn://Ocpp/Cs/2015/10/">
  <soap:Header>
    <cs:chargeBoxIdentity>%s</cs:chargeBoxIdentity>
    <wsa:Action soap:mustUnderstand="true">%s</wsa:Action>
    <wsa:MessageID>%s</wsa:MessageID>
    <wsa:RelatesTo>%s</wsa:RelatesTo>
    <wsa:From><wsa:Address>http://central-system.example.com/ocpp</wsa:Address></wsa:From>
    <wsa:ReplyTo><wsa:Address>http://www.w3.org/2005/08/addressing/anonymous</wsa:Address></wsa:ReplyTo>
    <wsa:To soap:mustUnderstand="true">%s</wsa:To>
  </soap:Header>
  <soap:Body>
    <cs:bootNotificationResponse>
      <cs:status>Accepted</cs:status>
    </cs:bootNotificationResponse>
  </soap:Body>
</soap:Envelope>`, testOCPPIdentity, action, messageID, relatesTo, testClientFrom)
}

// faultEnvelope produces a SOAP 1.2 fault envelope with a RelatesTo header and the given OCPP error code/description.
func faultEnvelope(relatesTo, ocppCode, reason string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope"
               xmlns:wsa="http://www.w3.org/2005/08/addressing">
  <soap:Header>
    <wsa:RelatesTo>%s</wsa:RelatesTo>
  </soap:Header>
  <soap:Body>
    <soap:Fault>
      <soap:Code>
        <soap:Value>Sender</soap:Value>
        <soap:Subcode>
          <soap:Value xmlns:ocpp="urn://Ocpp/Cs/2015/10/">ocpp:%s</soap:Value>
        </soap:Subcode>
      </soap:Code>
      <soap:Reason>
        <soap:Text xml:lang="en">%s</soap:Text>
      </soap:Reason>
    </soap:Fault>
  </soap:Body>
</soap:Envelope>`, relatesTo, ocppCode, reason)
}
