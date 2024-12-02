# HLS WebSocket CDN - Publisher client library

This is a client library to publish HLS to **HLS WebSocket CDN**, implemented in golang.

[Documentation](https://pkg.go.dev/github.com/AgustinSRG/hls-websocket-cdn/client-publisher)

## Installation

To install the library into your project, run:

```sh
go get github.com/AgustinSRG/hls-websocket-cdn/client-publisher
```

## Usage

For each HLS stream you want to publish, create and instance of `HlsWebSocketPublisher` by calling the `NewHlsWebSocketPublisher` function.

You can then call the `PublishFragment(duration, data)` to publish each HLS fragment.

When the stream finishes. You must call the `Close()` method.

```go
package main

import (
    "fmt"
	"time"

    // Import the module
    clientpublisher "github.com/AgustinSRG/hls-websocket-cdn/client-publisher"
)

func main() {
    // Create publisher instance
	publisher := clientpublisher.NewHlsWebSocketPublisher(clientpublisher.HlsWebSocketPublisherConfiguration{
		// URL of the CDN server
		ServerUrl: "ws://127.0.0.1/",
		// ID of the stream to publish
		StreamId: "test",
		// Secret to sign the authentication tokens
		AuthSecret: "secret",
		OnReady: func() {
			fmt.Println("Publisher is ready.")
		},
		OnError: func(url string, msg string) {
			fmt.Printf("Could not connect to %v: %v", url, msg)
		},
	})

	// After creating the publisher,
	// send the HLS fragments as soon as they are available
	for i := 0; i < 10; i++ {
		publisher.SendFragment(2, []byte{0x00})
		time.Sleep(3 * time.Second)
	}

	// When the last fragment is send, call Close() to finish
	publisher.Close()
}

```

## Compilation

In order to install dependencies, type:

```sh
go get .
```

To compile the code type:

```sh
go build .
```

# Tests

To test the code, type:

```sh
go test -v
```
