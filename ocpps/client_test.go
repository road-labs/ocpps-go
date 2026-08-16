package ocpps_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
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
			_, err := ocpps.NewClient(tc.serverURL, tc.soapURN, tc.ocppIdentity, tc.from)
			assert.Error(t, err)
		})
	}
}

func TestClient_WriteCall(t *testing.T) {
	t.Run("sends a SOAP envelope and returns the parsed CallResult", func(t *testing.T) {
		var receivedBody []byte
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedBody, _ = io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/soap+xml")
			_, _ = w.Write([]byte(bootNotificationResponseEnvelope("message-1", "reply-1")))
		}))
		defer srv.Close()

		client, err := ocpps.NewClient(srv.URL, testSOAPURN, testOCPPIdentity, testClientFrom)
		require.NoError(t, err)

		res, err := client.WriteCall(context.Background(), message.Call{
			MessageID: "message-1",
			Action:    "BootNotification",
			Payload:   []byte(`<bootNotificationRequest/>`),
		})
		require.NoError(t, err)

		// The reply's MessageID field is populated from the WS-Addressing RelatesTo header on the response.
		assert.Equal(t, "message-1", res.MessageID)
		assert.Contains(t, string(res.Payload), "bootNotificationResponse")

		// The outbound envelope should carry the OCPP-S SOAP headers.
		body := string(receivedBody)
		assert.Contains(t, body, "<Action")
		assert.Contains(t, body, "/BootNotification")
		assert.Contains(t, body, "message-1")
		assert.Contains(t, body, testOCPPIdentity)
		assert.Contains(t, body, testClientFrom)
	})

	t.Run("returns an error when the peer replies with a non-200 status", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "nope", http.StatusInternalServerError)
		}))
		defer srv.Close()

		client, err := ocpps.NewClient(srv.URL, testSOAPURN, testOCPPIdentity, testClientFrom)
		require.NoError(t, err)

		_, err = client.WriteCall(context.Background(), message.Call{
			MessageID: "message-1",
			Action:    "BootNotification",
			Payload:   []byte(`<bootNotificationRequest/>`),
		})
		assert.ErrorContains(t, err, "unexpected status code 500")
	})

	t.Run("returns an error when the reply action does not match", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/soap+xml")
			// Response advertises the wrong action - HeartbeatResponse instead of BootNotificationResponse
			_, _ = w.Write([]byte(bootNotificationResponseEnvelopeWithAction("message-1", "reply-1", "/HeartbeatResponse")))
		}))
		defer srv.Close()

		client, err := ocpps.NewClient(srv.URL, testSOAPURN, testOCPPIdentity, testClientFrom)
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

		client, err := ocpps.NewClient(srv.URL, testSOAPURN, testOCPPIdentity, testClientFrom)
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

		client, err := ocpps.NewClient(srv.URL, testSOAPURN, testOCPPIdentity, testClientFrom, clientopt.WithTimeout(50*time.Millisecond))
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

		client, err := ocpps.NewClient(srv.URL, testSOAPURN, testOCPPIdentity, testClientFrom)
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
