// HTTP server test tools

package main

import (
	"fmt"
	"net"
	"net/http"
)

// Runs a test server on a random port
// Returns the URL of the server + the listener to close it
func (server *HttpServer) RunTestServer() (string, net.Listener) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}

	go func() {
		_ = http.Serve(listener, server)
	}()

	port := listener.Addr().(*net.TCPAddr).Port
	url := "ws://127.0.0.1:" + fmt.Sprint(port) + "/"

	server.sourceController.config.ExternalWebsocketUrl = url

	server.logger.Debugf("Using port for test server: %v", port)

	return url, listener
}
