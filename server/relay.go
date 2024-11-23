// HLS source relay

package main

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// HLS source relay
type HlsRelay struct {
	// Relay ID
	id uint64

	// Mutex
	mu *sync.Mutex

	// Controller reference
	controller *RelayController

	// URL
	url string

	// Stream ID
	streamId string

	// True to enable the only_source pull option
	onlySource bool

	// Map of listeners
	listeners map[uint64]*HlsSourceListener

	// Buffer of fragments
	fragmentBuffer []*HlsFragment

	// Max length of the fragment buffer
	fragmentBufferMaxLength int

	// True if closed
	closed bool

	// True if the client is connected
	connected bool

	// Socket
	socket *websocket.Conn

	// Current fragment being received
	currentFragment *HlsFragment

	// True if expected binary message
	expectedBinary bool

	// Inactivity warning
	inactivityWarning bool

	// Channel to interrupt the heartbeat thread
	heartbeatInterruptChannel chan bool
}

// Creates new instance of HlsRelay
func NewHlsRelay(controller *RelayController, id uint64, url string, streamId string, fragmentBufferMaxLength int, onlySource bool) *HlsRelay {
	return &HlsRelay{
		id:                        id,
		mu:                        &sync.Mutex{},
		controller:                controller,
		url:                       url,
		streamId:                  streamId,
		onlySource:                onlySource,
		listeners:                 make(map[uint64]*HlsSourceListener),
		fragmentBuffer:            make([]*HlsFragment, 0),
		fragmentBufferMaxLength:   fragmentBufferMaxLength,
		closed:                    false,
		connected:                 false,
		socket:                    nil,
		currentFragment:           nil,
		expectedBinary:            false,
		inactivityWarning:         false,
		heartbeatInterruptChannel: make(chan bool, 1),
	}
}

// Adds a listener
// id - Connection ID
// Returns
// - success: True if the listener was added. If the source is closed, it will be false
// - channel: The channel to receive the events
// - initialFragments: List of fragments to be sent as initial (they were in the buffer)
func (relay *HlsRelay) AddListener(id uint64) (success bool, channel chan HlsEvent, initialFragments []*HlsFragment) {
	lis := &HlsSourceListener{
		Channel: make(chan HlsEvent, relay.fragmentBufferMaxLength),
	}

	relay.mu.Lock()
	defer relay.mu.Unlock()

	if relay.closed {
		return false, nil, nil
	}

	relay.listeners[id] = lis

	initialFragmentsBuffer := make([]*HlsFragment, len(relay.fragmentBuffer))
	copy(initialFragmentsBuffer, relay.fragmentBuffer)

	return true, lis.Channel, initialFragmentsBuffer
}

// Removes a listener
// id - Connection ID
func (relay *HlsRelay) RemoveListener(id uint64) {
	relay.mu.Lock()
	defer relay.mu.Unlock()

	delete(relay.listeners, id)
}

// Closes the relay
func (relay *HlsRelay) Close() {
	relay.mu.Lock()
	defer relay.mu.Unlock()

	if relay.closed {
		return
	}

	closeEvent := HlsEvent{
		EventType: HLS_EVENT_TYPE_CLOSE,
	}

	for _, lis := range relay.listeners {
		lis.Channel <- closeEvent
	}

	if relay.socket != nil {
		relay.socket.Close()
		relay.socket = nil
	}

	relay.listeners = nil
	relay.closed = true
	relay.connected = false

	relay.heartbeatInterruptChannel <- true
}

// Adds fragment
func (relay *HlsRelay) AddFragment(frag *HlsFragment) {
	relay.mu.Lock()
	defer relay.mu.Unlock()

	if relay.closed {
		return
	}

	// Append the fragment to the buffer

	if len(relay.fragmentBuffer) >= relay.fragmentBufferMaxLength && len(relay.fragmentBuffer) > 0 {
		relay.fragmentBuffer = append(relay.fragmentBuffer[1:], frag)
	} else {
		relay.fragmentBuffer = append(relay.fragmentBuffer, frag)
	}

	// Send fragment to the listeners

	fragmentEvent := HlsEvent{
		EventType: HLS_EVENT_TYPE_FRAGMENT,
		Fragment:  frag,
	}

	for _, lis := range relay.listeners {
		lis.Channel <- fragmentEvent
	}
}

// Called after closed
func (relay *HlsRelay) onClose() {
	// Send close
	relay.Close()

	// Set connection status
	relay.mu.Lock()

	relay.socket = nil
	relay.connected = false

	relay.mu.Unlock()

	// Unregister
	relay.controller.OnRelayClosed(relay)
}

// Logs error message for the relay
func (relay *HlsRelay) LogError(err error, msg string) {
	LogError(err, "[Relay: "+fmt.Sprint(relay.id)+"] "+msg)
}

// Logs info message for the relay
func (relay *HlsRelay) LogInfo(msg string) {
	LogInfo("[Relay: " + fmt.Sprint(relay.id) + "]" + msg)
}

// Logs debug message for the relay
func (relay *HlsRelay) LogDebug(msg string) {
	LogDebug("[Relay: " + fmt.Sprint(relay.id) + "] " + msg)
}

// Gets true of the relay is closed
func (relay *HlsRelay) IsClosed() bool {
	relay.mu.Lock()
	defer relay.mu.Unlock()

	return relay.closed
}

// Runs relay
// (run in a sub-routine)
func (relay *HlsRelay) Run() {
	defer func() {
		if err := recover(); err != nil {
			switch x := err.(type) {
			case string:
				relay.LogError(nil, "Error: "+x)
			case error:
				relay.LogError(x, "Relay connection closed with error")
			default:
				LogError(nil, "Relay connection Crashed!")
			}
		}
		// Ensure connection is closed
		relay.socket.Close()
		// Release resources
		relay.onClose()
		// Log
		relay.LogInfo("Relay connection closed")
	}()

	relay.LogInfo("Relay created. Url: " + relay.url + " | Stream: " + relay.streamId)

	socket, _, err := websocket.DefaultDialer.Dial(relay.url, nil)

	if err != nil {
		relay.LogError(err, "Could not connect to the server")
		return
	}

	if relay.IsClosed() {
		return
	}

	relay.LogInfo("Connected to the server")

	relay.mu.Lock()

	relay.socket = socket
	relay.connected = true

	relay.mu.Unlock()

	// Authenticate
	err = relay.SendPullMessage(socket)
	if err != nil {
		relay.LogError(err, "Could not authenticate")
		return
	}

	// Send heartbeat messages periodically
	go relay.sendHeartbeatMessages(socket)

	// Read incoming messages

	for {
		err := socket.SetReadDeadline(time.Now().Add(HEARTBEAT_MSG_PERIOD_SECONDS * 2 * time.Second))

		if err != nil {
			if !relay.IsClosed() {
				relay.LogError(err, "Could not set socket deadline")
			}
			break // Closed
		}

		if !relay.expectedBinary {
			if !relay.ReadTextMessage(socket) {
				break // Closed
			}
		} else {
			if !relay.ReadBinaryMessage(socket) {
				break // Closed
			}
		}

		if relay.IsClosed() {
			break
		}
	}
}

// Sends error message
func (relay *HlsRelay) SendErrorMessage(socket *websocket.Conn, errorCode string, errorMessage string) {
	msg := WebsocketProtocolMessage{
		MessageType: "E",
		Parameters: map[string]string{
			"code":    errorCode,
			"message": errorMessage,
		},
	}

	if log_debug_enabled {
		relay.LogDebug(">>> " + msg.Serialize())
	}

	socket.WriteMessage(websocket.TextMessage, []byte(msg.Serialize()))
}

// Reads text message
func (relay *HlsRelay) ReadTextMessage(socket *websocket.Conn) bool {
	socket.SetReadLimit(TEXT_MSG_READ_LIMIT)

	mt, message, err := socket.ReadMessage()

	if err != nil {
		if !relay.IsClosed() {
			relay.LogError(err, "Could not read text message")
		}
		return false
	}

	if mt != websocket.TextMessage {
		relay.SendErrorMessage(socket, "PROTOCOL_ERROR", "Expected text message, but received a binary one")
		return false
	}

	if log_debug_enabled {
		relay.LogDebug("<<< \n" + string(message))
	}

	parsedMessage := ParseWebsocketProtocolMessage(string(message))

	switch parsedMessage.MessageType {
	case "E":
		relay.LogDebug("Error from server. Code: " + parsedMessage.GetParameter("code") + ", Message: " + parsedMessage.GetParameter("message"))
		return false
	case "OK":
		relay.LogDebug("OK received. Waiting for fragments...")
	case "F":
		return relay.HandleFragmentMetadata(socket, parsedMessage)
	case "CLOSE":
		return relay.HandleClose()
	}

	return true
}

// Handles fragment metadata message
func (relay *HlsRelay) HandleFragmentMetadata(socket *websocket.Conn, msg *WebsocketProtocolMessage) bool {
	durationStr := msg.GetParameter("duration")

	if durationStr == "" {
		relay.SendErrorMessage(socket, "FRAGMENT_METADATA_ERROR", "The fragment duration must be provided")
		return false
	}

	duration, err := strconv.ParseFloat(durationStr, 32)

	if err != nil {
		relay.SendErrorMessage(socket, "FRAGMENT_METADATA_ERROR", "The fragment duration is not a valid floating point number")
		return false
	}

	if duration <= 0 {
		relay.SendErrorMessage(socket, "FRAGMENT_METADATA_ERROR", "The fragment duration must be positive")
		return false
	}

	relay.currentFragment = &HlsFragment{
		Duration: float32(duration),
	}

	relay.expectedBinary = true

	return true
}

// Handles close message
func (relay *HlsRelay) HandleClose() bool {
	relay.Close()
	return false
}

// Reads binary message
func (relay *HlsRelay) ReadBinaryMessage(socket *websocket.Conn) bool {
	if relay.currentFragment == nil {
		relay.SendErrorMessage(socket, "PROTOCOL_ERROR", "Unexpected binary message")
		return false
	}

	socket.SetReadLimit(relay.controller.config.MaxBinaryMessageSize)

	mt, message, err := socket.ReadMessage()

	if err != nil {
		if !relay.IsClosed() {
			relay.LogError(err, "Could not read binary message")
		}
		return false
	}

	if mt != websocket.TextMessage {
		relay.SendErrorMessage(socket, "PROTOCOL_ERROR", "Expected binary message, but received a text one")
		return false
	}

	if len(message) == 0 {
		relay.SendErrorMessage(socket, "PROTOCOL_ERROR", "Unexpected empty binary message")
		return false
	}

	relay.currentFragment.Data = message

	relay.AddFragment(relay.currentFragment)

	relay.expectedBinary = false
	relay.currentFragment = nil

	return true
}

// Sends the PULL message with the corresponding auth token
func (relay *HlsRelay) SendPullMessage(socket *websocket.Conn) error {
	onlySourceStr := "false"

	if relay.onlySource {
		onlySourceStr = "true"
	}

	msg := WebsocketProtocolMessage{
		MessageType: "PULL",
		Parameters: map[string]string{
			"stream":      relay.streamId,
			"auth":        relay.controller.authController.CreatePullToken(relay.streamId),
			"only_source": onlySourceStr,
		},
	}

	if log_debug_enabled {
		relay.LogDebug(">>> " + msg.Serialize())
	}

	return socket.WriteMessage(websocket.TextMessage, []byte(msg.Serialize()))
}

// Sends heartbeat messages until the connection gets closed
func (relay *HlsRelay) sendHeartbeatMessages(socket *websocket.Conn) {
	heartbeatInterval := HEARTBEAT_MSG_PERIOD_SECONDS * time.Second

	for {
		select {
		case <-relay.heartbeatInterruptChannel:
			return
		case <-time.After(heartbeatInterval):
			// Send heartbeat message
			msg := WebsocketProtocolMessage{
				MessageType: "H",
			}

			socket.WriteMessage(websocket.TextMessage, []byte(msg.Serialize()))
		}

		if relay.checkInactivity() {
			return
		}
	}
}

func (relay *HlsRelay) checkInactivity() bool {
	relay.mu.Lock()
	defer relay.mu.Lock()

	if len(relay.listeners) == 0 {
		if relay.inactivityWarning {
			relay.LogInfo("Closing the relay due to inactivity")

			if relay.socket != nil {
				relay.socket.Close()
				relay.socket = nil
			}

			relay.listeners = nil
			relay.closed = true
			relay.connected = false

			return true
		} else {
			relay.LogDebug("Inactivity detected")
			relay.inactivityWarning = true
		}
	}

	return false
}
