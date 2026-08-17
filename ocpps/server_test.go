package ocpps_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/road-labs/ocpps-go/ocpps"
	"github.com/road-labs/ocpps-go/ocpps/message"
	"github.com/road-labs/ocpps-go/ocpps/serveropt"
)

const (
	testServerPort      = 9823
	testServerAdvertise = "http://central-system.example.com/ocpp"
)

func TestNewServer_ValidatesInputs(t *testing.T) {
	hooks := &captureHooks{}
	tests := map[string]struct {
		listenAddr    string
		soapURN       string
		advertiseAddr string
	}{
		"missing listenAddr":    {"", testSOAPURN, testServerAdvertise},
		"missing soapURN":       {":8080", "", testServerAdvertise},
		"missing advertiseAddr": {":8080", testSOAPURN, ""},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := ocpps.NewServer(tc.listenAddr, tc.soapURN, tc.advertiseAddr, hooks)
			assert.Error(t, err)
		})
	}
}

func TestServer_HandleRequest(t *testing.T) {
	t.Run("invokes OnCallRead and writes the hook's CallResult", func(t *testing.T) {
		hooks := &captureHooks{
			respond: func(_ *ocpps.ClientInfo, call message.Call, w ocpps.ResponseWriter) error {
				return w.WriteCallResult(context.Background(), message.CallResult{
					MessageID: call.MessageID,
					Payload:   []byte(`<bootNotificationResponse><status>Accepted</status></bootNotificationResponse>`),
				})
			},
		}
		baseURL := startTestServer(t, hooks)

		resp := doPost(t, baseURL, bootNotificationRequestEnvelope("message-1"))
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.NotNil(t, hooks.gotClientInfo)
		assert.Equal(t, testOCPPIdentity, hooks.gotClientInfo.OCPPIdentity)
		assert.Equal(t, testClientFrom, hooks.gotClientInfo.FromURL)
		assert.Equal(t, "message-1", hooks.gotCall.MessageID)
		assert.Equal(t, "BootNotification", hooks.gotCall.Action)

		body := readBody(t, resp)
		assert.Contains(t, body, "BootNotificationResponse")
		assert.Contains(t, body, "message-1") // reply carries a RelatesTo header
	})

	t.Run("rejects non-POST requests with a ProtocolError SOAP fault", func(t *testing.T) {
		hooks := &captureHooks{}
		baseURL := startTestServer(t, hooks)

		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, baseURL, nil)
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// ProtocolError is a Sender-class fault: HTTP 400 per SOAP 1.2 §7.1.1.
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		body := readBody(t, resp)
		assert.Contains(t, body, string(message.ProtocolError))
		assert.False(t, hooks.called, "OnCallRead should not be invoked for non-POST requests")
	})

	t.Run("returns a ProtocolError SOAP fault for a malformed body", func(t *testing.T) {
		hooks := &captureHooks{}
		baseURL := startTestServer(t, hooks)

		resp := doPost(t, baseURL, "not xml")
		defer resp.Body.Close()

		// ProtocolError is a Sender-class fault: HTTP 400 per SOAP 1.2 §7.1.1.
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		body := readBody(t, resp)
		assert.Contains(t, body, string(message.ProtocolError))
		assert.False(t, hooks.called, "OnCallRead should not be invoked when the envelope cannot be parsed")
	})

	t.Run("writes an InternalError SOAP fault when the hook returns an error without replying", func(t *testing.T) {
		hooks := &captureHooks{
			respond: func(_ *ocpps.ClientInfo, _ message.Call, _ ocpps.ResponseWriter) error {
				return assert.AnError
			},
		}
		baseURL := startTestServer(t, hooks)

		resp := doPost(t, baseURL, bootNotificationRequestEnvelope("message-1"))
		defer resp.Body.Close()

		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
		body := readBody(t, resp)
		assert.Contains(t, body, string(message.InternalError))
		assert.Contains(t, body, "message-1") // the fault carries a RelatesTo pointing at the failing call
	})

	t.Run("exposes a health endpoint at /health by default", func(t *testing.T) {
		hooks := &captureHooks{}
		baseURL := startTestServer(t, hooks)

		resp, err := http.Get(baseURL + "/health")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "OK", readBody(t, resp))
		assert.False(t, hooks.called, "OnCallRead should not be invoked for a health probe")
	})

	t.Run("exposes the health endpoint at the configured path when WithHealthEndpoint is set", func(t *testing.T) {
		hooks := &captureHooks{}
		baseURL := startTestServer(t, hooks, serveropt.WithHealthEndpoint("/live"))

		liveResp, err := http.Get(baseURL + "/live")
		require.NoError(t, err)
		defer liveResp.Body.Close()
		assert.Equal(t, http.StatusOK, liveResp.StatusCode)
		assert.Equal(t, "OK", readBody(t, liveResp))

		// The default /health path is no longer registered, so it falls through to the root handler and is treated
		// as an OCPP-S request. A GET is rejected as an invalid method.
		defaultResp, err := http.Get(baseURL + "/health")
		require.NoError(t, err)
		defer defaultResp.Body.Close()
		assert.Equal(t, http.StatusBadRequest, defaultResp.StatusCode)
		assert.Contains(t, readBody(t, defaultResp), string(message.ProtocolError))
	})

	t.Run("routes only the configured path when WithPath is set", func(t *testing.T) {
		hooks := &captureHooks{
			respond: func(_ *ocpps.ClientInfo, call message.Call, w ocpps.ResponseWriter) error {
				return w.WriteCallResult(context.Background(), message.CallResult{
					MessageID: call.MessageID,
					Payload:   []byte(`<bootNotificationResponse/>`),
				})
			},
		}
		baseURL := startTestServer(t, hooks, serveropt.WithPath("/ocpp"))

		// Matching path routes to the hook.
		resp := doPost(t, baseURL+"/ocpp", bootNotificationRequestEnvelope("message-1"))
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)
		assert.True(t, hooks.called)

		// A non-matching path is rejected by the mux with a 404, and the hook is not invoked.
		hooks.called = false
		otherResp := doPost(t, baseURL+"/somewhere-else", bootNotificationRequestEnvelope("message-2"))
		defer otherResp.Body.Close()

		assert.Equal(t, http.StatusNotFound, otherResp.StatusCode)
		assert.False(t, hooks.called)
	})

	t.Run("writes a CallError SOAP fault when the hook replies with one", func(t *testing.T) {
		hooks := &captureHooks{
			respond: func(_ *ocpps.ClientInfo, call message.Call, w ocpps.ResponseWriter) error {
				return w.WriteCallError(context.Background(), message.CallError{
					MessageID:        call.MessageID,
					ErrorCode:        string(message.NotSupported),
					ErrorDescription: "action not implemented",
				})
			},
		}
		baseURL := startTestServer(t, hooks)

		resp := doPost(t, baseURL, bootNotificationRequestEnvelope("message-1"))
		defer resp.Body.Close()

		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
		body := readBody(t, resp)
		assert.Contains(t, body, string(message.NotSupported))
		assert.Contains(t, body, "action not implemented")
	})
}

func startTestServer(t *testing.T, hooks ocpps.ServerHooks, opts ...ocpps.ServerOption) string {
	t.Helper()

	server, err := ocpps.NewServer(
		fmt.Sprintf(":%d", testServerPort),
		testSOAPURN,
		testServerAdvertise,
		hooks,
		opts...,
	)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = server.Start(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	addr := fmt.Sprintf("127.0.0.1:%d", testServerPort)
	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	}, 2*time.Second, 10*time.Millisecond, "test server did not start listening in time")

	return "http://" + addr
}

type captureHooks struct {
	respond       func(*ocpps.ClientInfo, message.Call, ocpps.ResponseWriter) error
	called        bool
	gotClientInfo *ocpps.ClientInfo
	gotCall       message.Call
}

func (h *captureHooks) OnCallRead(_ context.Context, info *ocpps.ClientInfo, call message.Call, w ocpps.ResponseWriter) error {
	h.called = true
	h.gotClientInfo = info
	h.gotCall = call
	if h.respond == nil {
		return nil
	}
	return h.respond(info, call, w)
}

func doPost(t *testing.T, url, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/soap+xml")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(body)
}

func bootNotificationRequestEnvelope(messageID string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope"
               xmlns:wsa="http://www.w3.org/2005/08/addressing"
               xmlns:cs="urn://Ocpp/Cs/2015/10/">
  <soap:Header>
    <cs:chargeBoxIdentity>%s</cs:chargeBoxIdentity>
    <wsa:Action soap:mustUnderstand="true">/BootNotification</wsa:Action>
    <wsa:MessageID>%s</wsa:MessageID>
    <wsa:From><wsa:Address>%s</wsa:Address></wsa:From>
    <wsa:ReplyTo><wsa:Address>http://www.w3.org/2005/08/addressing/anonymous</wsa:Address></wsa:ReplyTo>
    <wsa:To soap:mustUnderstand="true">%s</wsa:To>
  </soap:Header>
  <soap:Body>
    <cs:bootNotificationRequest>
      <cs:chargePointVendor>Vendor</cs:chargePointVendor>
      <cs:chargePointModel>Model</cs:chargePointModel>
    </cs:bootNotificationRequest>
  </soap:Body>
</soap:Envelope>`, testOCPPIdentity, messageID, testClientFrom, testServerAdvertise)
}
