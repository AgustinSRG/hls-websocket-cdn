// Connection handler

package main

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/AgustinSRG/glog"
	"github.com/gorilla/websocket"
)

// Period to send HEARTBEAT messages to the client
const HEARTBEAT_MSG_PERIOD_SECONDS = 30

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

	// Logger
	logger *glog.Logger

	// Mutex for the struct
	mu *sync.Mutex

	// Channel to interrupt the heartbeat thread
	heartbeatInterruptChannel chan bool

	// True if closed
	closed bool

	// Internal flag to indicate if the next message is expected to be binary
	expectedBinary bool

	// Connection current mode
	mode int

	// Stream ID
	streamId string

	// HLS source to push to
	sourceToPush *HlsSource

	// Current fragment to push
	currentFragmentToPush *HlsFragment

	// Channel to interrupt the pulling process
	pullingInterruptChannel chan bool
}

// Creates connection handler
func CreateConnectionHandler(conn *websocket.Conn, server *HttpServer) *ConnectionHandler {
	return &ConnectionHandler{
		id:                        0,
		connection:                conn,
		server:                    server,
		logger:                    server.logger,
		mu:                        &sync.Mutex{},
		heartbeatInterruptChannel: make(chan bool, 1),
		closed:                    false,
		expectedBinary:            false,
		mode:                      0,
		streamId:                  "",
		sourceToPush:              nil,
		currentFragmentToPush:     nil,
		pullingInterruptChannel:   nil,
	}
}

// Called after the connection is closed
func (ch *ConnectionHandler) onClose() {
	ch.mu.Lock()

	ch.closed = true

	ch.mu.Unlock()

	// Clear

	if ch.mode == CONNECTION_MODE_PUSH {
		if ch.sourceToPush != nil {
			ch.sourceToPush.Close()
			ch.server.sourceController.RemoveSource(ch.streamId, ch.sourceToPush)
			ch.sourceToPush = nil
		}

		ch.logger.Info("Source closed due to connection closed.")
	} else if ch.mode == CONNECTION_MODE_PULL {
		if ch.pullingInterruptChannel != nil {
			ch.pullingInterruptChannel <- true
		}
	}

	// Interrupt heartbeat
	ch.heartbeatInterruptChannel <- true
}

// Runs connection handler
func (ch *ConnectionHandler) Run() {
	defer func() {
		// Ensure connection is closed
		ch.connection.Close()
		// Release resources
		ch.onClose()
		// Log
		ch.logger.Info("Connection closed.")
	}()

	// Get a connection ID
	ch.id = ch.server.GetConnectionId()

	// Update logger
	ch.logger = ch.server.logger.CreateChildLogger("[#" + fmt.Sprint(ch.id) + "] ")

	ch.logger.Info("Connection established.")

	go ch.sendHeartbeatMessages() // Start heartbeat sending

	for {
		var deadline time.Time

		if ch.mode == 0 {
			deadline = time.Now().Add(HEARTBEAT_MSG_PERIOD_SECONDS * time.Second)
		} else {
			deadline = time.Now().Add(HEARTBEAT_MSG_PERIOD_SECONDS * 2 * time.Second)
		}

		err := ch.connection.SetReadDeadline(deadline)

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

	if ch.logger.Config.TraceEnabled {
		ch.logger.Trace("<<< " + string(message))
	}

	parsedMessage := ParseWebsocketProtocolMessage(string(message))

	switch parsedMessage.MessageType {
	case "E":
		ch.logger.Debugf("Error from client. Code: %v Message: %v", parsedMessage.GetParameter("code"), parsedMessage.GetParameter("message"))
		return false
	case "PULL":
		return ch.HandlePull(parsedMessage)
	case "PUSH":
		return ch.HandlePush(parsedMessage)
	case "F":
		return ch.HandleFragmentMetadata(parsedMessage)
	case "CLOSE":
		return ch.HandleClose()
	}

	if ch.mode == 0 {
		ch.SendErrorMessage("PROTOCOL_ERROR", "Expected action message (PUSH, PULL) as the first message")
		return false
	}

	return true
}

// Reads binary message and handles it
func (ch *ConnectionHandler) ReadBinaryMessage() bool {
	if ch.currentFragmentToPush == nil || ch.sourceToPush == nil {
		ch.SendErrorMessage("PROTOCOL_ERROR", "Unexpected binary message")
		return false
	}

	ch.connection.SetReadLimit(ch.server.config.MaxBinaryMessageSize)

	mt, message, err := ch.connection.ReadMessage()

	if err != nil {
		ch.logger.Errorf("Error reading binary message: %v", err)
		return false
	}

	if mt != websocket.BinaryMessage {
		ch.SendErrorMessage("PROTOCOL_ERROR", "Expected binary message, but received a text one")
		return false
	}

	if len(message) == 0 {
		ch.SendErrorMessage("PROTOCOL_ERROR", "Unexpected empty binary message")
		return false
	}

	ch.currentFragmentToPush.Data = message

	ch.sourceToPush.AddFragment(ch.currentFragmentToPush)

	ch.expectedBinary = false
	ch.currentFragmentToPush = nil

	return true
}

// Task to send HEARTBEAT periodically
func (ch *ConnectionHandler) sendHeartbeatMessages() {
	heartbeatInterval := HEARTBEAT_MSG_PERIOD_SECONDS * time.Second

	for {
		select {
		case <-ch.heartbeatInterruptChannel:
			return
		case <-time.After(heartbeatInterval):
			// Send heartbeat message
			msg := WebsocketProtocolMessage{
				MessageType: "H",
			}

			ch.Send(&msg)
		}
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

	if ch.logger.Config.TraceEnabled {
		ch.logger.Trace(">>> " + msg.Serialize())
	}

	_ = ch.connection.WriteMessage(websocket.TextMessage, []byte(msg.Serialize()))
}

// Sends a message to the websocket client with attached binary data
func (ch *ConnectionHandler) SendWithBinary(msg *WebsocketProtocolMessage, binaryData []byte) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	if ch.closed {
		return
	}

	if ch.logger.Config.TraceEnabled {
		ch.logger.Trace(">>> " + msg.Serialize())
		ch.logger.Trace(">>> [BINARY] " + fmt.Sprint(len(binaryData)) + " bytes")
	}

	_ = ch.connection.WriteMessage(websocket.TextMessage, []byte(msg.Serialize()))
	_ = ch.connection.WriteMessage(websocket.BinaryMessage, []byte(binaryData))
}

// Sends a close message and closes the connection
func (ch *ConnectionHandler) SendClose() {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	if ch.closed {
		return
	}

	msg := WebsocketProtocolMessage{
		MessageType: "CLOSE",
	}

	if ch.logger.Config.TraceEnabled {
		ch.logger.Trace(">>> " + msg.Serialize())
	}

	_ = ch.connection.WriteMessage(websocket.TextMessage, []byte(msg.Serialize()))
	_ = ch.connection.Close()
}

// Sends a fragment
func (ch *ConnectionHandler) SendFragment(frag *HlsFragment) {
	ch.SendWithBinary(&WebsocketProtocolMessage{
		MessageType: "F",
		Parameters: map[string]string{
			"duration": fmt.Sprint(frag.Duration),
		},
	}, frag.Data)
}

// Handles the PULL message
func (ch *ConnectionHandler) HandlePull(msg *WebsocketProtocolMessage) bool {
	if ch.mode != 0 {
		ch.SendErrorMessage("PROTOCOL_ERROR", "A PUSH message may only be sent as the first message")
		return false
	}

	streamId := msg.GetParameter("stream")

	if streamId == "" {
		ch.SendErrorMessage("PROTOCOL_ERROR", "Stream ID cannot be empty")
		return false
	}

	if len(streamId) > 255 {
		ch.SendErrorMessage("PROTOCOL_ERROR", "Stream ID cannot be larger than 255 characters")
		return false
	}

	authToken := msg.GetParameter("auth")

	if !ch.server.authController.ValidatePullToken(authToken, streamId) {
		ch.SendErrorMessage("AUTH_ERROR", "Invalid auth token")
		return false
	}

	onlySource := msg.GetParameter("only_source") == "true"
	maxInitialFragments := -1

	maxInitialFragmentsStr := msg.GetParameter("max_initial_fragments")
	if maxInitialFragmentsStr != "" {
		n, err := strconv.Atoi(maxInitialFragmentsStr)

		if err != nil {
			ch.SendErrorMessage("PROTOCOL_ERROR", "max_initial_fragments must be a valid integer number")
			return false
		}

		maxInitialFragments = n
	}

	// Create interrupt channel
	ch.pullingInterruptChannel = make(chan bool, 1)

	// PULL the stream

	if ch.server.authController.IsPushAllowed() {
		source := ch.server.sourceController.GetSource(streamId)

		if source != nil {
			// Send OK
			ch.Send(&WebsocketProtocolMessage{
				MessageType: "OK",
			})

			// Pull
			go ch.PullFromHlsSource(source, ch.pullingInterruptChannel, maxInitialFragments)

			// Switch mode
			ch.streamId = streamId
			ch.mode = CONNECTION_MODE_PULL

			return true
		}
	}

	if !onlySource {
		relay := ch.server.relayController.RelayStream(streamId)

		if relay != nil {
			// Send OK
			ch.Send(&WebsocketProtocolMessage{
				MessageType: "OK",
			})

			// Pull
			go ch.PullFromHlsRelay(relay, ch.pullingInterruptChannel, maxInitialFragments)

			// Switch mode
			ch.streamId = streamId
			ch.mode = CONNECTION_MODE_PULL

			return true
		}
	}

	// If not found in any place, send OK and CLOSE (Empty stream)

	ch.Send(&WebsocketProtocolMessage{
		MessageType: "OK",
	})

	ch.Send(&WebsocketProtocolMessage{
		MessageType: "CLOSE",
	})

	return false
}

// Handles the PUSH message
func (ch *ConnectionHandler) HandlePush(msg *WebsocketProtocolMessage) bool {
	if ch.mode != 0 {
		ch.SendErrorMessage("PROTOCOL_ERROR", "A PUSH message may only be sent as the first message")
		return false
	}

	streamId := msg.GetParameter("stream")

	if streamId == "" {
		ch.SendErrorMessage("PROTOCOL_ERROR", "Stream ID cannot be empty")
		return false
	}

	if len(streamId) > 255 {
		ch.SendErrorMessage("PROTOCOL_ERROR", "Stream ID cannot be larger than 255 characters")
		return false
	}

	// Check auth

	authToken := msg.GetParameter("auth")

	if !ch.server.authController.ValidatePushToken(authToken, streamId) {
		ch.SendErrorMessage("AUTH_ERROR", "Invalid auth token")
		return false
	}

	// Create source

	hlsSource := ch.server.sourceController.CreateSource(streamId)

	if hlsSource == nil {
		ch.SendErrorMessage("PUSH_ERROR", "There is already another connection pushing an stream with the same identifier. Please, choose another one.")
		return false
	}

	ch.sourceToPush = hlsSource

	go hlsSource.PeriodicallyAnnounce()

	// Switch mode
	ch.streamId = streamId
	ch.mode = CONNECTION_MODE_PUSH

	// Send OK
	ch.Send(&WebsocketProtocolMessage{
		MessageType: "OK",
	})

	return true
}

func (ch *ConnectionHandler) HandleFragmentMetadata(msg *WebsocketProtocolMessage) bool {
	if ch.mode != CONNECTION_MODE_PUSH {
		ch.SendErrorMessage("PROTOCOL_ERROR", "A fragment message can only be sent in PUSH mode")
		return false
	}

	durationStr := msg.GetParameter("duration")

	if durationStr == "" {
		ch.SendErrorMessage("FRAGMENT_METADATA_ERROR", "The fragment duration must be provided")
		return false
	}

	duration, err := strconv.ParseFloat(durationStr, 32)

	if err != nil {
		ch.SendErrorMessage("FRAGMENT_METADATA_ERROR", "The fragment duration is not a valid floating point number")
		return false
	}

	if duration <= 0 {
		ch.SendErrorMessage("FRAGMENT_METADATA_ERROR", "The fragment duration must be positive")
		return false
	}

	ch.currentFragmentToPush = &HlsFragment{
		Duration: float32(duration),
	}

	ch.expectedBinary = true

	return true
}

func (ch *ConnectionHandler) HandleClose() bool {
	if ch.mode != CONNECTION_MODE_PUSH {
		ch.SendErrorMessage("PROTOCOL_ERROR", "A close message can only be sent in PUSH mode")
		return false
	}

	if ch.sourceToPush == nil {
		ch.SendErrorMessage("PROTOCOL_ERROR", "Unexpected close message")
		return false
	}

	ch.sourceToPush.Close()
	ch.server.sourceController.RemoveSource(ch.streamId, ch.sourceToPush)
	ch.sourceToPush = nil

	ch.streamId = ""

	ch.mode = 0

	return false // After this message, the connection will be closed
}
