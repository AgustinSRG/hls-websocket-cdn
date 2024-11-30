// Publisher client configuration

package clientpublisher

import "time"

// Configuration for the publisher client
type HlsWebSocketPublisherConfiguration struct {
	// Server URL
	ServerUrl string

	// Function to get the server URL
	// If set, ServerUrl is ignored
	GetServerUrl func() string

	// ID of the stream to publish
	StreamId string

	// Secret to generate authentication tokens
	AuthSecret string

	// Max length of the queue to keep fragments
	// if they cannot be sent to the server immediately
	// (10 by default)
	QueueMaxLength int

	// Function to be called when the publisher is ready
	OnReady func()

	// Function called on connection or authentication error
	// Receives the server URL and the error message
	OnError func(url string, msg string)

	// Delay to retry the connection after an error
	// Default: 1 second
	ConnectionRetryDelay time.Duration
}
