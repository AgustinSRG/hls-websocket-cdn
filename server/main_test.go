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

type PublisherSpectatorsSync struct {
	wgPublisher  *sync.WaitGroup
	wgSpectators *sync.WaitGroup
}

func MakePublisherSpectatorsSync(spectatorsCount int) *PublisherSpectatorsSync {
	wgPublisher := &sync.WaitGroup{}
	wgPublisher.Add(1)

	wgSpectators := &sync.WaitGroup{}
	wgSpectators.Add(spectatorsCount)

	return &PublisherSpectatorsSync{
		wgPublisher:  wgPublisher,
		wgSpectators: wgSpectators,
	}
}

func runTestPublisher(name string, serverUrl string, streamId string, dataToPublish []HlsFragment, wg *sync.WaitGroup, groupSync *PublisherSpectatorsSync, t *testing.T) {
	defer wg.Done()

	// Connect

	socket, _, err := websocket.DefaultDialer.Dial(serverUrl, nil)

	if err != nil {
		t.Error(err)
		groupSync.wgPublisher.Done()
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
			groupSync.wgPublisher.Done()
			return
		}

		socket.SetReadLimit(TEXT_MSG_READ_LIMIT)

		mt, message, err := socket.ReadMessage()

		if err != nil {
			t.Error(err)
			groupSync.wgPublisher.Done()
			return
		}

		if mt != websocket.TextMessage {
			t.Errorf("[Publisher: %v] Unexpected non-text message", name)
			groupSync.wgPublisher.Done()
			return
		}

		parsedMessage := ParseWebsocketProtocolMessage(string(message))

		switch parsedMessage.MessageType {
		case "OK":
			done = true
		case "E":
			t.Errorf("[Publisher: %v] Received error message from server: %v", name, parsedMessage.Serialize())
			groupSync.wgPublisher.Done()
			return
		}
	}

	groupSync.wgPublisher.Done() // Publisher is ready to go

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

	// Wait for the spectators before closing

	groupSync.wgSpectators.Wait()

	// Send the close

	closeMessage := WebsocketProtocolMessage{
		MessageType: "CLOSE",
	}

	socket.WriteMessage(websocket.TextMessage, []byte(closeMessage.Serialize()))
}

func runTestSpectator(name string, serverUrl string, streamId string, dataToExpect []HlsFragment, wg *sync.WaitGroup, groupSync *PublisherSpectatorsSync, t *testing.T) {
	defer wg.Done()

	// Wait for the publisher before connecting

	groupSync.wgPublisher.Wait()

	// Connect

	socket, _, err := websocket.DefaultDialer.Dial(serverUrl, nil)

	if err != nil {
		groupSync.wgSpectators.Done()
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
			groupSync.wgSpectators.Done()
			return
		}

		socket.SetReadLimit(TEXT_MSG_READ_LIMIT)

		mt, message, err := socket.ReadMessage()

		if err != nil {
			t.Error(err)
			groupSync.wgSpectators.Done()
			return
		}

		if mt != websocket.TextMessage {
			t.Errorf("[Spectator: %v] Unexpected non-text message", name)
			groupSync.wgSpectators.Done()
			return
		}

		parsedMessage := ParseWebsocketProtocolMessage(string(message))

		switch parsedMessage.MessageType {
		case "OK":
			done = true
		case "E":
			t.Errorf("[Spectator: %v] Received error message from server: %v", name, parsedMessage.Serialize())
			groupSync.wgSpectators.Done()
			return
		}

		// Send Heartbeat
		heartbeatMessage := WebsocketProtocolMessage{
			MessageType: "H",
		}

		socket.WriteMessage(websocket.TextMessage, []byte(heartbeatMessage.Serialize()))
	}

	groupSync.wgSpectators.Done() // Spectator is ready to go

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
				t.Errorf("[Spectator: %v] Unexpected non-binary message", name)
				return
			}

			fragment.Data = message

			// Check fragment

			if fragmentIndex > len(dataToExpect) {
				t.Errorf("[Spectator: %v] Unexpected extra fragment. Index=%v / Expected fragment count:%v ", name, fragmentIndex, dataToExpect)
				return
			}

			expectedFragment := dataToExpect[fragmentIndex]

			if expectedFragment.Duration != fragment.Duration {
				t.Errorf("[Spectator: %v] [F: %v] Duration does not match. Expected %v, Actual: %v", name, fragmentIndex, expectedFragment.Duration, fragment.Duration)
				return
			}

			if !bytes.Equal(expectedFragment.Data, fragment.Data) {
				t.Errorf("[Spectator: %v] [F: %v] Data does not match. Expected %v, Actual: %v", name, fragmentIndex, expectedFragment.Data, fragment.Data)
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
				t.Errorf("[Spectator: %v] Unexpected non-text message", name)
				return
			}

			parsedMessage := ParseWebsocketProtocolMessage(string(message))

			switch parsedMessage.MessageType {
			case "F":
				parsedDuration, err := strconv.ParseFloat(parsedMessage.GetParameter("duration"), 32)

				if err != nil {
					t.Errorf("[Spectator: %v] Invalid duration of fragment: %v", name, parsedMessage.Serialize())
					return
				}

				fragment.Duration = float32(parsedDuration)
				expectingData = true
			case "CLOSE":
				closed = true
			case "E":
				t.Errorf("[Spectator: %v] Received error message from server: %v", name, parsedMessage.Serialize())
				return
			}
		}

		// Send Heartbeat
		heartbeatMessage := WebsocketProtocolMessage{
			MessageType: "H",
		}

		socket.WriteMessage(websocket.TextMessage, []byte(heartbeatMessage.Serialize()))
	}

	if fragmentIndex < len(dataToExpect) {
		t.Errorf("[Spectator: %v] Received less fragments than expected. Expected: %v, Received: %v", name, len(dataToExpect), fragmentIndex)
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

	group1 := MakePublisherSpectatorsSync(1)

	wg.Add(1)
	go runTestPublisher("P1", singleServer.url, TEST_STREAM_ID_1, TEST_STREAM_DATA_1, wg, group1, t)

	wg.Add(1)
	go runTestSpectator("S1", singleServer.url, TEST_STREAM_ID_1, TEST_STREAM_DATA_1, wg, group1, t)

	group2 := MakePublisherSpectatorsSync(2)

	wg.Add(1)
	go runTestPublisher("P2", singleServer.url, TEST_STREAM_ID_2, TEST_STREAM_DATA_2, wg, group2, t)

	wg.Add(1)
	go runTestSpectator("S2", singleServer.url, TEST_STREAM_ID_2, TEST_STREAM_DATA_2, wg, group2, t)

	wg.Add(1)
	go runTestSpectator("S3", singleServer.url, TEST_STREAM_ID_2, TEST_STREAM_DATA_2, wg, group2, t)

	// Wait for clients

	wg.Wait()
}

// Test a scenario with 2 servers and a publish registry
// Publisher -> Server1 -> Server2 -> Spectator
func TestPublishRegistryScenario(t *testing.T) {
	testMain()

	// Prepare mocks

	mockPublishRegistry := NewMockPublishRegistry()

	// Prepare servers

	server1 := makeTestServer(mockPublishRegistry, true, "")
	defer server1.Close()

	server2 := makeTestServer(mockPublishRegistry, true, "")
	defer server2.Close()

	// Run clients

	// Stream 1

	wg := &sync.WaitGroup{}

	group1 := MakePublisherSpectatorsSync(1)

	wg.Add(1)
	go runTestPublisher("P1", server1.url, TEST_STREAM_ID_1, TEST_STREAM_DATA_1, wg, group1, t)

	wg.Add(1)
	go runTestSpectator("S1", server2.url, TEST_STREAM_ID_1, TEST_STREAM_DATA_1, wg, group1, t)

	group2 := MakePublisherSpectatorsSync(2)

	wg.Add(1)
	go runTestPublisher("P2", server2.url, TEST_STREAM_ID_2, TEST_STREAM_DATA_2, wg, group2, t)

	wg.Add(1)
	go runTestSpectator("S2", server1.url, TEST_STREAM_ID_2, TEST_STREAM_DATA_2, wg, group2, t)

	wg.Add(1)
	go runTestSpectator("S3", server2.url, TEST_STREAM_ID_2, TEST_STREAM_DATA_2, wg, group2, t)

	// Wait for clients

	wg.Wait()
}
