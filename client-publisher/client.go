// Publisher client

package clientpublisher

import (
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const heartbeat_msg_period_seconds = 30
const text_msg_read_limit = 1600

// HLS WebSocket publisher client
type HlsWebSocketPublisher struct {
	// Mutex for the struct
	mu *sync.Mutex

	// Configuration
	Config HlsWebSocketPublisherConfiguration

	// True if closed
	closed bool

	// True if ready
	ready bool

	// Socket
	socket *websocket.Conn

	// Queue of pending fragments
	pendingQueue []cdnPublisherPendingFragment

	// Channel to interrupt the heartbeat process
	heartbeatInterruptChannel chan bool
}

// Creates a new instance of HlsWebSocketPublisher
// Receives the configuration as the only parameter
func NewHlsWebSocketPublisher(config HlsWebSocketPublisherConfiguration) *HlsWebSocketPublisher {
	publisher := &HlsWebSocketPublisher{
		mu:                        &sync.Mutex{},
		Config:                    config,
		closed:                    false,
		ready:                     false,
		socket:                    nil,
		pendingQueue:              make([]cdnPublisherPendingFragment, 0),
		heartbeatInterruptChannel: make(chan bool, 1),
	}

	go publisher.run()
	go publisher.sendHeartbeatMessages()

	return publisher
}

// Checks if the publisher is closed
func (publisher *HlsWebSocketPublisher) IsClosed() bool {
	publisher.mu.Lock()
	defer publisher.mu.Unlock()

	return publisher.closed
}

// Gets the URL to connect to the server
func (publisher *HlsWebSocketPublisher) getServerUrl() string {
	if publisher.Config.GetServerUrl != nil {
		return publisher.Config.GetServerUrl()
	} else {
		return publisher.Config.ServerUrl
	}
}

// Waits after an error
func (publisher *HlsWebSocketPublisher) waitAfterError() {
	delay := publisher.Config.ConnectionRetryDelay

	if delay == 0 {
		delay = 1 * time.Second
	}

	time.Sleep(delay)
}

// Runs publisher thread
func (publisher *HlsWebSocketPublisher) run() {
	for !publisher.IsClosed() {
		url := publisher.getServerUrl()

		socket, _, err := websocket.DefaultDialer.Dial(url, nil)

		if err != nil {
			if publisher.Config.OnError != nil {
				publisher.Config.OnError(url, err.Error())
			}

			publisher.waitAfterError()
			continue
		}

		if publisher.IsClosed() {
			socket.Close()
			return
		}

		// Connected, send authentication

		cdnStreamId := publisher.Config.StreamId
		authToken, err := signAuthToken(publisher.Config.AuthSecret, "PUSH", cdnStreamId)

		if err != nil {
			socket.Close()

			if publisher.Config.OnError != nil {
				publisher.Config.OnError(url, err.Error())
			}

			publisher.waitAfterError()
			continue
		}

		authMessage := WebsocketProtocolMessage{
			MessageType: "PUSH",
			Parameters: map[string]string{
				"stream": cdnStreamId,
				"auth":   authToken,
			},
		}

		socket.WriteMessage(websocket.TextMessage, []byte(authMessage.Serialize()))

		// Connected

		publisher.onConnected(socket)

		var closedWithError = false

		// Read incoming messages

		for !publisher.IsClosed() {
			err := socket.SetReadDeadline(time.Now().Add(heartbeat_msg_period_seconds * 2 * time.Second))

			if err != nil {
				if !publisher.IsClosed() {
					if publisher.Config.OnError != nil {
						publisher.Config.OnError(url, err.Error())
					}

					closedWithError = true
				}
				break // Closed
			}

			socket.SetReadLimit(text_msg_read_limit)

			mt, message, err := socket.ReadMessage()

			if err != nil {
				if !publisher.IsClosed() {
					if publisher.Config.OnError != nil {
						publisher.Config.OnError(url, err.Error())
					}

					closedWithError = true
				}
				break // Closed
			}

			if mt != websocket.TextMessage {
				continue
			}

			parsedMessage := ParseWebsocketProtocolMessage(string(message))

			switch parsedMessage.MessageType {
			case "E":
				if publisher.Config.OnError != nil {
					publisher.Config.OnError(url, "Error from CDN. Code: "+parsedMessage.GetParameter("code")+", Message: "+parsedMessage.GetParameter("message"))
				}
				closedWithError = true
			case "OK":
				// Ready
				publisher.onReady()
			}
		}

		publisher.onDisconnected()

		if closedWithError {
			publisher.waitAfterError()
		}
	}
}

// Sends heartbeat messages periodically
func (pub *HlsWebSocketPublisher) sendHeartbeatMessages() {
	heartbeatMessage := WebsocketProtocolMessage{
		MessageType: "H",
	}

	for {
		select {
		case <-time.After(time.Duration(heartbeat_msg_period_seconds) * time.Second):
			pub.sendMessage(&heartbeatMessage)
		case <-pub.heartbeatInterruptChannel:
			return
		}
	}
}

// Send message
func (pub *HlsWebSocketPublisher) sendMessage(msg *WebsocketProtocolMessage) {
	pub.mu.Lock()
	defer pub.mu.Unlock()

	if pub.closed || pub.socket == nil {
		return
	}

	pub.socket.WriteMessage(websocket.TextMessage, []byte(msg.Serialize()))
}

// Internal function to send the fragment message
func (pub *HlsWebSocketPublisher) sendFragmentInternal(duration float32, data []byte) {
	msg := WebsocketProtocolMessage{
		MessageType: "F",
		Parameters: map[string]string{
			"duration": fmt.Sprint(duration),
		},
	}

	pub.socket.WriteMessage(websocket.TextMessage, []byte(msg.Serialize()))
	pub.socket.WriteMessage(websocket.BinaryMessage, data)
}

// Called when the connection is opened
func (pub *HlsWebSocketPublisher) onConnected(socket *websocket.Conn) {
	pub.mu.Lock()
	defer pub.mu.Unlock()

	if pub.closed {
		return
	}

	pub.socket = socket
}

// Call on ready
func (pub *HlsWebSocketPublisher) onReady() {
	pub.mu.Lock()
	defer pub.mu.Unlock()

	if pub.closed {
		return
	}

	pub.ready = true

	for _, f := range pub.pendingQueue {
		pub.sendFragmentInternal(f.duration, f.data)
	}

	pub.pendingQueue = make([]cdnPublisherPendingFragment, 0)

	if pub.Config.OnReady != nil {
		go pub.Config.OnReady()
	}
}

// Call when disconnected from the server
func (pub *HlsWebSocketPublisher) onDisconnected() {
	pub.mu.Lock()
	defer pub.mu.Unlock()

	if pub.closed {
		return
	}

	pub.ready = false
	pub.socket = nil
}

func (pub *HlsWebSocketPublisher) SendFragment(duration float32, data []byte) {
	if len(data) == 0 {
		return
	}

	pub.mu.Lock()
	defer pub.mu.Unlock()

	if pub.closed {
		return
	}

	if pub.ready {
		// Ready, just send the fragment
		pub.sendFragmentInternal(duration, data)
	} else {
		// Not ready, append to the queue
		queueMaxLength := pub.Config.QueueMaxLength

		if queueMaxLength == 0 {
			queueMaxLength = 10
		}

		pendingFragment := cdnPublisherPendingFragment{
			duration: duration,
			data:     data,
		}

		if len(pub.pendingQueue) >= queueMaxLength && len(pub.pendingQueue) > 0 {
			pub.pendingQueue = append(pub.pendingQueue[1:], pendingFragment)
		} else {
			pub.pendingQueue = append(pub.pendingQueue, pendingFragment)
		}
	}

}

// Finish the publisher
// This sends the CLOSE message and terminates the connection
func (pub *HlsWebSocketPublisher) Close() {
	pub.mu.Lock()
	defer pub.mu.Unlock()

	if pub.closed {
		return
	}

	if pub.socket != nil {
		// Send close message
		closeMessage := WebsocketProtocolMessage{
			MessageType: "CLOSE",
		}

		pub.socket.WriteMessage(websocket.TextMessage, []byte(closeMessage.Serialize()))

		// Close connection
		pub.socket.Close()
		pub.socket = nil
	}

	pub.pendingQueue = make([]cdnPublisherPendingFragment, 0)
	pub.closed = true
	pub.ready = false

	// Interrupt heartbeat
	pub.heartbeatInterruptChannel <- true
}
