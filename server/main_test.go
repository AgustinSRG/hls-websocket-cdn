// Main test

package main

import (
	"bytes"
	"fmt"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/AgustinSRG/genv"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
)

const TEST_JWT_SECRET = "test-secret"

const TEST_STREAM_ID_1 = "test1"

var TEST_STREAM_DATA_1 = []HlsFragment{
	{
		Duration: 1,
		Data:     []byte{0xaa, 0xbb, 0xcc, 0x12},
	},
	{
		Duration: 2.5,
		Data:     []byte{0x11},
	},
	{
		Duration: 2,
		Data:     []byte{0xff, 0x00, 0xff, 0xff},
	},
}

const TEST_STREAM_ID_2 = "test2"

var TEST_STREAM_DATA_2 = []HlsFragment{
	{
		Duration: 1,
		Data:     []byte{0x00, 0x02, 0x03, 0x05},
	},
	{
		Duration: 2.5,
		Data:     []byte{0x00, 0x02, 0x03, 0x05, 0x06, 0xff, 0x77, 0x44},
	},
	{
		Duration: 2,
		Data:     []byte{0xff, 0xff, 0xff},
	},
	{
		Duration: 1.5,
		Data:     []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
	},
	{
		Duration: 3,
		Data:     []byte{0x00, 0xff, 0x03, 0x05, 0x00, 0xff, 0x03, 0x05},
	},
}

func testMain() {
	godotenv.Load() // Load env vars

	// Configure logs
	SetDebugLogEnabled(genv.GetEnvBool("LOG_DEBUG", false))
	SetInfoLogEnabled(genv.GetEnvBool("LOG_INFO", true))
}

func runTestPublisher(serverUrl string, streamId string, dataToPublish []HlsFragment, wg *sync.WaitGroup, t *testing.T) {
	defer wg.Done()

	// Connect

	socket, _, err := websocket.DefaultDialer.Dial(serverUrl, nil)

	if err != nil {
		t.Error(err)
		return
	}

	defer socket.Close()

	// Authenticate

	authToken := signAuthToken(TEST_JWT_SECRET, "PUSH", streamId)

	msg := WebsocketProtocolMessage{
		MessageType: "PUSH",
		Parameters: map[string]string{
			"stream": streamId,
			"auth":   authToken,
		},
	}

	socket.WriteMessage(websocket.TextMessage, []byte(msg.Serialize()))

	// Wait for the OK

	done := false

	for !done {
		err := socket.SetReadDeadline(time.Now().Add(HEARTBEAT_MSG_PERIOD_SECONDS * 2 * time.Second))

		if err != nil {
			t.Error(err)
			return
		}

		socket.SetReadLimit(TEXT_MSG_READ_LIMIT)

		mt, message, err := socket.ReadMessage()

		if err != nil {
			t.Error(err)
			return
		}

		if mt != websocket.TextMessage {
			t.Errorf("Unexpected non-text message")
			return
		}

		parsedMessage := ParseWebsocketProtocolMessage(string(message))

		switch parsedMessage.MessageType {
		case "OK":
			done = true
		case "E":
			t.Errorf("Received error message from server: %v", parsedMessage.Serialize())
			return
		}
	}

	// Send the fragments

	for _, f := range dataToPublish {
		metadataMessage := WebsocketProtocolMessage{
			MessageType: "F",
			Parameters: map[string]string{
				"duration": fmt.Sprint(f.Duration),
			},
		}

		socket.WriteMessage(websocket.TextMessage, []byte(metadataMessage.Serialize()))
		socket.WriteMessage(websocket.BinaryMessage, f.Data)
	}

	// Send the close

	closeMessage := WebsocketProtocolMessage{
		MessageType: "CLOSE",
	}

	socket.WriteMessage(websocket.TextMessage, []byte(closeMessage.Serialize()))
}

func runTestSpectator(serverUrl string, streamId string, dataToExpect []HlsFragment, wg *sync.WaitGroup, t *testing.T) {
	defer wg.Done()

	// Connect

	socket, _, err := websocket.DefaultDialer.Dial(serverUrl, nil)

	if err != nil {
		t.Error(err)
		return
	}

	defer socket.Close()

	// Authenticate

	authToken := signAuthToken(TEST_JWT_SECRET, "PULL", streamId)

	msg := WebsocketProtocolMessage{
		MessageType: "PULL",
		Parameters: map[string]string{
			"stream": streamId,
			"auth":   authToken,
		},
	}

	socket.WriteMessage(websocket.TextMessage, []byte(msg.Serialize()))

	// Wait for the OK

	done := false

	for !done {
		err := socket.SetReadDeadline(time.Now().Add(HEARTBEAT_MSG_PERIOD_SECONDS * 2 * time.Second))

		if err != nil {
			t.Error(err)
			return
		}

		socket.SetReadLimit(TEXT_MSG_READ_LIMIT)

		mt, message, err := socket.ReadMessage()

		if err != nil {
			t.Error(err)
			return
		}

		if mt != websocket.TextMessage {
			t.Errorf("Unexpected non-text message")
			return
		}

		parsedMessage := ParseWebsocketProtocolMessage(string(message))

		switch parsedMessage.MessageType {
		case "OK":
			done = true
		case "E":
			t.Errorf("Received error message from server: %v", parsedMessage.Serialize())
			return
		}

		// Send Heartbeat
		heartbeatMessage := WebsocketProtocolMessage{
			MessageType: "H",
		}

		socket.WriteMessage(websocket.TextMessage, []byte(heartbeatMessage.Serialize()))
	}

	// Wait for the messages

	closed := false
	expectingData := false
	fragmentIndex := 0
	fragment := HlsFragment{
		Duration: 0,
		Data:     make([]byte, 0),
	}

	for !closed {
		err := socket.SetReadDeadline(time.Now().Add(HEARTBEAT_MSG_PERIOD_SECONDS * 2 * time.Second))

		if err != nil {
			t.Error(err)
			return
		}

		if expectingData {
			socket.SetReadLimit(DEFAULT_MAX_BINARY_MSG_SIZE)

			mt, message, err := socket.ReadMessage()

			if err != nil {
				t.Error(err)
				return
			}

			if mt != websocket.BinaryMessage {
				t.Errorf("Unexpected non-binary message")
				return
			}

			fragment.Data = message

			// Check fragment

			if fragmentIndex > len(dataToExpect) {
				t.Errorf("Unexpected extra fragment. Index=%v / Expected fragment count:%v ", fragmentIndex, dataToExpect)
				return
			}

			expectedFragment := dataToExpect[fragmentIndex]

			if expectedFragment.Duration != fragment.Duration {
				t.Errorf("[F: %v] Duration does not match. Expected %v, Actual: %v", fragmentIndex, expectedFragment.Duration, fragment.Duration)
				return
			}

			if !bytes.Equal(expectedFragment.Data, fragment.Data) {
				t.Errorf("[F: %v] Data does not match. Expected %v, Actual: %v", fragmentIndex, expectedFragment.Data, fragment.Data)
				return
			}

			fragmentIndex++

			expectingData = false
		} else {
			socket.SetReadLimit(TEXT_MSG_READ_LIMIT)

			mt, message, err := socket.ReadMessage()

			if err != nil {
				t.Error(err)
				return
			}

			if mt != websocket.TextMessage {
				t.Errorf("Unexpected non-text message")
				return
			}

			parsedMessage := ParseWebsocketProtocolMessage(string(message))

			switch parsedMessage.MessageType {
			case "F":
				parsedDuration, err := strconv.ParseFloat(parsedMessage.GetParameter("duration"), 32)

				if err != nil {
					t.Errorf("Invalid duration of fragment: %v", parsedMessage.Serialize())
					return
				}

				fragment.Duration = float32(parsedDuration)
				expectingData = true
			case "CLOSE":
				closed = true
			case "E":
				t.Errorf("Received error message from server: %v", parsedMessage.Serialize())
				return
			}
		}

		// Send Heartbeat
		heartbeatMessage := WebsocketProtocolMessage{
			MessageType: "H",
		}

		socket.WriteMessage(websocket.TextMessage, []byte(heartbeatMessage.Serialize()))
	}
}

type TestServer struct {
	server   *HttpServer
	listener net.Listener
	url      string
}

func makeTestServer(publishRegistry *MockPublishRegistry, allowPush bool, relayFrom string) *TestServer {
	// Auth
	authController := NewAuthController(AuthConfiguration{
		PullSecret: TEST_JWT_SECRET,
		PushSecret: TEST_JWT_SECRET,
		AllowPush:  allowPush,
	})

	// Sources controller
	sourcesController := NewSourcesController(SourcesControllerConfig{
		FragmentBufferMaxLength: DEFAULT_FRAGMENT_BUFFER_MAX_LENGTH,
		ExternalWebsocketUrl:    "",
	}, publishRegistry)

	// Relay controller
	relayController := NewRelayController(RelayControllerConfig{
		RelayFromUrl:            relayFrom,
		RelayFromEnabled:        relayFrom != "",
		FragmentBufferMaxLength: DEFAULT_FRAGMENT_BUFFER_MAX_LENGTH,
		MaxBinaryMessageSize:    DEFAULT_MAX_BINARY_MSG_SIZE,
	}, authController, publishRegistry)

	// Setup server
	server := CreateHttpServer(HttpServerConfig{
		// Other config
		WebsocketPrefix:      "/",
		MaxBinaryMessageSize: DEFAULT_MAX_BINARY_MSG_SIZE,
	}, authController, sourcesController, relayController)

	// Run test server

	url, listener := server.RunTestServer()

	return &TestServer{
		server:   server,
		listener: listener,
		url:      url,
	}
}

func (ts *TestServer) Close() {
	ts.listener.Close()
}

// Test a direct scenario
// Publisher -> Server -> Spectator
func TestDirectScenario(t *testing.T) {
	testMain()

	// Prepare mocks

	mockPublishRegistry := NewMockPublishRegistry()

	// Prepare servers

	singleServer := makeTestServer(mockPublishRegistry, true, "")
	defer singleServer.Close()

	// Run clients

	wg := &sync.WaitGroup{}

	wg.Add(1)
	go runTestPublisher(singleServer.url, TEST_STREAM_ID_1, TEST_STREAM_DATA_1, wg, t)

	wg.Add(1)
	go runTestSpectator(singleServer.url, TEST_STREAM_ID_1, TEST_STREAM_DATA_1, wg, t)

	wg.Add(1)
	go runTestPublisher(singleServer.url, TEST_STREAM_ID_2, TEST_STREAM_DATA_2, wg, t)

	wg.Add(1)
	go runTestSpectator(singleServer.url, TEST_STREAM_ID_2, TEST_STREAM_DATA_2, wg, t)

	// Wait for clients

	wg.Wait()
}
