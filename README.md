# ocpps-go

Go implementation of the OCPP-S "SOAP over HTTP" specification.

## Features

This package handles a base level OCPP-S implementation, including:

- Client and server communication over HTTP, using either plaintext or TLS
- Serialization and parsing of the OCPP-S SOAP envelopes (WS-Addressing headers,
  `chargeBoxIdentity` header, SOAP faults for call errors), with basic validation
- Request/response correlation via the WS-Addressing `MessageID` and `RelatesTo`
  headers

This package is a sibling to [ocppj-go](https://github.com/road-labs/ocppj-go);
where ocppj-go covers the JSON-over-WebSockets binding of OCPP, this package
covers the SOAP-over-HTTP binding used by OCPP 1.5 and OCPP 1.6 SOAP profiles.

## Usage

This package covers both client and server implementations of the OCPP-S protocol. In OCPP-S, both the charging
station and the central system act as SOAP endpoints — either side can initiate a call by POSTing to the other's
endpoint. The client implementation can be used to send calls to a peer, and the server implementation provides
the plumbing to receive them.

Callers wire in server behaviour through hooks. `ocpps.ServerHooks` covers reads of incoming calls, with a
`ResponseWriter` supplied for replying with either a call result or a call error.

### Client

```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/road-labs/ocpps-go/ocpps"
	"github.com/road-labs/ocpps-go/ocpps/clientopt"
	"github.com/road-labs/ocpps-go/ocpps/message"
)

func main() {
	ctx := context.Background()

	client, err := ocpps.NewClient(
		"http://central-system.example.com/ocpp",
		"urn://Ocpp/Cp/2015/10/",
		"ChargeStationFoo",
		"http://charge-station.example.com/ocpp",
		clientopt.WithTimeout(15*time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}

	res, err := client.WriteCall(ctx, message.Call{
		MessageID: "message-1",
		Action:    "BootNotification",
		Payload:   []byte(`<bootNotificationRequest xmlns="urn://Ocpp/Cs/2015/10/"/>`),
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("received result: %s", res.Payload)
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
		"urn://Ocpp/Cs/2015/10/",
		"http://central-system.example.com/ocpp",
		&serverHooks{},
		serveropt.WithPath("/ocpp"),
	)
	if err != nil {
		log.Fatal(err)
	}

	_ = server.Start(ctx)
}

// serverHooks implements ocpps.ServerHooks, receiving calls read from each connected client.
type serverHooks struct{}

func (s *serverHooks) OnCallRead(ctx context.Context, info *ocpps.ClientInfo, call message.Call, w ocpps.ResponseWriter) error {
	// Reply via w.WriteCallResult or w.WriteCallError.
	return nil
}
```

## Testing

```sh
go test ./...
```

## License

MIT. See [LICENSE](LICENSE).
