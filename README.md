# ocpps-go

Go implementation of the OCPP-S "SOAP over HTTP" specification.

## Features

This package handles a base level OCPP-S implementation, including:

- Client and server communication over HTTP, using either plaintext or TLS
- Serialization and parsing of the OCPP-S SOAP envelopes (WS-Addressing headers, `chargeBoxIdentity` header, SOAP faults
  for call errors, etc), with basic validation
- Request/response correlation via the WS-Addressing `MessageID` and `RelatesTo` headers

This package is a sibling to [ocppj-go](https://github.com/road-labs/ocppj-go); where ocppj-go covers the
JSON-over-WebSockets binding of OCPP, this package covers the SOAP-over-HTTP binding used by OCPP 1.5 and OCPP 1.6 SOAP
profiles. As with ocppj-go, the code is not tied to any specific version of the OCPP protocol - it can be used with both
1.5 and 1.6.

## Usage

This package covers both client and server implementations of the OCPP-S protocol. In OCPP-S, both the charging station
and the central system act as SOAP endpoints - either side can initiate a call by POSTing to the other's endpoint. The
client implementation can be used to send calls to a peer, and the server implementation provides the plumbing to
receive them.

Both client and server implementations require registration of hooks. For server usage, OnCallRead must be used to
respond to requests. All remaining server hooks, and all client hooks are not used for wiring in required behaviour;
instead they offer a convenient way of inspect or logging inbound and outbound payloads. Where such requirements are not
needed, simply implement stub functions.

### Client

```go
package main

import (
  "context"
  "encoding/xml"
  "errors"
  "fmt"
  "log"
  "time"

  "github.com/google/uuid"
  "github.com/road-labs/ocpp-types-go/gen/ocpp16"
  "github.com/road-labs/ocpps-go/ocpps"
  "github.com/road-labs/ocpps-go/ocpps/clientopt"
  "github.com/road-labs/ocpps-go/ocpps/message"
)

func main() {
  ctx := context.Background()

  client, err := ocpps.NewClient(
    "http://central-system.example.com/ocpp",
    "urn://Ocpp/Cs/2015/10/", // If used from the central system side "urn://Ocpp/Cp/2012/06/"
    "ChargeStationFoo",
    "http://charge-station.example.com/ocpp",
    &clientHooks{},
    clientopt.WithTimeout(15*time.Second),
  )
  if err != nil {
    log.Fatal(err)
  }

  reqPayload, err := xml.Marshal(ocpp16.BootNotification{
    ChargePointVendor: "vendor",
    ChargePointModel:  "model",
  })
  if err != nil {
    log.Fatal(err)
  }

  res, err := client.WriteCall(ctx, message.Call{
    MessageID: uuid.NewString(),
    Action:    "BootNotification",
    Payload:   reqPayload,
  })
  if err != nil {
    var callErr *ocpps.CallError
    if errors.As(err, &callErr) {
      log.Fatalf("call error: %s (%s)", callErr.ErrorCode, callErr.ErrorDescription)
    }
    log.Fatal(err)
  }

  var resPayload ocpp16.BootNotificationResponse
  if err = xml.Unmarshal(res.Payload, &resPayload); err != nil {
    log.Fatal(err)
  }

  log.Printf("received result: %+v", resPayload)
}

// clientHooks implements ocpps.ClientHooks.
type clientHooks struct{}

func (c *clientHooks) OnCallWritten(context.Context, *ocpps.ClientInfo, message.Call) error {
  return nil
}
func (c *clientHooks) OnCallResultRead(context.Context, *ocpps.ClientInfo, message.CallResult) error {
  return nil
}
func (c *clientHooks) OnCallErrorRead(context.Context, *ocpps.ClientInfo, message.CallError) error {
  return nil
}
```

### Server

```go
package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/road-labs/ocpps-go/ocpps"
	"github.com/road-labs/ocpps-go/ocpps/message"
	"github.com/road-labs/ocpps-go/ocpps/serveropt"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	server, err := ocpps.NewServer(
		":8080",
		"urn://Ocpp/Cs/2015/10/", // If used on the charge point side "urn://Ocpp/Cp/2012/06/"
		"http://central-system.example.com/ocpp",
		&serverHooks{},
		serveropt.WithPath("/ocpp"),
	)
	if err != nil {
		log.Fatal(err)
	}

	_ = server.Start(ctx)
}

// serverHooks implements ocpps.ServerHooks.
type serverHooks struct{}

func (s *serverHooks) OnCallRead(ctx context.Context, info *ocpps.ClientInfo, call message.Call, w ocpps.ResponseWriter) error {
	// Reply via w.WriteCallResult or w.WriteCallError.
	return nil
}

func (s *serverHooks) OnCallResultWritten(context.Context, *ocpps.ClientInfo, message.CallResult) error {
	return nil
}

func (s *serverHooks) OnCallErrorWritten(context.Context, *ocpps.ClientInfo, message.CallError) error {
	return nil
}
```

## Testing

```sh
go test ./...
```

## License

MIT. See [LICENSE](LICENSE).
