// Connection handler

package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Period to send HEARTBEAT messages to the client
const HEARTBEAT_MSG_PERIOD_SECONDS = 30

// Max time with no HEARTBEAT messages to consider the connection dead
const HEARTBEAT_TIMEOUT_MS = 2 * HEARTBEAT_MSG_PERIOD_SECONDS * 1000

// Limit (in bytes) for text messages (to prevent DOS attacks)
const TEXT_MSG_READ_LIMIT = 1600

// Push mode
const CONNECTION_MODE_PUSH = 1

// Pull mode
const CONNECTION_MODE_PULL = 2

// Connection handler
type ConnectionHandler struct {
	// Connection id
	id uint64

	// Connection
	connection *websocket.Conn

	// HTTP server
	server *HttpServer

	// Mutex for the struct
	mu *sync.Mutex

	// Timestamp: Last time a message was sent to the client
	lastSentMessage int64

	// True if closed
	closed bool

	// Internal flag to indicate if the next message is expected to be binary
	expectedBinary bool

	// Connection current mode
	mode int
}

// Creates connection handler
func CreateConnectionHandler(conn *websocket.Conn, server *HttpServer) *ConnectionHandler {
	return &ConnectionHandler{
		id:              0,
		connection:      conn,
		server:          server,
		mu:              &sync.Mutex{},
		lastSentMessage: time.Now().UnixMilli(),
		closed:          false,
		expectedBinary:  false,
		mode:            0,
	}
}

// Logs error message for the connection
func (ch *ConnectionHandler) LogError(err error, msg string) {
	LogError(err, "[Request: "+fmt.Sprint(ch.id)+"] "+msg)
}

// Logs info message for the connection
func (ch *ConnectionHandler) LogInfo(msg string) {
	LogInfo("[Request: " + fmt.Sprint(ch.id) + "] " + msg)
}

// Logs debug message for the connection
func (ch *ConnectionHandler) LogDebug(msg string) {
	LogDebug("[Request: " + fmt.Sprint(ch.id) + "] " + msg)
}

// Called after the connection is closed
func (ch *ConnectionHandler) onClose() {
	ch.mu.Lock()

	ch.closed = true

	ch.mu.Unlock()

	// TODO
}

// Runs connection handler
func (ch *ConnectionHandler) Run() {
	defer func() {
		if err := recover(); err != nil {
			switch x := err.(type) {
			case string:
				ch.LogError(nil, "Error: "+x)
			case error:
				ch.LogError(x, "Connection closed with error")
			default:
				ch.LogError(nil, "Connection Crashed!")
			}
		}
		ch.LogInfo("Connection closed.")
		// Ensure connection is closed
		ch.connection.Close()
		// Release resources
		ch.onClose()
	}()

	// Get a connection ID
	ch.id = ch.server.GetConnectionId()

	ch.LogInfo("Connection established.")

	go ch.sendHeartbeatMessages() // Start heartbeat sending

	for {
		err := ch.connection.SetReadDeadline(time.Now().Add(60 * time.Second))

		if err != nil {
			break
		}

		if !ch.expectedBinary {
			if !ch.ReadTextMessage() {
				break // Closed
			}
		} else {
			if !ch.ReadBinaryMessage() {
				break // Closed
			}
		}
	}
}

// Reads a text message, parses it, and handles it
func (ch *ConnectionHandler) ReadTextMessage() bool {
	ch.connection.SetReadLimit(TEXT_MSG_READ_LIMIT)

	mt, message, err := ch.connection.ReadMessage()

	if err != nil {
		return false
	}

	if mt != websocket.TextMessage {
		ch.SendErrorMessage("PROTOCOL_ERROR", "Expected text message, but received a binary one")
		return false
	}

	if log_debug_enabled {
		ch.LogDebug("<<< \n" + string(message))
	}

	parsedMessage := ParseWebsocketProtocolMessage(string(message))

	switch parsedMessage.MessageType {
	case "E":
		ch.LogDebug("Error from client. Code: " + parsedMessage.GetParameter("code") + ", Message: " + parsedMessage.GetParameter("message"))
	case "PULL":
	case "PUSH":
	case "OK":
	case "F":
	case "CLOSE":
	}

	return true
}

// Reads binary message and handles it
func (ch *ConnectionHandler) ReadBinaryMessage() bool {
	ch.connection.SetReadLimit(ch.server.config.MaxBinaryMessageSize)

	mt, message, err := ch.connection.ReadMessage()

	if err != nil {
		ch.LogError(err, "Error reading binary message")
		return false
	}

	if mt != websocket.TextMessage {
		ch.SendErrorMessage("PROTOCOL_ERROR", "Expected binary message, but received a text one")
		return false
	}

	// TODO

	if len(message) == 0 {
		return false
	}

	return true
}

// Checks if a heartbeat message is needed to keep the connection alive
func (ch *ConnectionHandler) checkHeartbeatNeeded() bool {
	now := time.Now().UnixMilli()

	ch.mu.Lock()
	defer ch.mu.Unlock()

	return now-ch.lastSentMessage > (HEARTBEAT_MSG_PERIOD_SECONDS * time.Second).Milliseconds()
}

// Task to send HEARTBEAT periodically
func (ch *ConnectionHandler) sendHeartbeatMessages() {
	for {
		time.Sleep(HEARTBEAT_MSG_PERIOD_SECONDS * time.Second)

		if ch.closed {
			return // Closed
		}

		if !ch.checkHeartbeatNeeded() {
			continue
		}

		// Send heartbeat message
		msg := WebsocketProtocolMessage{
			MessageType: "H",
		}

		ch.Send(&msg)
	}
}

// Sends error message
func (ch *ConnectionHandler) SendErrorMessage(errorCode string, errorMessage string) {
	msg := WebsocketProtocolMessage{
		MessageType: "E",
		Parameters: map[string]string{
			"code":    errorCode,
			"message": errorMessage,
		},
	}

	ch.Send(&msg)
}

// Sends a message to the websocket client
func (ch *ConnectionHandler) Send(msg *WebsocketProtocolMessage) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	if ch.closed {
		return
	}

	if log_debug_enabled {
		ch.LogDebug(">>> " + msg.Serialize())
	}

	ch.connection.WriteMessage(websocket.TextMessage, []byte(msg.Serialize()))

	ch.lastSentMessage = time.Now().UnixMilli()
}

// Sends a message to the websocket client with attached binary data
func (ch *ConnectionHandler) SendWithBinary(msg *WebsocketProtocolMessage, binaryData []byte) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	if ch.closed {
		return
	}

	if log_debug_enabled {
		ch.LogDebug(">>> " + msg.Serialize())
		ch.LogDebug(">>>[BINARY] " + fmt.Sprint(len(binaryData)) + " bytes")
	}

	ch.connection.WriteMessage(websocket.TextMessage, []byte(msg.Serialize()))
	ch.connection.WriteMessage(websocket.BinaryMessage, []byte(binaryData))

	ch.lastSentMessage = time.Now().UnixMilli()
}
